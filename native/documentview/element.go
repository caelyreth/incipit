package documentview

import (
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
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
	Input             TextInput
	Theme             theme.Theme
	Prose             theme.ProseStyle
	Animate           bool
	Typewriter        bool
	Label             string
	AutoFocus         bool
	Placeholder       string
	Layout            *textlayout.Cache
	Motion            *motion.Motion
	Title             string
	UIFontSize        float32
	BottomInset       float32
	scroll            float32
	lastHeight        float32
	manualUntil       time.Time
	pointerAnchor     int
	pointerHead       int
	pointer           pointerPhase
	lastWidth         float32
	focusPending      bool
	current           *textlayout.Layout
	placeholderLayout *ui.TextLayout
	placeholderKey    placeholderKey
	titleLayout       *ui.TextLayout
	titleKey          placeholderKey
	lastCaretY        float32
	lastViewportH     float32
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
	if e.AutoFocus {
		if e.focusPending {
			box.AutoFocus()
			e.focusPending = false
		}
	} else if e.focusPending {
		box.Focus()
		e.focusPending = false
	}
	box.HandleInput(e.handleInput).
		TextCaret(e.caret(c)).
		Draw(func(p *ui.Painter, rect ui.Rect) {
			e.paint(p, rect, box.Focused(), animate)
		})
	if client != nil {
		box.HandleTextInput(client)
	}
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
		offset := e.hit(event.X, event.Y)
		anchor := offset
		if event.Clicks >= 2 {
			if word, ok := e.wordAt(offset); ok {
				anchor, offset = word.Start, word.End
			}
		} else if event.Mods&ui.Shift != 0 {
			anchor = e.pointerAnchor
		} else {
			e.pointerAnchor = offset
		}
		e.pointer = pointerPressed
		e.pointerHead = offset
		return e.Input.Select(anchor, offset, event.Time) == nil
	case ui.InputPointerMove:
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
		if e.pointer == pointerIdle {
			return false
		}
		e.pointer = pointerIdle
		e.Motion.Activity(event.Time)
		return true
	case ui.InputScroll:
		e.manualScroll(event.DY, event.Time)
		e.Input.Scroll(event.DY)
		return true
	case ui.InputText, ui.InputCompose:
		e.Motion.Activity(event.Time)
		return false
	case ui.InputKeyDown, ui.InputCommand:
		e.Motion.Activity(event.Time)
		return e.Input.Keymap(event)
	default:
		return false
	}
}

func (e *Element) hit(x, y float32) int {
	if e == nil || e.current == nil {
		return 0
	}
	return int(e.current.Hit(ui.Point{
		X: x,
		Y: y + e.scroll - e.contentTop(),
	}))
}

func (e *Element) wordAt(offset int) (struct{ Start, End int }, bool) {
	var result struct{ Start, End int }
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return result, false
	}
	text := bridge.Controller.Text()
	unit, err := editor.EnclosingUTF16Range(text, model.UTF16Offset(offset), editor.SelectionWord)
	if err != nil || unit.Start >= unit.End {
		return result, false
	}
	result.Start, result.End = int(unit.Start), int(unit.End)
	return result, true
}

func (e *Element) caret(c *ui.Context) ui.Rect {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return ui.Rect{}
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

func (e *Element) paint(p *ui.Painter, rect ui.Rect, focused, animate bool) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return
	}
	frame, err := bridge.Controller.SurfaceFrame(p.Now())
	if err != nil {
		return
	}
	flow := frame.Flow()
	style := textlayout.NewStyle(e.Prose, e.Theme)
	e.lastHeight = rect.H
	if e.lastWidth != rect.W {
		// Rewrapped rows have no previous identity; never morph across them.
		e.Motion.ResetGeometry()
		e.lastWidth = rect.W
	}
	e.current = e.Layout.Update(flow, rect.W, style)
	title := e.title(rect.W)
	if title != nil {
		p.TextLayout(title, rect.X, rect.Y-e.scroll, e.Theme.Prose.Heading)
	}
	contentTop := e.contentTop()
	caret := productCaret(e.current.Caret(frame.Caret()), e.Prose)
	caret.Y += contentTop
	if e.lastCaretY != caret.Y || e.lastViewportH != rect.H {
		e.Motion.Activity(p.Now())
		e.revealCaret(e.current, caret.Y, p.Now())
		e.lastCaretY = caret.Y
		e.lastViewportH = rect.H
	}
	e.advanceScroll(p.Now())
	maxScroll := max(0, e.current.Height()+contentTop+e.BottomInset-rect.H)
	e.scroll = max(0, min(e.scroll, maxScroll))
	state := &State{
		Flow:              flow,
		Frame:             frame,
		Layout:            e.current,
		Motion:            e.Motion,
		Theme:             e.Theme,
		Prose:             e.Prose,
		ScrollY:           e.scroll,
		Focused:           focused,
		Animate:           animate,
		Placeholder:       e.Placeholder,
		PlaceholderLayout: e.placeholder(rect.W),
		PointerActive:     e.pointer != pointerIdle,
		PointerSelecting:  e.pointer == pointerSelecting,
		Composing:         bridge.Controller.IsComposing(),
		ContentTop:        contentTop,
		BottomInset:       e.BottomInset,
	}
	Paint(p, rect, state)
	bridge.Controller.SetGeometry(LayoutGeometry{
		Layout: e.current, YOffset: contentTop, ScrollY: e.scroll,
	})
}

func productCaret(caret ui.Rect, prose theme.ProseStyle) ui.Rect {
	height := min(caret.H, prose.CaretHeight())
	caret.Y += (caret.H - height) / 2
	caret.H = height
	caret.W = 2
	return caret
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

func (e *Element) title(width float32) *ui.TextLayout {
	if e.Title == "" || width <= 0 {
		return nil
	}
	key := placeholderKey{
		Text:       e.Title,
		FontSize:   e.Prose.TitleSize(),
		LineHeight: e.Prose.TitleLineHeight(),
		Width:      width,
	}
	if e.titleLayout != nil && e.titleKey == key {
		return e.titleLayout
	}
	font := ui.Font{
		Family:     theme.ProseFontFamily,
		Size:       e.Prose.TitleSize(),
		LineHeight: e.Prose.TitleLineHeight() / e.Prose.TitleSize(),
	}
	e.titleLayout = ui.ShapeText(e.Title, font, width)
	e.titleKey = key
	return e.titleLayout
}

func (e *Element) contentTop() float32 {
	if e.Title == "" {
		return 0
	}
	scale := e.UIFontSize
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	return e.Prose.TitleLineHeight() + theme.Space24.DIPs(scale)
}
