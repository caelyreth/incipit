package documentview

import (
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// TextInput is the native event boundary used by the document element. Keymap
// dispatch and clipboard remain injected, while text, selection, geometry and
// composition are owned by the controller-backed client.
type TextInput interface {
	Client() ui.TextInputClient
	Select(anchor, head int, at time.Time) error
	Scroll(delta float32)
	Keymap(ui.InputEvent) bool
}

// Element is the reusable custom writing surface. It owns retained paragraph
// layout and contractual caret/selection motion, but not the document.
type Element struct {
	Input       TextInput
	Theme       theme.Theme
	Prose       theme.ProseStyle
	Animate     bool
	Typewriter  bool
	Label       string
	AutoFocus   bool
	Placeholder string
	Layout      *textlayout.Cache
	Motion      *motion.Motion
	TopInset    float32
	BottomInset float32
	// Margin is the room beside the column, where hints that must not
	// read as part of the text show; UIFontSize sizes the keys they name.
	Margin     float32
	UIFontSize float32
	// Pictures reads the pictures the page's figures show.
	Pictures *Pictures
	figures  FigureMotion
	drop     dropLine
	// dropOver is where dragged pictures would go, while they are over the
	// page: the offset of the block they would follow.
	dropOver   int
	dropActive bool
	resizing   *figureResize
	hover      figureHover
	// captions is how open each figure's caption is.
	captions map[model.NodeID]motion.Spring
	// announced is the figure last read out as it became the selection,
	// and menuFigure the one a secondary click opened the menu of.
	announced            model.NodeID
	menuFigure           model.NodeID
	scroll               float32
	overscroll           scrollfx.Elastic
	lastHeight           float32
	manualUntil          time.Time
	pointerAnchor        int
	pointerHead          int
	pointer              pointerPhase
	lastWidth            float32
	guides               Guides
	focusPending         bool
	current              *textlayout.Layout
	placeholderLayout    *ui.TextLayout
	placeholderKey       placeholderKey
	lastCaretY           float32
	lastViewportH        float32
	lastTypewriter       bool
	drawnCaret           ui.Rect
	caretPresentationKey caretPresentationKey
	mounted              bool
}

type caretPresentationKey struct {
	text      string
	selection controller.UTF16Range
	reversed  bool
	width     float32
	height    float32
}

// pointerPhase follows one press. A press keeps the caret on its springs, so a
// click glides from the previous caret and interrupts motion already in
// flight; only a drag that has moved the caret manipulates geometry directly.
type pointerPhase uint8

const (
	pointerIdle pointerPhase = iota
	pointerPressed
	pointerSelecting
)

type placeholderKey struct {
	Text       string
	FontSize   float32
	LineHeight float32
	Width      float32
}

func NewElement(
	input TextInput,
	semantic theme.Theme,
	prose theme.ProseStyle,
	now time.Time,
) *Element {
	return &Element{
		Input:        input,
		Theme:        semantic,
		Prose:        prose,
		Label:        "Document",
		Placeholder:  "Start writing…",
		AutoFocus:    true,
		focusPending: true,
		Layout:       textlayout.NewCache(),
		Motion:       motion.New(now),
	}
}

func (e *Element) View(c *ui.Context) {
	if e == nil || e.Input == nil {
		return
	}
	client := e.Input.Client()
	animate := e.Animate && !c.Preferences().ReduceMotion
	box := ui.Box(c).Fill().FocusRing(false).
		Role(ui.RoleTextField).
		Label(e.label()).
		Value(e.value()).
		Focusable()
	if e.focusPending {
		if e.AutoFocus && !e.mounted {
			box.AutoFocus()
		} else {
			box.Focus()
		}
		e.focusPending = false
	}
	e.mounted = true
	// A picture decoded off the UI thread redraws the page that asked.
	e.Pictures.SetWake(c.Services().Invalidate)
	e.takeDrops(box)
	e.followFigures(c, box)
	box.ContextMenu(e.contextMenu)
	box.HandleInput(e.handleInput).
		TextCaret(e.caret(c)).
		Draw(func(p *ui.Painter, rect ui.Rect) {
			e.paint(p, rect, box.Focused(), animate)
		})
	if client != nil {
		box.HandleTextInput(client)
	}
}

// Paint draws the document into rect from another element's drawing, as a
// page that takes no input: a scripted showcase paints it as focused, so
// its caret, guides and focused decorations show without keyboard focus.
func (e *Element) Paint(p *ui.Painter, rect ui.Rect, focused, animate bool) {
	if e != nil {
		e.paint(p, rect, focused, animate)
	}
}

// ContentSize is the laid-out document's extent as last painted: its
// widest line and its height.
func (e *Element) ContentSize() (float32, float32) {
	if e == nil || e.current == nil {
		return 0, 0
	}
	width := float32(0)
	length := model.StringLenUTF16(e.current.Flow().Text())
	for _, line := range e.current.RangeRects(model.NewUTF16Range(0, model.UTF16Offset(length))) {
		width = max(width, line.X+line.W)
	}
	return width, e.current.Height()
}

// Focus requests keyboard focus on the next frame, as after a page switch.
func (e *Element) Focus() {
	if e != nil {
		e.focusPending = true
	}
}

// FocusPending reports whether a future frame will restore keyboard focus.
func (e *Element) FocusPending() bool {
	return e != nil && e.focusPending
}

func (e *Element) label() string {
	if e == nil || e.Label == "" {
		return "Document"
	}
	return e.Label
}

func (e *Element) value() string {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return ""
	}
	return bridge.Controller.Text()
}

