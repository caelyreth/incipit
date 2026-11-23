package page

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/motion"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// Element draws an editor and takes its input. It keeps what one page's
// view needs between frames: the scroll and a press in progress.
type Element struct {
	Editor *Editor
	// tags is the picker a tag being written opens (tags.go).
	tags tagPicker
	// tools are the selection's (selection_tools.go).
	tools toolsState
	// Theme and Prose are the colors and text measures the page shows in.
	Theme theme.Theme
	Prose theme.ProseStyle
	// Top and Bottom are the room above and below the text, and Dock how
	// far up from the view's foot a dock floating over the page reaches:
	// the view fades out there and at its top (edges.go), and a revealed
	// caret's line stays clear of both.
	Top, Bottom, Dock float32
	// LayoutWidth fixes the wrapping viewport independently of animated
	// bounds; Inset places its text within them. Zero follows the actual
	// viewport, as a standalone editor does.
	LayoutWidth, Inset float32
	// Typewriter keeps the caret's line at the viewport's center.
	Typewriter bool
	// Keys are the keymap the page's keys resolve through, and its guides
	// show; nil is the defaults.
	Keys *keymap.Keymap
	// UIFontSize sizes the guides' keycaps as the app's controls.
	UIFontSize float32
	// Pictures gives figures their pictures; nil shows each one's place.
	Pictures *Pictures

	cachedTheme   theme.Theme
	cachedPresent present.Theme
	hasTheme      bool

	client *client
	// scroll is the page's offset, which glide moves and a scroll by hand
	// sets; overscroll is its elastic pull past an edge.
	scroll        float32
	restoreScroll *float32
	glide         motion.Glide
	overscroll    motion.Elastic
	// ranged is the scroll range the last paint found, and astray whether
	// it shrank past the page, which then glides back in (stepViewport).
	ranged   [2]float32
	astray   bool
	edges    components.EdgeFade
	viewport ui.Rect
	column   float32 // the column's left, in the element
	// drag is the press the page holds until it lets go: a control's or
	// the text's (controls.go). menuChosen is whether a secondary click
	// readied the menu since the last press or key, menuOffers whether it
	// was on a control calling up offers; hotKey
	// the control the pointer worked at the last paint, springs the
	// values its objects and controls move, and memos what each unit's
	// kind keeps besides, while the unit is on the page.
	drag         drag
	menuOffers   bool
	menuChosen   bool
	hotKey       controlKey
	springs      springs
	memos        map[memoKey]any
	focusPending bool
	// held is until when a scroll by hand holds the page; typewriter and
	// viewH are the mode and the height last followed.
	held       time.Time
	typewriter bool
	viewH      float32

	// still is a page drawn by PaintStill.
	still bool
	// builds counts the views built of the element (View): a paint of an
	// earlier one, as of a container fading out after the page was built
	// elsewhere, is not the page's to draw.
	builds uint64

	// caret glides between the places the caret is drawn at, in page
	// coordinates. It, the scroll's glide and the caret's breathing move
	// in paint, on frames that paint again without building the view.
	caret motion.Rect
	// selection glides the selection's rows with the caret, in page
	// coordinates.
	selection motion.Selection
	// decos are the decorations the page draws, as last drawn, by
	// identity, so each fades in and out.
	decos      map[decoKey]*shownDeco
	decoInView map[doc.ID]bool
	decoCounts map[decoKey]int
	// decoLayout is the layout decorations were last painted in.
	decoLayout *pageLayout
	// scratch paint slices reused across frames to avoid per-unit slice allocs.
	shownUnits       []shownUnit
	shownSurfacesBuf []shownSurface
	// rooms spring each unit's height as a surface in it opens and
	// closes, stepped as the page is painted at paintNow; roomsMoving asks
	// for the next paint. animate is whether motion is wanted, as the
	// last paint found.
	rooms map[doc.ID]*room
	// flow is the page's units on their way (flows.go), and left the units
	// that left it, drawn as they go.
	flow flow
	left []left
	// sizing are the text units whose size moves (sizes.go).
	sizing map[doc.ID]bool
	// roomsOf and forgotOf are the layouts whose units the rooms and the
	// memos were last pruned to: they change only with the layout.
	roomsOf, forgotOf *pageLayout
	paintNow          time.Time
	animate           bool
	roomsMoving       bool
	// roomsSettled is whether every room of roomsOf stood still after the
	// last frame, stepped with motion roomsAnimate.
	roomsSettled bool
	roomsAnimate bool
	// pointer is where the pointer last was, in the element, pointerIn
	// whether it is over the page, and near the unit it is by (controls.go);
	// focused is whether the page had the keys when last painted.
	pointer   ui.Point
	pointerIn bool
	near      doc.ID
	focused   bool
	// drop is the line showing where pictures dragged over the page go,
	// and dropAt the place they would.
	drop   dropLine
	dropAt *doc.Caret
}

