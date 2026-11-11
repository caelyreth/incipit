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
	hover   doc.ID
	pointer ui.Point
	focused bool
	// drop is the line showing where pictures dragged over the page go,
	// and dropAt the place they would.
	drop   dropLine
	dropAt *doc.Caret
}

// NewElement is an element over ed.
func NewElement(ed *Editor) *Element {
	el := &Element{Editor: ed, Label: "Page", focusPending: true, figures: map[doc.ID]*figureMotion{}}
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
	if pc, ok := pl.lookup(el.shownSelection().Head); ok && pc.unit.figure != nil && el.hinted(pc.unit) {
		_, x, _ := hintOf(pc.unit, el.Editor.theme, el.Editor.metrics)
		r.X = pc.unit.x + pc.surface.x + x
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
	el.focused = focused
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
	el.paintDecos(p, pl, ox, oy, top, bottom, now, animate)
	// The selection glides beneath the text; a drag places it directly,
	// so it follows the pointer and glides on from there.
	covers := ed.deco.Covers
	if g := el.gesture; g != nil && g.valid {
		covers = pl.selectionCovers(g.selection)
	}
	rows := el.selection.Step(el.selectionRows(pl, covers), now, animate && el.gesture == nil)
	if el.selection.Moving() {
		p.AnimationFrame()
	}
	wash := t.Prose.Selection.Alpha(el.selection.Opacity())
	for _, row := range rows {
		p.Fill(ui.Rect{X: ox + row.X, Y: oy + row.Y, W: row.W, H: row.H}, wash, 2)
	}
	whole := map[doc.ID]bool{}
	for _, c := range covers {
		whole[c.Unit] = c.Whole
	}
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
		switch {
		case u.figure != nil:
			el.paintFigure(p, u, ux, uy, whole[u.pres.Unit], now, animate)
		case u.object:
			// TODO(tables): tables draw their own grid; every other
			// object is a divider until then.
			mid := uy + u.h/2
			p.Line(ux, mid, ox+m.Width, mid, 1, t.Prose.Subtle)
			continue
		}
		for _, s := range u.surfaces {
			sx, sy := ux+s.x, uy+s.y
			if s.card {
				p.Fill(ui.Rect{X: sx - cardPad(m)/2, Y: sy - cardPad(m)/2, W: s.w + cardPad(m), H: s.h + cardPad(m)}, t.Surface.Inset, 6)
			}
			p.TextLayout(s.layout, sx, sy, t.Prose.Text)
			el.paintGhosts(p, s, sx, sy)
			el.paintBoxes(p, s, sx, sy)
		}
	}
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
}