func (e *Element) handleInput(event ui.InputEvent) bool {
	if e == nil || e.Input == nil {
		return false
	}
	switch event.Kind {
	case ui.InputPointerDown:
		if e.current == nil {
			return false
		}
		e.Motion.Activity(event.Time)
		if handled, ok := e.pressFigure(event); handled {
			return ok
		}
		offset := e.hit(event.X, event.Y)
		anchor := offset
		switch {
		case event.Clicks >= 3:
			if paragraph, ok := e.unitAt(offset, editor.SelectionParagraph); ok {
				anchor, offset = paragraph.Start, paragraph.End
			}
		case event.Clicks == 2:
			if word, ok := e.unitAt(offset, editor.SelectionWord); ok {
				anchor, offset = word.Start, word.End
			}
		case event.Mods&ui.Shift != 0:
			anchor = e.selectionAnchor(offset)
		}
		e.pointerAnchor = anchor
		e.pointer = pointerPressed
		e.pointerHead = offset
		return e.Input.Select(anchor, offset, event.Time) == nil
	case ui.InputPointerMove:
		if e.resizing != nil {
			e.dragResize(event.X)
			return true
		}
		if e.pointer == pointerIdle || e.current == nil || event.Button != 0 {
			return false
		}
		head := e.hit(event.X, event.Y)
		if head == e.pointerHead {
			// Jitter inside the pressed caret position is still a click.
			return true
		}
		e.pointer = pointerSelecting
		e.pointerHead = head
		return e.Input.Select(e.pointerAnchor, head, event.Time) == nil
	case ui.InputPointerUp:
		if e.resizing != nil {
			e.finishResize()
			e.Motion.Activity(event.Time)
			return true
		}
		if e.pointer == pointerIdle {
			return false
		}
		e.pointer = pointerIdle
		e.Motion.Activity(event.Time)
		return true
	case ui.InputScroll:
		e.manualScroll(event.DY, event.Time, event.Precise, event.Momentum)
		e.Input.Scroll(event.DY)
		return true
	case ui.InputSelect:
		start, end := event.Start, event.End
		if start > end {
			start, end = end, start
		}
		text := e.value()
		start16 := utf16OffsetForRune(text, start)
		end16 := utf16OffsetForRune(text, end)
		if start == end {
			start16 = snapGraphemeBoundary(text, start16, false)
			end16 = start16
		} else {
			start16 = snapGraphemeBoundary(text, start16, false)
			end16 = snapGraphemeBoundary(text, end16, true)
		}
		e.Motion.Activity(event.Time)
		return e.Input.Select(
			start16,
			end16,
			event.Time,
		) == nil
	case ui.InputText, ui.InputCompose:
		e.Motion.Activity(event.Time)
		return false
	case ui.InputKeyDown, ui.InputCommand:
		e.Motion.Activity(event.Time)
		if event.Kind == ui.InputKeyDown {
			if handled, ok := e.figureKey(event); handled {
				return ok
			}
		}
		return e.Input.Keymap(event)
	default:
		return false
	}
}

// followFigures shows what the pointer can do over a figure, as its
// cursor, and reads out a figure as it becomes the selection.
func (e *Element) followFigures(c *ui.Context, box ui.Element) {
	e.hover = figureHover{}
	if x, y, inside := box.PointerPosition(); inside {
		if figure, side, ok := e.figureAt(x, y); ok {
			e.hover = figureHover{node: figure.Node, side: side, ok: true}
		}
	}
	switch {
	case e.resizing != nil || e.hover.ok && e.hover.side != 0:
		box.Cursor(ui.CursorResizeEW)
	case e.hover.ok:
		box.Cursor(ui.CursorDefault)
	}
	figure, selected := e.selectedFigure()
	switch {
	case !selected:
		e.announced = 0
	case figure.Node != e.announced:
		e.announced = figure.Node
		c.Announce(figureAnnouncement(figure))
	}
}