// NewElement is an element over ed.
func NewElement(ed *Editor) *Element {
	el := &Element{Editor: ed, focusPending: true}
	el.client = &client{el: el}
	return el
}

// Focus gives the page the keyboard focus in the next frame.
func (el *Element) Focus() { el.focusPending = true }

func (el *Element) metrics(width float32) Metrics {
	col := el.Prose.Measure(width)
	return Metrics{
		Body:    el.Prose.FontSize,
		Spacing: el.Prose.Spacing,
		Width:   col,
		Indent:  el.Prose.FontSize * 1.6,
		// A list's first level steps in less than a quote, its marker
		// near the text's edge.
		ListIndent: el.Prose.FontSize * 0.75,
		Gap:        el.Prose.Leading() * 0.9,
		// The margins less the gutters objects' controls stand in.
		Reach:  max((width-col)/2-el.controlGutter(), 0),
		Family: el.Prose.Family,
	}
}

// settle sets each unit's layout as its kind remembers it, stepping it on
// to the paint's time on a frame; it reports whether any still moves.
func (el *Element) settle(pl *pageLayout, frame bool) bool {
	moving := false
	for _, u := range pl.units {
		if u.kind != nil && u.kind.settle(el, u, frame) {
			moving = true
		}
	}
	return moving
}

// wheelAcross gives the wheel's sideways scroll at a page point to the
// topmost unit whose kind takes it.
func (el *Element) relayout() *pageLayout {
	ed := el.Editor
	w := el.LayoutWidth
	if w <= 0 {
		w = el.viewport.W
	}
	if w <= 0 {
		w = el.Prose.Column() + 2*el.Prose.FontSize
	}
	m := el.metrics(w)
	if el.still {
		m.Width, m.Reach = w, 0
	}
	if !el.hasTheme || el.cachedTheme != el.Theme {
		el.cachedTheme = el.Theme
		el.cachedPresent = Theme(el.Theme)
		el.hasTheme = true
	}
	ed.SetStyle(el.cachedTheme, el.cachedPresent, m)
	el.column = max((w-m.Width)/2, 0)
	if el.LayoutWidth > 0 {
		el.column = el.Inset
	}
	pl := ed.Relayout()
	el.settleRooms(pl, el.paintNow, false, el.animate)
	el.settle(pl, false)
	el.applyRestoredScroll(pl)
	return pl
}

// ScrollY is the page's parked scroll offset, without the elastic pull
// of an overscroll in progress.
func (el *Element) Still(width float32) (float32, float32) {
	el.still = true
	el.LayoutWidth = 0
	el.viewport.W = width
	el.Top, el.Bottom, el.Dock = 0, 0, 0
	el.relayout()
	return el.textSize()
}

// PaintStill draws the page Still laid out into r, the caret shown and
// moving as on a page being written.
func (el *Element) PaintStill(p *ui.Painter, r ui.Rect) {
	el.viewport = r
	el.paint(p, r, true)
	el.paintCaretInView(p, r, true)
}

// followPointer has the objects follow the pointer, c timing what waits
// for it to rest.
func (el *Element) followPointer(c components.Clock) {
	for _, u := range el.Editor.layout.units {
		if u.kind != nil {
			u.kind.follow(el, c, u)
		}
	}
}

func (el *Element) textSize() (float32, float32) {
	pl := el.Editor.layout
	if pl == nil {
		return 0, 0
	}
	var w float32
	for _, u := range pl.units {
		for _, s := range u.surfaces {
			w = max(w, u.x+s.x+s.reach)
		}
	}
	return w, pl.height
}

