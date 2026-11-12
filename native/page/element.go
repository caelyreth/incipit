package page

import (
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/vocab"
	"math"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Element draws an editor and takes its input. It keeps what one page's
// view needs between frames: the scroll and a press in progress.
type Element struct {
	Editor *Editor
	// Theme and Prose are the colors and text measures the page shows in.
	Theme theme.Theme
	Prose theme.ProseStyle
	// Top and Bottom are the room above and below the text.
	Top, Bottom float32
	Label       string
	// Typewriter keeps the caret's line at the viewport's center.
	Typewriter bool
	// Keys are the keymap the page's keys resolve through, and its guides
	// show; nil is the defaults.
	Keys *keymap.Keymap
	// UIFontSize sizes the guides' keycaps as the app's controls.
	UIFontSize float32
	// Pictures gives figures their pictures; nil shows each one's place.
	Pictures *Pictures

	client *client
	// scroll is the page's offset, which glide moves and a scroll by hand
	// sets; overscroll is its elastic pull past an edge.
	scroll        float32
	restoreScroll *float32
	glide         motion.Glide
	overscroll    scrollfx.Elastic
	viewport      ui.Rect
	column        float32 // the column's left, in the element
	gesture       *gesture
	focusPending  bool
	// held is until when a scroll by hand holds the page; typewriter and
	// viewH are the mode and the height last followed.
	held       time.Time
	typewriter bool
	viewH      float32

	// still is a page drawn by PaintStill.
	still bool
	// guides are the key hints as last painted.
	guides guides

	// caret glides between the places the caret is drawn at, in page
	// coordinates. It, the scroll's glide and the caret's breathing move
	// in paint, on frames that paint again without building the view.
	caret motion.Rect
	// selection glides the selection's rows with the caret, in page
	// coordinates.
	selection motion.Selection
	// decos are the decorations the page draws, as last drawn, by
	// identity, so each fades in and out.
	decos map[decoKey]*shownDeco
	// figures are the figures' springs, by figure; hover is the figure
	// under the pointer, and focused whether the page had the keys when
	// last painted.
	figures map[doc.ID]*figureMotion
	// rooms spring each unit's height as a surface in it opens and
	// closes, stepped as the page is painted at paintNow; roomsMoving asks
	// for the next paint. maths are the formulas' fades. animate is
	// whether motion is wanted, as the last paint found.
	rooms       map[doc.ID]*room
	maths       map[doc.ID]*mathMotion
	paintNow    time.Time
	animate     bool
	roomsMoving bool
	hover       doc.ID
	pointer     ui.Point
	focused     bool
	// drop is the line showing where pictures dragged over the page go,
	// and dropAt the place they would.
	drop   dropLine
	dropAt *doc.Caret
}

// NewElement is an element over ed.
func NewElement(ed *Editor) *Element {
	el := &Element{Editor: ed, Label: "Page", focusPending: true, figures: map[doc.ID]*figureMotion{}, maths: map[doc.ID]*mathMotion{}}
	el.client = &client{el: el}
	return el
}

// Focus gives the page the keyboard focus in the next frame.
func (el *Element) Focus() { el.focusPending = true }

func (el *Element) metrics(width float32) Metrics {
	col := min(el.Prose.Column(), max(width-2*el.Prose.FontSize, el.Prose.FontSize*8))
	return Metrics{
		Body:    el.Prose.FontSize,
		Spacing: el.Prose.Spacing,
		Width:   col,
		Indent:  el.Prose.FontSize * 1.6,
		Gap:     el.Prose.Leading() * 0.9,
	}
}

// relayout lays the page out at the element's width.
func (el *Element) relayout() *pageLayout {
	ed := el.Editor
	w := el.viewport.W
	if w <= 0 {
		w = el.Prose.Column() + 2*el.Prose.FontSize
	}
	m := el.metrics(w)
	if el.still {
		m.Width = w
	}
	ed.SetStyle(el.Theme.Surface, Theme(el.Theme), m)
	el.column = max((w-m.Width)/2, 0)
	pl := ed.Relayout()
	el.settleRooms(pl, el.paintNow, false, el.animate)
	el.applyRestoredScroll(pl)
	return pl
}

// ScrollY is the page's parked scroll offset, without the elastic pull
// of an overscroll in progress.
func (el *Element) ScrollY() float32 {
	if el.restoreScroll != nil {
		return *el.restoreScroll
	}
	return el.scroll
}

// RestoreScroll places the page at a saved offset after its next layout.
// Applying it after layout clamps it to the page's current extent.
func (el *Element) RestoreScroll(y float32) { el.restoreScroll = &y }

func (el *Element) applyRestoredScroll(pl *pageLayout) {
	if el.restoreScroll == nil || el.viewport.W <= 0 || el.viewport.H <= 0 {
		return
	}
	low, high := el.scrollRange(pl)
	el.scroll = max(low, min(*el.restoreScroll, high))
	el.restoreScroll = nil
	el.glide.Cancel()
	el.overscroll.Reset()
	// The restored offset was saved at this viewport height. Mark the
	// height as followed so the next view does not mistake opening the
	// page for a resize and reveal the caret over the saved position.
	el.typewriter, el.viewH = el.Typewriter, el.viewport.H
}

// Still lays the page out at width as a showcase stage draws a story,
// without taking input: the text at the whole width from its top. It
// returns the text's extent: its widest line and its height.
func (el *Element) Still(width float32) (float32, float32) {
	el.still = true
	el.viewport.W = width
	el.Top, el.Bottom = 0, 0
	el.relayout()
	return el.textSize()
}

// PaintStill draws the page Still laid out into r, the caret shown and
// moving as on a page being written.
func (el *Element) PaintStill(p *ui.Painter, r ui.Rect) {
	el.viewport = r
	el.paint(p, r, true)
}

func (el *Element) textSize() (float32, float32) {
	pl := el.Editor.layout
	if pl == nil {
		return 0, 0
	}
	var w float32
	for _, u := range pl.units {
		for _, s := range u.surfaces {
			for _, line := range s.layout.Lines() {
				w = max(w, u.x+s.x+line.Rect.X+line.Rect.W)
			}
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
	ui.Column(c).Key("page-surface").Fill().Role(ui.RoleNone).Children(func() {
		box := ui.Box(c).Key("page-editor").Fill().FocusRing(false).
			Role(ui.RoleTextField).Label(el.Label).Value(el.fieldValue()).Focusable()
		box.HandleTextInput(el.client)
		if el.focusPending {
			box.AutoFocus()
			box.Focus()
			el.focusPending = false
		}
		services := c.Services()
		if el.Pictures != nil {
			el.Pictures.Wake = services.Invalidate
		}
		el.takeDrops(box)
		if el.Editor.resize != nil || el.hoverGrip() {
			box.Cursor(ui.CursorResizeEW)
		}
		box.HandleInput(func(ev ui.InputEvent) bool { return el.handle(ev, services) }).
			TextCaret(el.caretInElement()).
			Draw(func(p *ui.Painter, r ui.Rect) {
				if r.W != el.viewport.W || r.H != el.viewport.H {
					el.viewport = r
					el.relayout()
					c.Services().Invalidate()
				}
				el.viewport = r
				el.paint(p, r, box.Focused())
			})
		el.accessUnits(c)
	})
}

// fieldValue is the text the accessibility text field exposes: the
// surface the caret is in, which is the surface the text client edits.
func (el *Element) fieldValue() string {
	if el.Editor == nil || el.Editor.layout == nil {
		return ""
	}
	pc, ok := el.Editor.layout.lookup(el.Editor.Selection().Head)
	if !ok || pc.surface == nil {
		return ""
	}
	return pc.surface.text
}

// accessUnits projects each presentation unit as a positioned node in the
// page's accessibility tree. The boxes draw nothing and take no input;
// their roles, names, values and bounds let assistive technology move
// through the document by its units while the page's text client remains
// its one editing surface.
func (el *Element) accessUnits(c *ui.Context) {
	pl := el.Editor.layout
	if pl == nil {
		return
	}
	top := el.Top - el.offset()
	for _, u := range pl.units {
		p := u.pres
		role, description := accessRole(p)
		width := max(el.Editor.metrics.Width-u.x, 1)
		height := max(u.h, 1)
		node := ui.Box(c).Key("page-unit-" + p.Unit.String()).Absolute().
			Left(el.column + u.x).Top(top + u.y).Width(width).Height(height).
			Role(role).Label(p.Label).Value(p.Value).RoleDescription(description).
			IgnoreInput(true)
		if p.Level > 0 {
			node.Level(p.Level)
		}
	}
}

// accessRole is a presentation role's platform role and the app's name
// for it where the platform's is broader.
func accessRole(p *present.Presentation) (ui.Role, string) {
	switch p.Role {
	case "heading":
		return ui.RoleHeading, ""
	case "listitem":
		return ui.RoleListItem, ""
	case "code":
		return ui.RoleText, "code"
	case "math":
		return ui.RoleText, "math"
	case "separator":
		return ui.RoleSeparator, ""
	case "figure":
		return ui.RoleImage, ""
	case "table":
		return ui.RoleTable, ""
	case "cell":
		return ui.RoleCell, ""
	default:
		return ui.RoleText, ""
	}
}

// toPage is an element point in page coordinates.
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
func (el *Element) offset() float32 { return el.scroll - el.overscroll.Offset() }

// scrollRange is how far the page scrolls. In typewriter mode room above
// and below lets the first and last lines sit at the center like every
// other line, so offsets may be negative; otherwise the page scrolls over
// what overflows.
func (el *Element) scrollRange(pl *pageLayout) (float32, float32) {
	if pl == nil || el.viewport.H <= 0 {
		return 0, 0
	}
	if el.Typewriter {
		line := el.Prose.LineHeight()
		half := (el.viewport.H - line) / 2
		first := el.Top - half
		return first, max(first, el.Top+pl.height-line-half)
	}
	return 0, max(0, el.Top+pl.height+el.Bottom-el.viewport.H)
}

// follow keeps the caret's line in view by one rule: the page moves for
// what the writer does, never on its own. It reveals the line after an
// intent, a resize or a change of mode; typewriter mode centers it.
// Leaving the mode coasts whatever glide is under way into the plain
// range, at its speed, rather than finishing it and snapping.
func (el *Element) follow(pl *pageLayout, now time.Time) {
	ed := el.Editor
	modeChanged := el.typewriter != el.Typewriter
	resized := el.viewH != el.viewport.H
	el.typewriter, el.viewH = el.Typewriter, el.viewport.H
	if modeChanged && !el.Typewriter {
		low, high := el.scrollRange(pl)
		el.glide.Settle(el.scroll, low, high, now)
		ed.reveal = false
		return
	}
	if !ed.reveal && !resized && !modeChanged {
		return
	}
	ed.reveal = false
	el.revealCaret(pl, now)
}

// revealCaret glides the caret's whole line into view, from where the
// page is and at the speed it moves.
func (el *Element) revealCaret(pl *pageLayout, now time.Time) {
	if el.viewport.H <= 0 {
		return
	}
	r, ok := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if !ok {
		return
	}
	line, y := max(r.H, el.Prose.LineHeight()), el.Top+r.Y
	if !el.Typewriter && y < el.Top+line {
		// The room above the first line is that line's: revealing it
		// reveals the page's top.
		line, y = line+y, 0
	}
	target, changed := motion.Reveal(el.scroll, el.viewport.H, y, line, el.Typewriter, now.Before(el.held))
	if !changed {
		return
	}
	low, high := el.scrollRange(pl)
	target = max(low, min(target, high))
	if target == el.scroll && !el.glide.Moving() {
		return
	}
	el.glide.To(el.scroll, target, now)
}

// stepViewport brings the glide and the pull past an edge to now, and
// reports whether either still moves.
func (el *Element) stepViewport(now time.Time, animate bool) bool {
	_, pulling := el.overscroll.Step(now, animate)
	var gliding bool
	el.scroll, gliding = el.glide.Step(el.scroll, now, animate)
	return pulling || gliding
}

// scrollBy scrolls by hand: it stops a glide, holds the page against the
// caret a moment, and pulls elastic past an edge.
func (el *Element) scrollBy(ev ui.InputEvent, now time.Time) {
	el.glide.Cancel()
	el.held = now.Add(manualHold)
	low, high := el.scrollRange(el.Editor.layout)
	el.scroll = components.OverscrollEditor.Scroll(&el.overscroll, el.scroll, ev.DY, low, high, el.viewport.H, 0,
		scrollfx.Phase(ev.Precise, ev.Momentum))
}

// --- animation ------------------------------------------------------------

// stepCaret moves the drawn caret toward the selection head's place; it
// reports whether it still moves.
func (el *Element) stepCaret(pl *pageLayout, now time.Time, animate bool) bool {
	r, ok := pl.caretBox(el.shownSelection().Head, el.Editor.metrics)
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
			r.X = x
		}
	}
	el.caret.Step(el.caretShape(r), now, motion.CaretFrequency, animate)
	return el.caret.Moving()
}

// caretShape is the caret drawn in a text row: two DIPs wide, as tall as
// the prose's caret, growing with a row of larger text and centered in it
// with the prose's small optical drop. A gap's bar keeps its shape.
func (el *Element) caretShape(row ui.Rect) ui.Rect {
	if row.W > 2 {
		return row
	}
	line := el.Prose.LineHeight()
	h := row.H
	if h <= 0 {
		h = line
	}
	scale := max(1, h/line)
	height := el.Prose.CaretHeight() * scale
	return ui.Rect{X: row.X, Y: row.Y + (h-height)/2 + el.Prose.CaretOffsetY()*scale, W: 2, H: height}
}

// --- input ----------------------------------------------------------------

func (el *Element) handle(ev ui.InputEvent, services ui.Services) bool {
	ed := el.Editor
	at := ev.Time
	if at.IsZero() {
		at = time.Now()
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		x, y := el.toPage(ev.X, ev.Y)
		if el.beginResize(x, y) {
			el.Focus()
			services.Invalidate()
			return true
		}
		if !el.beginGesture(x, y, ev.Clicks, ev.Mods&ui.Shift != 0, at) {
			return false
		}
		el.Focus()
		services.Invalidate()
		return true
	case ui.InputPointerMove:
		x, y := el.toPage(ev.X, ev.Y)
		if r := ed.resize; r != nil {
			if span := r.spanAt(x); span != r.span {
				r.span = span
				services.Invalidate()
			}
			return true
		}
		el.hoverAt(x, y, services)
		if !el.moveGesture(x, y, at) {
			return false
		}
		services.Invalidate()
		return true
	case ui.InputPointerUp:
		if ed.resize != nil {
			el.finishResize(ev.Canceled, at)
			services.Invalidate()
			return true
		}
		if el.gesture == nil {
			return false
		}
		el.finishGesture(ev.Canceled, at)
		services.Invalidate()
		return true
	case ui.InputScroll:
		el.scrollBy(ev, at)
		services.Invalidate()
		return true
	case ui.InputKeyDown:
		if (el.gesture != nil || ed.resize != nil) && ev.Key == ui.KeyEscape {
			el.finishGesture(true, at)
			el.finishResize(true, at)
			services.Invalidate()
			return true
		}
		if ed.deco.Composition != nil {
			// The input method has the keys while it composes.
			return false
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

func (el *Element) paint(p *ui.Painter, r ui.Rect, focused bool) {
	ed := el.Editor
	pl := ed.layout
	if pl == nil {
		return
	}
	// What moves with time steps at the paint's own time: a frame that
	// paints again without building the view moves it on.
	now, animate := p.Now(), !p.Preferences().ReduceMotion
	el.focused, el.paintNow, el.animate = focused, now, animate
	el.settleRooms(pl, now, true, animate)
	if el.roomsMoving {
		p.AnimationFrame()
	}
	el.previewGesture(pl)
	if el.stepViewport(now, animate) {
		p.AnimationFrame()
	}
	if el.stepCaret(pl, now, animate) {
		p.AnimationFrame()
	}
	t := el.Theme
	sel := el.shownSelection()
	ox, oy := r.X+el.column, r.Y+el.Top-el.offset()
	top, bottom := r.Y, r.Y+r.H
	m := ed.metrics
	paints := el.shownPaints(pl)
	// In layers, the same for every surface: what units draw and the
	// panels their surfaces stand on, then every decoration and the
	// selection over them, then the surfaces' text over those.
	type shownUnit struct {
		u        *unitLayout
		ux, uy   float32
		surfaces []shownSurface
	}
	var shown []shownUnit
	for _, u := range pl.units {
		uy := oy + u.y
		if uy > bottom || uy+u.h < top {
			continue
		}
		ux := ox + u.x
		// Quote bars, one per quote level.
		for q := 0; q < u.pres.Context.Quotes; q++ {
			bx := ox + float32(u.pres.Context.Indent+q)*m.Indent + m.Indent*0.3
			p.Fill(ui.Rect{X: bx, Y: uy - m.Gap/2, W: 2, H: u.h + m.Gap}, t.Prose.Subtle, 1)
		}
		panel := float32(0)
		if u.kind != nil {
			panel = u.kind.paint(el, p, u, ux, uy)
		}
		su := shownUnit{u: u, ux: ux, uy: uy, surfaces: el.shownSurfaces(u, panel)}
		el.clipUnit(p, u, ox, uy, func() {
			for _, ss := range su.surfaces {
				if ss.panel {
					el.paintPanel(p, ss.s, ux, uy)
				}
			}
		})
		shown = append(shown, su)
	}
	el.paintDecos(p, pl, paints, ox, oy, top, bottom, now, animate)
	// The selection's text glides beneath the text; a drag places it
	// directly, so it follows the pointer and glides on from there. What
	// else it covers is painted as every paint is.
	rows := el.selection.Step(selectionRows(pl, paints), now, animate && el.gesture == nil)
	if el.selection.Moving() {
		p.AnimationFrame()
	}
	wash := t.Prose.Selection.Alpha(el.selection.Opacity())
	for _, row := range rows {
		p.Fill(ui.Rect{X: ox + row.X, Y: oy + row.Y, W: row.W, H: row.H}, wash, 2)
	}
	for _, su := range shown {
		el.clipUnit(p, su.u, ox, su.uy, func() {
			for _, ss := range su.surfaces {
				el.paintSurface(p, ss.s, su.ux, su.uy, ss.panel, ss.a)
			}
		})
	}
	el.paintOutlines(p, ox, oy, now, animate)
	el.sweepMaths()
	el.sweepFigures()
	if el.paintDrop(p, ox, oy, m, now, animate) {
		p.AnimationFrame()
	}
	if el.stepGuides(pl, focused, now, animate) {
		p.AnimationFrame()
	}
	el.paintGuides(p, r, ox, oy)
	if focused && sel.Kind == doc.SelectText && el.caret.Shown() {
		cr := el.caret.Value()
		cr.X += ox
		cr.Y += oy
		el.paintCaret(p, r, cr, sel.IsCollapsed(), now, animate)
	}
}

// paintSurface draws one of a unit's surfaces at a strength: a panel, as
// a card or a math block's source, on its inset fill.
func (el *Element) paintSurface(p *ui.Painter, s *surfaceLayout, ux, uy float32, panel bool, a float32) {
	sx, sy := ux+s.x, uy+s.y
	area := ui.Rect{X: sx, Y: sy, W: s.w, H: s.h}
	if panel {
		area = el.panelArea(s, ux, uy)
	}
	p.TextLayout(s.layout, sx, sy, el.Theme.Prose.Text)
	el.paintGhosts(p, s, sx, sy)
	el.paintBoxes(p, s, sx, sy)
	el.veil(p, area, a)
}

// shownSurface is a surface as the page shows it: on a panel or not, at
// a strength.
type shownSurface struct {
	s     *surfaceLayout
	panel bool
	a     float32
}

// shownSurfaces are the surfaces of u the page shows: its text; a card
// as far as it has opened, and one closed fading out where it stood; a
// panel surface, as a math block's source, as strongly as its object
// shows it (panel).
func (el *Element) shownSurfaces(u *unitLayout, panel float32) []shownSurface {
	var out []shownSurface
	open := el.opened(u)
	for _, s := range u.surfaces {
		switch {
		case s.panel:
			if panel > 0.001 {
				out = append(out, shownSurface{s, true, panel})
			}
		case !s.shown:
		case s.card:
			out = append(out, shownSurface{s, true, open})
		default:
			out = append(out, shownSurface{s, false, 1})
		}
	}
	if prev, a := el.closing(u); prev != nil && a > 0.001 {
		for i, s := range prev.surfaces {
			if s.card && s.shown && (i >= len(u.surfaces) || !u.surfaces[i].shown) {
				out = append(out, shownSurface{s, true, a})
			}
		}
	}
	return out
}

// clipUnit paints within the room u stands in while it springs.
func (el *Element) clipUnit(p *ui.Painter, u *unitLayout, ox, uy float32, paint func()) {
	m := el.Editor.metrics
	p.Clip(ui.Rect{X: ox - cardPad(m), Y: uy, W: m.Width + 2*cardPad(m), H: u.shown}, 0, paint)
}

// panelArea is the inset a panel surface stands on.
func (el *Element) panelArea(s *surfaceLayout, ux, uy float32) ui.Rect {
	m := el.Editor.metrics
	return ui.Rect{X: ux + s.x - cardPad(m)/2, Y: uy + s.y - cardPad(m)/2, W: s.w + cardPad(m), H: s.h + cardPad(m)}
}

// paintPanel lays the inset a panel surface stands on, beneath the
// decorations over its text.
func (el *Element) paintPanel(p *ui.Painter, s *surfaceLayout, ux, uy float32) {
	p.Fill(el.panelArea(s, ux, uy), el.Theme.Surface.Inset, 6)
}

// decoKey is a decoration's identity across frames: where it is, where it
// comes from, how it looks, and which of its likes there it is. Its range
// is not part of it, so a run that grows as it is typed keeps its fade.
type decoKey struct {
	unit      doc.ID
	surface   int
	highlight bool
	origin    uint64
	direction vocab.Direction
	duration  time.Duration
	look      deco
	nth       int
}

// shownDeco is a decoration as last drawn, fading in while it is there
// and out where it last stood once it is not.
type shownDeco struct {
	fade      motion.Fade
	rows      []decoRow
	look      deco
	direction vocab.Direction
	duration  time.Duration
	started   time.Time
	seen      bool
	alpha     float32
}

// decoRow is one line's piece of a decoration, in page coordinates: the
// box hugging its glyphs and the line's baseline and descent.
type decoRow struct {
	box           ui.Rect
	base, descent float32
	// object marks an object's piece, drawn around its box, rounded by
	// radius (0: outlineRadius's).
	object bool
	radius float32
}

// paintDecos draws what the page draws itself beneath the text, from each
// surface's styles and the decorations' highlights alike: a background a
// run per line, hugging the line's glyphs and rounded at its ends, and a
// line below them. Each fades in as it appears and out as it goes.
func (el *Element) paintDecos(p *ui.Painter, pl *pageLayout, paints []present.Paint, ox, oy, top, bottom float32, now time.Time, animate bool) {
	if el.decos == nil {
		el.decos = map[decoKey]*shownDeco{}
	}
	for _, sd := range el.decos {
		sd.seen = false
	}
	counts := map[decoKey]int{}
	note := func(u *unitLayout, surface int, highlight bool, d deco, origin uint64, direction vocab.Direction, duration time.Duration, visible bool, rows func() []decoRow) {
		look := d
		look.rg = ui.TextInputRange{}
		k := decoKey{unit: u.pres.Unit, surface: surface, highlight: highlight, origin: origin,
			direction: direction, duration: duration, look: look}
		k.nth = counts[k]
		counts[k]++
		sd, ok := el.decos[k]
		if !ok {
			sd = &shownDeco{look: look, direction: direction, duration: duration, started: now}
			el.decos[k] = sd
		}
		sd.seen = true
		if visible || sd.rows == nil {
			sd.rows = rows()
		}
	}
	visible := func(u *unitLayout) bool {
		uy := oy + u.y
		return !(uy > bottom || uy+u.h < top)
	}
	for _, pt := range paints {
		d := decoOf(ui.TextInputRange{}, el.Editor.theme.Resolve([]vocab.Group{pt.Group}))
		if !d.drawn() {
			continue
		}
		for _, c := range pt.Covers {
			u := pl.unitOf(c.Unit)
			if u == nil {
				continue
			}
			for _, pc := range pl.pieces(u, c) {
				if pt.Group == vocab.GroupSelection && pc.surface >= 0 {
					// The selection's text glides with the caret.
					continue
				}
				note(u, pc.surface, true, d, pt.Origin, pt.Direction, pt.Duration, visible(u), func() []decoRow { return pc.rows })
			}
		}
	}
	for _, u := range pl.units {
		vis := visible(u)
		for _, s := range u.surfaces {
			if !s.shown {
				continue
			}
			for _, d := range s.decos {
				note(u, s.index, false, d, 0, 0, 0, vis, func() []decoRow { return decoRows(s, d.rg, u.x+s.x, u.y+s.y) })
			}
		}
	}
	for k, sd := range el.decos {
		if sd.duration > 0 && sd.seen {
			sd.alpha = pulseAlpha(now.Sub(sd.started), sd.duration, animate)
			sd.fade.Set(sd.alpha)
			if animate && now.Before(sd.started.Add(sd.duration)) {
				p.AnimationFrame()
			}
		} else {
			sd.alpha = sd.fade.Step(sd.seen, now, animate)
			if sd.fade.Moving() {
				p.AnimationFrame()
			}
			if sd.fade.Gone() {
				delete(el.decos, k)
			}
		}
	}
	// In layers, whatever order they were found in: the text's backgrounds,
	// the highlights' over them, then every line over both.
	for _, layer := range []func(k decoKey) bool{
		func(k decoKey) bool { return !k.highlight },
		func(k decoKey) bool { return k.highlight },
	} {
		for k, sd := range el.decos {
			if layer(k) {
				el.paintBackground(p, sd, ox, oy, now, animate)
			}
		}
	}
	for _, sd := range el.decos {
		el.paintLine(p, sd, ox, oy, now, animate)
	}
}

// pulseSweep is how quickly a directional decoration uncovers its range.
// It matches the pulse's fast entrance.
const pulseSweep = 90 * time.Millisecond

// pulseAlpha gives a pulse its fast entrance, short hold and long decay.
// The decay reaches zero at duration, so the annotation's deadline removes
// something already invisible.
func pulseAlpha(age, duration time.Duration, animate bool) float32 {
	if !animate || duration <= 0 {
		return 1
	}
	age = max(age, 0)
	if age >= duration {
		return 0
	}
	in := min(90*time.Millisecond, duration/4)
	out := min(280*time.Millisecond, duration/2)
	hold := duration - in - out
	if age < in {
		t := float32(age) / float32(in)
		rest := 1 - t
		return 1 - rest*rest*rest
	}
	if age < in+hold {
		return 1
	}
	t := min(float32(age-in-hold)/float32(out), 1)
	return 0.5 + 0.5*float32(math.Cos(math.Pi*float64(t)))
}

// revealed is the part of a decoration's rows the sweep shows at now. A
// decoration without a direction is whole; forward reveals from each
// line's left and backward from its right, one line after another.
func revealed(sd *shownDeco, now time.Time, animate bool) []decoRow {
	if sd.direction == 0 || !animate || sd.started.IsZero() {
		return sd.rows
	}
	progress := min(max(float32(now.Sub(sd.started)), 0)/float32(pulseSweep), 1)
	if progress >= 1 {
		return sd.rows
	}
	rest := 1 - progress
	progress = 1 - rest*rest*rest
	total := float32(0)
	for _, row := range sd.rows {
		total += row.box.W
	}
	if total == 0 {
		return sd.rows
	}
	remaining := total * progress
	out := make([]decoRow, 0, len(sd.rows))
	for n := range sd.rows {
		i := n
		if sd.direction == vocab.Backward {
			i = len(sd.rows) - 1 - n
		}
		row := sd.rows[i]
		if remaining <= 0 {
			break
		}
		w := min(row.box.W, remaining)
		if sd.direction == vocab.Backward {
			row.box.X += row.box.W - w
		}
		row.box.W = w
		out = append(out, row)
		remaining -= w
	}
	return out
}

// decoRows are a range's pieces, a line each, hugging the line's glyphs;
// an empty range is a point, a piece with no width where its caret stands.
func decoRows(s *surfaceLayout, rg ui.TextInputRange, x, y float32) []decoRow {
	var out []decoRow
	rects := s.layout.SelectionRects(rg)
	if rg.Start == rg.End {
		c := s.caretRect(rg.Start)
		rects = []ui.Rect{{X: c.X, Y: c.Y, H: c.H}}
	}
	for _, r := range rects {
		row := decoRow{box: glyphBox(s, r), base: r.H}
		if l, ok := lineAt(s, r); ok {
			row.base, row.descent = l.Baseline, l.Descent
		}
		row.box.X += x
		row.box.Y += y
		row.base += y
		out = append(out, row)
	}
	return out
}

func (el *Element) paintBackground(p *ui.Painter, sd *shownDeco, ox, oy float32, now time.Time, animate bool) {
	for _, row := range revealed(sd, now, animate) {
		r := row.box
		if row.object || r.W == 0 || sd.look.bg == (ui.Color{}) {
			continue
		}
		p.Fill(ui.Rect{X: ox + r.X - decoPad, Y: oy + r.Y - decoPad/2, W: r.W + decoPad*2, H: r.H + decoPad}, sd.look.bg.Alpha(sd.alpha), 3)
	}
}

// paintOutlines draws every paint's outlines around the objects it holds,
// over what the objects draw, so an outline on an object's own frame takes
// the frame's place.
func (el *Element) paintOutlines(p *ui.Painter, ox, oy float32, now time.Time, animate bool) {
	for _, sd := range el.decos {
		for _, row := range revealed(sd, now, animate) {
			if !row.object {
				continue
			}
			r := row.box
			r.X += ox
			r.Y += oy
			radius := row.radius
			if radius == 0 {
				radius = outlineRadius(el.Editor.metrics, r)
			}
			components.PaintRing(p, r, radius, 0, sd.look.outline.Alpha(sd.alpha))
		}
	}
}

func (el *Element) paintLine(p *ui.Painter, sd *shownDeco, ox, oy float32, now time.Time, animate bool) {
	if sd.look.line == present.LineNone {
		return
	}
	ink := sd.look.ink
	if ink == (ui.Color{}) {
		ink = el.Theme.Prose.Text
	}
	ink = ink.Alpha(sd.alpha)
	for _, row := range revealed(sd, now, animate) {
		if row.object {
			continue
		}
		x, w := ox+row.box.X, row.box.W
		y := oy + row.base + max(underlineGap, row.descent*0.45)
		switch sd.look.line {
		case present.LineUnder:
			p.Line(x, y, x+w, y, underlineWidth, ink)
		case present.LineBox:
			// A box around the text, as an object it marks is outlined,
			// standing off it as a background does.
			r := outset(ui.Rect{X: x, Y: oy + row.box.Y, W: w, H: row.box.H}, decoPad, 1)
			components.PaintRing(p, r, outlineRadius(el.Editor.metrics, r), 0, ink)
		}
	}
}

const (
	// decoPad widens a background past the glyphs it lies under.
	decoPad = 2
	// underlineGap is the least room between the baseline and a line;
	// underlineWidth its stroke.
	underlineGap   = 2
	underlineWidth = 1.5
)

// selectionRows are the selection's rows of text, in the page.
func selectionRows(pl *pageLayout, paints []present.Paint) []ui.Rect {
	var out []ui.Rect
	for _, pt := range paints {
		if pt.Group != vocab.GroupSelection {
			continue
		}
		for _, c := range pt.Covers {
			u := pl.unitOf(c.Unit)
			if u == nil {
				continue
			}
			for _, pc := range pl.pieces(u, c) {
				if pc.surface < 0 {
					continue
				}
				for _, r := range pc.rows {
					out = append(out, r.box)
				}
			}
		}
	}
	return out
}

// shownPaints are the paints the page draws: the engine's, the selection
// a drag holds standing in for the published one.
func (el *Element) shownPaints(pl *pageLayout) []present.Paint {
	paints := el.Editor.deco.Paints
	g := el.gesture
	if g == nil || !g.valid {
		return paints
	}
	out := []present.Paint{}
	r := el.Editor.Engine.Snapshot()
	if from, to, ok := present.SelectionExtent(r, g.selection); ok {
		pres := make([]*present.Presentation, len(pl.units))
		for i, u := range pl.units {
			pres[i] = u.pres
		}
		out = append(out, present.Paint{Group: vocab.GroupSelection, Covers: present.Covers(r, pres, from, to)})
	}
	for _, pt := range paints {
		if pt.Group != vocab.GroupSelection {
			out = append(out, pt)
		}
	}
	return out
}

// paintCaret draws the caret in the accent. A lone caret at rest breathes:
// solid a moment after the last input, then fading through transparent
// and back; while the pointer is held, a composition is under way or a
// range is selected, it stays solid.
func (el *Element) paintCaret(p *ui.Painter, view, caret ui.Rect, collapsed bool, now time.Time, animate bool) {
	breathes := animate && collapsed && el.gesture == nil && el.Editor.deco.Composition == nil
	alpha := float32(1)
	if breathes {
		alpha = motion.CaretOpacity(el.Editor.active, now)
	}
	if alpha > 0 {
		p.Fill(caret, el.Theme.Accent.Foreground.Alpha(alpha), 1)
	}
	if !breathes || caret.Y+caret.H <= view.Y || caret.Y >= view.Y+view.H {
		return
	}
	if wake, ok := motion.CaretWake(el.Editor.active, now); ok {
		p.After(wake.Sub(now))
		return
	}
	p.AnimationFrame()
}

// paintGhosts draws a surface's ghosts in the room the line keeps for
// them.
func (el *Element) paintGhosts(p *ui.Painter, s *surfaceLayout, sx, sy float32) {
	for _, sg := range s.segs {
		if sg.kind != segGhost || sg.shaped == nil {
			continue
		}
		line, _, _ := s.lineOf(sg.u0)
		r := s.caretRect(sg.u0)
		gb := float32(0)
		if gl := sg.shaped.Lines(); len(gl) > 0 {
			gb = gl[0].Baseline
		}
		ink := sg.ink
		if ink == (ui.Color{}) {
			ink = el.Theme.Prose.Text
		}
		p.TextLayout(sg.shaped, sx+r.X, sy+line.Baseline-gb, ink)
	}
}

// paintBoxes draws closed inline nodes in their object boxes: a formula
// as its math, others as their source.
func (el *Element) paintBoxes(p *ui.Painter, s *surfaceLayout, sx, sy float32) {
	objs := s.layout.Objects()
	if len(objs) == 0 {
		return
	}
	i := 0
	for _, sg := range s.segs {
		if sg.kind != segBox {
			continue
		}
		if i >= len(objs) {
			break
		}
		o := objs[i].Rect
		i++
		if sg.inline != nil {
			sg.inline.paint(el, p, sg, ui.Rect{X: sx + o.X, Y: sy + o.Y, W: o.W, H: o.H})
			continue
		}
		bg := el.Theme.Accent.Muted
		if sg.open {
			bg = el.Theme.Prose.Selection
		}
		p.Fill(ui.Rect{X: sx + o.X, Y: sy + o.Y, W: o.W, H: o.H}, bg, 4)
		// TODO(perf): keep boxes' shaped text with the surface's layout.
		lay := ui.ShapeRichTextWith([]ui.Span{sg.face}, ui.Font{Family: sg.face.Font, Size: sg.face.Size}, ui.TextShape{})
		_, h := lay.Size()
		p.TextLayout(lay, sx+o.X+boxPad, sy+o.Y+(o.H-h)/2, sg.face.Color)
	}
}