// takeDrops makes the page take pictures dragged onto it, following the
// pointer with the line that shows where they would go.
func (e *Element) takeDrops(box ui.Element) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Images == nil {
		return
	}
	options := transfer.DropOptions{Formats: pictureFormats}
	_, over := ui.DataDragOver(box, options)
	if over && e.current != nil {
		if x, y, inside := box.PointerPosition(); inside {
			e.dropOver = e.hit(x, y)
		} else if !e.dropActive {
			e.dropOver = bridge.Controller.Selection().Caret()
		}
	}
	e.dropActive = over
	if drop, ok := ui.DropData(box, options); ok {
		e.dropActive = false
		bridge.dropPictures(drop.Data, e.dropOver)
	}
}

func (e *Element) hit(x, y float32) int {
	if e == nil || e.current == nil {
		return 0
	}
	return int(e.current.Hit(ui.Point{
		X: x,
		Y: y + e.visibleScroll() - e.TopInset,
	}))
}

func (e *Element) unitAt(offset int, unit editor.SelectionUnit) (struct{ Start, End int }, bool) {
	var result struct{ Start, End int }
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return result, false
	}
	text := bridge.Controller.Text()
	range16, err := editor.EnclosingUTF16Range(text, model.UTF16Offset(offset), unit)
	if err != nil || range16.Start >= range16.End {
		return result, false
	}
	result.Start, result.End = int(range16.Start), int(range16.End)
	return result, true
}

func (e *Element) selectionAnchor(fallback int) int {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return fallback
	}
	selection := bridge.Controller.Selection()
	if selection.Range.Empty() {
		return selection.Caret()
	}
	if selection.Reversed {
		return selection.Range.End
	}
	return selection.Range.Start
}

func (e *Element) caret(c *ui.Context) ui.Rect {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return ui.Rect{}
	}
	if e.caretPresentationKey == e.currentCaretKey(bridge.Controller) &&
		e.drawnCaret.H > 0 {
		return e.drawnCaret
	}
	selection := bridge.Controller.Selection()
	bounds, _, ok := bridge.Controller.BoundsForRange(controller.UTF16Range{
		Start: selection.Caret(),
		End:   selection.Caret(),
	})
	if !ok {
		return ui.Rect{}
	}
	rect := ui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: bounds.H}
	return rect
}

func (e *Element) currentCaretKey(controllerValue *controller.DocumentController) caretPresentationKey {
	if e == nil || controllerValue == nil {
		return caretPresentationKey{}
	}
	height := float32(0)
	if e.current != nil {
		height = e.current.Height()
	}
	selection := controllerValue.Selection()
	return caretPresentationKey{
		text:      controllerValue.Text(),
		selection: selection.Range,
		reversed:  selection.Reversed,
		width:     e.lastWidth,
		height:    height,
	}
}

func (e *Element) caretPresentation(controllerValue *controller.DocumentController) *CaretPresentation {
	if e == nil || controllerValue == nil || e.current == nil ||
		e.drawnCaret.H <= 0 ||
		e.caretPresentationKey != e.currentCaretKey(controllerValue) {
		return nil
	}
	return &CaretPresentation{
		Text:      controllerValue.Text(),
		Selection: controllerValue.Selection().Range,
		Rect: controller.Rect{
			X: e.drawnCaret.X,
			Y: e.drawnCaret.Y,
			W: e.drawnCaret.W,
			H: e.drawnCaret.H,
		},
	}
}