// View builds the element.
func (el *Element) View(c *ui.Context) {
	ed := el.Editor
	if ed == nil {
		return
	}
	ed.Wake = c.Services().Invalidate
	ed.Due(c.Now())
	pl := el.relayout()
	el.follow(pl, c.Now())
	el.builds++
	build := el.builds
	ui.Column(c).Key("page-surface").Fill().Role(ui.RoleNone).Children(func() {
		box := ui.Box(c).Key("page-editor").Fill().FocusRing(false).
			Role(ui.RoleTextField).Label(locales.Page()).Value(el.fieldValue()).Focusable()
		// While the tools read the keys, the page takes their text itself,
		// with no input method composing it (toolKey, toolText).
		if el.tools.keyed() {
			box.HandleTextInput(nil).InputMethod(false)
		} else {
			box.HandleTextInput(el.client)
		}
		if el.focusPending {
			box.Focus(ui.FocusOptions{PreventScroll: true})
			el.focusPending = false
		}
		services := c.Services()
		if el.Pictures != nil {
			el.Pictures.Wake = services.Invalidate
		}
		el.takeDrops(box)
		// The objects' controls follow the pointer; what they open moves
		// at the paint's time (step).
		el.pointerIn = box.Hovered()
		el.near = el.nearAt(el.toPage(el.pointer.X, el.pointer.Y))
		el.followPointer(c)
		if cursor, ok := el.cursorAt(el.toPage(el.pointer.X, el.pointer.Y)); ok {
			box.Cursor(cursor)
		}
		box.ContextMenu(func(m *ui.Menu) { el.contextMenu(m, services) })
		// The paint reads what the build knew, never the build's elements:
		// a container leaving keeps painting what it held after the frames
		// that built it, as a pane fading out. Such a paint draws the page
		// only while it was built nowhere since and still has its view; it
		// never lays the page out again for its own size.
		focused := box.Focused()
		box.HandleInput(func(ev ui.InputEvent) bool { return el.handle(ev, services) }).
			TextCaret(el.caretInElement()).
			Children(func() {
				ui.Box(c).Key("page-content").Fill().Role(ui.RoleNone).Background(el.Theme.Surface.Page).
					Draw(func(p *ui.Painter, r ui.Rect) {
						if build != el.builds || ed.closed {
							return
						}
						if r.W != el.viewport.W || r.H != el.viewport.H {
							el.viewport = r
							el.relayout()
							services.Invalidate()
						}
						el.viewport = r
						el.paint(p, r, focused)
					})
			}).DrawOver(func(p *ui.Painter, r ui.Rect) {
			if build != el.builds || ed.closed {
				return
			}
			el.paintCaretInView(p, r, focused)
			el.paintEdges(p, r)
		})
		el.accessUnits(c)
		el.syncTags()
		if box.Focused() {
			el.tagPickerView(c, services)
		}
		el.selectionToolsView(c, pl, box.Focused())
	})
}

// fieldValue is the text the accessibility text field exposes: the
// surface the caret is in, which is the surface the text client edits.
func (el *Element) toPage(x, y float32) (float32, float32) {
	return x - el.column, y - el.Top + el.offset()
}

// fromPage is a page rectangle in element coordinates.
func (el *Element) fromPage(r ui.Rect) ui.Rect {
	r.X += el.column
	r.Y += el.Top - el.offset()
	return r
}

func (el *Element) caretInElement() ui.Rect {
	pl := el.Editor.layout
	if pl == nil {
		return ui.Rect{}
	}
	r, ok := pl.caretBox(el.shownSelection().Head, el.Editor.metrics)
	if !ok {
		return ui.Rect{}
	}
	return el.fromPage(r)
}

// --- scrolling ------------------------------------------------------------

// manualHold is how long a scroll by hand holds the page against the
// caret's line.
const manualHold = 650 * time.Millisecond

// offset is where the page is drawn from: its scroll, less its pull past
// an edge.
func (el *Element) stepCaret(pl *pageLayout, now time.Time, animate bool) bool {
	cm, ok := pl.caretMetrics(el.shownSelection().Head)
	if !ok {
		el.caret.Hide()
		return false
	}
	if !el.caret.Shown() {
		// A caret appearing is activity: solid a moment, then breathing,
		// as on a page just opened before any input.
		el.Editor.active = now
	}
	if pc, ok := pl.lookup(el.shownSelection().Head); ok && pc.unit.kind != nil && pc.surface != nil {
		if x, ok := pc.unit.kind.caretX(el, pc.unit, pc.surface); ok {
			cm.rect.X = x
		}
	}
	el.caret.Step(el.caretShape(cm), now, caretFollow, animate)
	return el.caret.Moving()
}

// caretAscentShare is the portion of the caret's height reaching above the
// line's baseline; anchoring here keeps mixed-script fallbacks and headings
// optically centered on the baseline rather than dragged by taller descenders.
const caretAscentShare = float32(0.78)