// paintDecos draws what the page draws itself beneath the text, from each
// surface's styles and the decorations' highlights alike: a background a
// run per line, hugging the line's glyphs and rounded at its ends, and a
// line below them. Each fades in as it appears and out as it goes.
func (el *Element) paintDecos(p *ui.Painter, pl *pageLayout, ox, oy, top, bottom float32, now time.Time, animate bool) {
	if el.decos == nil {
		el.decos = map[decoKey]*shownDeco{}
	}
	for _, sd := range el.decos {
		sd.seen = false
	}
	counts := map[decoKey]int{}
	note := func(u *unitLayout, s *surfaceLayout, highlight bool, d deco, origin uint64, direction vocab.Direction, duration time.Duration, visible bool) {
		look := d
		look.rg = ui.TextInputRange{}
		k := decoKey{unit: u.pres.Unit, surface: s.index, highlight: highlight, origin: origin,
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
			sd.rows = decoRows(s, d.rg, u.x+s.x, u.y+s.y)
		}
	}
	visible := func(u *unitLayout) bool {
		uy := oy + u.y
		return !(uy > bottom || uy+u.h < top)
	}
	for _, h := range el.Editor.deco.Highlights {
		from, ok1 := pl.lookup(h.From)
		to, ok2 := pl.lookup(h.To)
		if !ok1 || !ok2 || from.surface == nil || from.surface != to.surface || to.stop.u < from.stop.u {
			continue
		}
		if d := decoOf(ui.TextInputRange{Start: from.stop.u, End: to.stop.u}, el.Editor.theme.Resolve([]vocab.Group{h.Group})); d.drawn() {
			note(from.unit, from.surface, true, d, h.Origin, h.Direction, h.Duration, visible(from.unit))
		}
	}
	for _, u := range pl.units {
		vis := visible(u)
		for _, s := range u.surfaces {
			for _, d := range s.decos {
				note(u, s, false, d, 0, 0, 0, vis)
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
	lines := s.layout.Lines()
	var out []decoRow
	rects := s.layout.SelectionRects(rg)
	if rg.Start == rg.End {
		c := s.caretRect(rg.Start)
		rects = []ui.Rect{{X: c.X, Y: c.Y, H: c.H}}
	}
	for _, r := range rects {
		row := decoRow{box: r, base: r.H}
		for _, l := range lines {
			if mid := r.Y + r.H/2; mid >= l.Rect.Y && mid < l.Rect.Y+l.Rect.H {
				row.box.Y, row.box.H = l.Baseline-l.Ascent, l.Ascent+l.Descent
				row.base, row.descent = l.Baseline, l.Descent
				break
			}
		}
		row.box.X += x
		row.box.Y += y
		row.base += y
		out = append(out, row)
	}
	return out
}

func (el *Element) paintBackground(p *ui.Painter, sd *shownDeco, ox, oy float32, now time.Time, animate bool) {
	if sd.look.bg == (ui.Color{}) {
		return
	}
	for _, row := range revealed(sd, now, animate) {
		r := row.box
		if r.W == 0 {
			continue
		}
		p.Fill(ui.Rect{X: ox + r.X - decoPad, Y: oy + r.Y - decoPad/2, W: r.W + decoPad*2, H: r.H + decoPad}, sd.look.bg.Alpha(sd.alpha), 3)
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
		x, w := ox+row.box.X, row.box.W
		y := oy + row.base + max(underlineGap, row.descent*0.45)
		switch sd.look.line {
		case present.LineUnder:
			p.Line(x, y, x+w, y, underlineWidth, ink)
		case present.LineWavy:
			wavyLine(p, x, w, y, ink)
		}
	}
}

// wavyLine draws a wavy line under [x, x+w): caret marks side by side, as
// many as fit a wave each, so under a single glyph, or at a point, it is
// one caret mark.
func wavyLine(p *ui.Painter, x, w, y float32, c ui.Color) {
	n := max(1, int(math.Round(float64(w/(2*caretMark)))))
	step := w / float32(n)
	a := min(step/2, caretMark)
	if w == 0 {
		a = caretMark
	}
	for i := range n {
		caretMarkAt(p, x+step*(float32(i)+0.5), y, a, c)
	}
}

// caretMarkAt draws a "^" whose apex is at (cx, y), reaching a below it
// on either side: two strokes turned from the apex.
func caretMarkAt(p *ui.Painter, cx, y, a float32, c ui.Color) {
	arm := a * math.Sqrt2
	w := float32(underlineWidth)
	p.Transform(ui.Rotation(-45), cx, y, func() {
		p.Fill(ui.Rect{X: cx - arm, Y: y - w/2, W: arm + w/2, H: w}, c, w/2)
	})
	p.Transform(ui.Rotation(45), cx, y, func() {
		p.Fill(ui.Rect{X: cx - w/2, Y: y - w/2, W: arm + w/2, H: w}, c, w/2)
	})
}

const (
	// decoPad widens a background past the glyphs it lies under.
	decoPad = 2
	// underlineGap is the least room between the baseline and a line;
	// underlineWidth its stroke; caretMark half a wave's width.
	underlineGap   = 2
	underlineWidth = 1.5
	caretMark      = 3.5
)

// selectionRows are the rows the selection covers, in page coordinates:
// a whole object's box, and each text row of a covered stretch.
func (el *Element) selectionRows(pl *pageLayout, covers []present.Cover) []ui.Rect {
	var out []ui.Rect
	byUnit := map[doc.ID]*unitLayout{}
	for _, u := range pl.units {
		byUnit[u.pres.Unit] = u
	}
	for _, c := range covers {
		u, ok := byUnit[c.Unit]
		if !ok {
			continue
		}
		if u.object && u.figure == nil {
			if c.Whole {
				out = append(out, ui.Rect{X: u.x, Y: u.y, W: el.Editor.metrics.Width - u.x, H: u.h})
			}
			continue
		}
		// A figure held whole wears its ring; its caption is washed as
		// text.
		ranges := pl.coverRanges(u, c)
		for _, s := range u.surfaces {
			rg, ok := ranges[s]
			if !ok {
				continue
			}
			for _, sr := range s.layout.SelectionRects(rg) {
				sr.X += u.x + s.x
				sr.Y += u.y + s.y
				out = append(out, sr)
			}
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

// paintBoxes draws closed inline nodes' text in their object boxes.
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
		bg := el.Theme.Accent.Muted
		if sg.card >= 0 {
			bg = el.Theme.Prose.Selection
		}
		p.Fill(ui.Rect{X: sx + o.X, Y: sy + o.Y, W: o.W, H: o.H}, bg, 4)
		// TODO(perf): keep boxes' shaped text with the surface's layout.
		lay := ui.ShapeRichTextWith([]ui.Span{sg.face}, ui.Font{Family: sg.face.Font, Size: sg.face.Size}, ui.TextShape{})
		_, h := lay.Size()
		p.TextLayout(lay, sx+o.X+boxPad, sy+o.Y+(o.H-h)/2, sg.face.Color)
	}
}