func (e *Element) paint(p *ui.Painter, rect ui.Rect, focused, animate bool) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return
	}
	if _, moving := e.overscroll.Step(p.Now(), animate); moving {
		p.AnimationFrame()
	}
	frame, err := bridge.Controller.SurfaceFrame(p.Now())
	if err != nil {
		return
	}
	flow, opening := e.captionedFlow(e.resizedFlow(frame.Flow()), frame.Selection(), p.Now(), animate)
	if opening {
		p.AnimationFrame()
	}

	style := textlayout.NewStyle(e.Prose, e.Theme)
	e.lastHeight = rect.H
	if e.lastWidth != rect.W {
		// Rewrapped rows have no previous identity; never morph across them.
		e.Motion.ResetGeometry()
		e.guides.Reset()
		e.lastWidth = rect.W
	}
	e.current = e.Layout.Update(flow, rect.W, style)
	caret := e.current.Caret(frame.Caret())
	caret.Y += e.TopInset
	line, ok := e.current.CaretLine(frame.Caret())
	if !ok {
		line = caret
	}
	line.Y += e.TopInset
	modeChanged := e.lastTypewriter != e.Typewriter
	if e.lastCaretY != caret.Y || e.lastViewportH != rect.H || modeChanged {
		if e.lastCaretY != caret.Y {
			// The caret moved; a page glide alone leaves its breathing be.
			e.Motion.Activity(p.Now())
		}
		if modeChanged && !e.Typewriter {
			// Leaving typewriter mode redirects whatever glide is under way,
			// at its speed, into the plain range, rather than letting it
			// reach a centered offset and then snapping into range.
			low, high := e.scrollRange(e.current, rect.H)
			e.Motion.Settle(e.scroll, low, high, p.Now())
		} else {
			// Entering it glides to the centered line from where the page
			// is and at the speed it is moving.
			e.revealCaret(e.current, line, p.Now())
		}
		e.lastCaretY = caret.Y
		e.lastViewportH = rect.H
		e.lastTypewriter = e.Typewriter
	}
	scrollMin, scrollMax := e.scrollRange(e.current, rect.H)
	if !e.Motion.Scrolling() && (e.scroll < scrollMin || e.scroll > scrollMax) {
		if animate {
			// The range moved out from under the page, as when text is
			// deleted: glide back into it rather than snap.
			e.Motion.Settle(e.scroll, scrollMin, scrollMax, p.Now())
		} else {
			e.scroll = max(scrollMin, min(e.scroll, scrollMax))
		}
	}
	e.advanceScroll(p.Now())
	state := &State{
		Flow:              flow,
		Frame:             frame,
		Layout:            e.current,
		Motion:            e.Motion,
		Theme:             e.Theme,
		Prose:             e.Prose,
		ScrollY:           e.visibleScroll(),
		Focused:           focused,
		Animate:           animate,
		Placeholder:       e.Placeholder,
		PlaceholderLayout: e.placeholder(rect.W),
		PointerActive:     e.pointer != pointerIdle,
		PointerSelecting:  e.pointer == pointerSelecting,
		Composing:         bridge.Controller.IsComposing(),
		TopInset:          e.TopInset,
		Margin:            e.Margin,
		UIFontSize:        e.UIFontSize,
		BottomInset:       e.BottomInset,
		Guides:            &e.guides,
		Pictures:          e.Pictures,
		FigureMotion:      &e.figures,
		Drop:              &e.drop,
		HoverFigure:       e.hover,
		Resizing:          e.resizing,
	}
	if e.drop.advance(e.current.BlockEdge(model.UTF16Offset(e.dropOver)), e.dropActive, p.Now(), animate) {
		p.AnimationFrame()
	}
	Paint(p, rect, state)
	e.drawnCaret = state.DrawnCaret
	e.caretPresentationKey = e.currentCaretKey(bridge.Controller)
	bridge.Controller.SetGeometry(LayoutGeometry{
		Layout: e.current, YOffset: e.TopInset, ScrollY: e.visibleScroll(),
		Caret: e.caretPresentation(bridge.Controller),
	})
}

func (e *Element) visibleScroll() float32 {
	if e == nil {
		return 0
	}
	return e.scroll - e.overscroll.Offset()
}

// scrollRange is the span of offsets the viewport may take. Normally it is
// the overflow: the content and the clearance its last line needs above the
// writing dock, less the viewport, and nothing when the content fits. In
// typewriter mode virtual space above and below lets the first and last
// lines sit at the center like every other line, so offsets may be
// negative.
func (e *Element) scrollRange(layout *textlayout.Layout, viewport float32) (float32, float32) {
	if e == nil || layout == nil || viewport <= 0 {
		return 0, 0
	}
	top := e.TopInset
	if e.Typewriter {
		line := e.Prose.LineHeight()
		half := (viewport - line) / 2
		first := top - half
		last := top + layout.Height() - line - half
		return first, max(first, last)
	}
	return 0, max(0, top+layout.Height()+e.BottomInset-viewport)
}

func (e *Element) placeholder(width float32) *ui.TextLayout {
	if e.Placeholder == "" || width <= 0 {
		return nil
	}
	key := placeholderKey{
		Text:       e.Placeholder,
		FontSize:   e.Prose.FontSize,
		LineHeight: e.Prose.LineHeight(),
		Width:      width,
	}
	if e.placeholderLayout != nil && e.placeholderKey == key {
		return e.placeholderLayout
	}
	font := ui.Font{
		Family:     theme.ProseFontFamily,
		Size:       e.Prose.FontSize,
		LineHeight: e.Prose.LineHeight() / e.Prose.FontSize,
	}
	e.placeholderLayout = ui.ShapeText(e.Placeholder, font, width)
	e.placeholderKey = key
	return e.placeholderLayout
}