// caretShape is the caret drawn in a text row: two DIPs wide, anchored to
// the line's baseline and scaled to the surface's font size; a gap's bar
// keeps its shape.
func (el *Element) caretShape(cm caretMetric) ui.Rect {
	row := cm.rect
	if cm.kind != spotText || row.W > 2 {
		return row
	}
	scale := cm.scale(el.Prose.FontSize)
	height := el.Prose.CaretHeight() * scale
	if cm.hasBaseline {
		return ui.Rect{X: row.X, Y: cm.baseline - height*caretAscentShare, W: 2, H: height}
	}
	h := row.H
	if h <= 0 {
		h = el.Prose.LineHeight() * scale
	}
	return ui.Rect{X: row.X, Y: row.Y + (h-height)/2 + el.Prose.CaretOffsetY()*scale, W: 2, H: height}
}

// --- input ----------------------------------------------------------------

func (el *Element) handle(ev ui.InputEvent, services ui.Services) bool {
	ed := el.Editor
	at := ev.Time
	if at.IsZero() {
		at = time.Now()
	}
	if ev.Kind == ui.InputPointerDown || ev.Kind == ui.InputKeyDown {
		// The menu a secondary click chose lasts until the next press.
		el.menuOffers, el.menuChosen = false, false
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		x, y := el.toPage(ev.X, ev.Y)
		if ev.Button == 1 {
			// A secondary click readies the menu for where it is.
			el.readyMenu(x, y, at)
			services.Invalidate()
			return false
		}
		if ev.Button != 0 {
			return false
		}
		if !el.press(x, y, ev.Clicks, ev.Mods&ui.Shift != 0, at) {
			return false
		}
		el.Focus()
		services.Invalidate()
		return true
	case ui.InputPointerMove:
		el.pointer = ui.Point{X: ev.X, Y: ev.Y}
		x, y := el.toPage(ev.X, ev.Y)
		if el.drag != nil {
			el.drag.move(x, y, at)
			services.Invalidate()
			return true
		}
		// The controls of what the pointer is by follow it.
		was := el.near
		el.near = el.nearAt(x, y)
		if was != 0 || el.near != 0 {
			services.Invalidate()
		}
		return false
	case ui.InputPointerUp:
		if !el.letGo(ev.Canceled, at) {
			return false
		}
		services.Invalidate()
		return true
	case ui.InputScroll:
		// Sideways, the wheel scrolls what the pointer is over across, as
		// its kind takes it; Shift turns it sideways.
		dx := ev.DX
		if ev.Mods&ui.Shift != 0 && dx == 0 {
			dx = ev.DY
		}
		if ev.Precise && !ev.Momentum && ev.DX == 0 && ev.DY == 0 {
			// The fingers lifted: whatever they held lets go, a table
			// scrolled across under them and the page alike.
			x, y := el.toPage(el.pointer.X, el.pointer.Y)
			el.wheelAcross(x, y, ev, 0)
			el.scrollBy(ev, at)
			services.Invalidate()
			return true
		}
		if x, y := el.toPage(el.pointer.X, el.pointer.Y); dx != ev.DX || math.Abs(float64(dx)) > math.Abs(float64(ev.DY)) {
			if el.wheelAcross(x, y, ev, dx) {
				services.Invalidate()
				return true
			}
			// Sideways, with nothing under it to scroll across, the page
			// leaves the scroll to what is around it, as a strip of pages.
			return false
		}
		el.scrollBy(ev, at)
		services.Invalidate()
		return true
	case ui.InputText:
		return el.toolText(ev.Text)
	case ui.InputKeyDown:
		el.tools.took = false
		if ev.Key == ui.KeyEscape && el.letGo(true, at) {
			services.Invalidate()
			return true
		}
		if el.toolKey(ev) {
			services.Invalidate()
			return true
		}
		if ev.Key == ui.KeyEscape && el.tools.shown {
			// Escape puts the selection's tools away before it does
			// anything to the selection.
			el.tools.summoned, el.tools.dismissed = false, true
			services.Invalidate()
			return true
		}
		if ed.deco.Composition != nil {
			// The input method has the keys while it composes.
			return false
		}
		if el.tagKeys(ev, services, at) {
			services.Invalidate()
			return true
		}
		if in, ok := el.keyIntent(ev); ok && el.run(in, services, at) {
			services.Invalidate()
			return true
		}
		return false
	case ui.InputCommand:
		if el.command(ev.Text, services, at) {
			services.Invalidate()
			return true
		}
		return false
	}
	return false
}

// command answers an edit-menu command.
func (el *Element) command(name string, services ui.Services, at time.Time) bool {
	if name == "copy" {
		return el.Editor.writeClipboard(services)
	}
	in, ok := menuIntents[name]
	return ok && el.run(in, services, at)
}

// --- painting -------------------------------------------------------------
