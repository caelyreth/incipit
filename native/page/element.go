package page

import (
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/vocab"
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

	client *client
	// scroll is the page's offset, which glide moves and a scroll by hand
	// sets; overscroll is its elastic pull past an edge.
	scroll       float32
	glide        motion.Glide
	overscroll   scrollfx.Elastic
	viewport     ui.Rect
	column       float32 // the column's left, in the element
	pressed      bool
	focusPending bool
	lastClicks   int
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
}

// NewElement is an element over ed.
func NewElement(ed *Editor) *Element {
	el := &Element{Editor: ed, Label: "Page", focusPending: true}
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
	return ed.Relayout()
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
	box := ui.Box(c).Key("page-editor").Fill().FocusRing(false).
		Role(ui.RoleTextField).Label(el.Label).Focusable()
	box.HandleTextInput(el.client)
	if el.focusPending {
		box.AutoFocus()
		box.Focus()
		el.focusPending = false
	}
	services := c.Services()
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
	r, ok := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
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
	r, ok := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if !ok {
		el.caret.Hide()
		return false
	}
	if !el.caret.Shown() {
		// A caret appearing is activity: solid a moment, then breathing,
		// as on a page just opened before any input.
		el.Editor.active = now
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
		el.pressed = true
		el.lastClicks = max(ev.Clicks, 1)
		ed.place(x, y, el.lastClicks, ev.Mods&ui.Shift != 0, at)
		el.Focus()
		services.Invalidate()
		return true
	case ui.InputPointerMove:
		if !el.pressed {
			return false
		}
		x, y := el.toPage(ev.X, ev.Y)
		// A drag extends by the unit the press selected by.
		ed.place(x, y, el.lastClicks, true, at)
		services.Invalidate()
		return true
	case ui.InputPointerUp:
		el.pressed = false
		return el.pressed
	case ui.InputScroll:
		el.scrollBy(ev, at)
		services.Invalidate()
		return true
	case ui.InputKeyDown:
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
		if text := el.Editor.copy(); text != "" {
			services.WriteClipboard(text)
		}
		return true
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
	if el.stepViewport(now, animate) {
		p.AnimationFrame()
	}
	if el.stepCaret(pl, now, animate) {
		p.AnimationFrame()
	}
	t := el.Theme
	sel := ed.Selection()
	covers := map[doc.ID]present.Cover{}
	for _, c := range ed.deco.Covers {
		covers[c.Unit] = c
	}
	ox, oy := r.X+el.column, r.Y+el.Top-el.offset()
	top, bottom := r.Y, r.Y+r.H
	m := ed.metrics
	el.paintHighlights(p, pl, ox, oy)
	for _, u := range pl.units {
		uy := oy + u.y
		if uy > bottom || uy+u.h < top {
			continue
		}
		ux := ox + u.x
		cover, covered := covers[u.pres.Unit]
		// Quote bars, one per quote level.
		for q := 0; q < u.pres.Context.Quotes; q++ {
			bx := ox + float32(u.pres.Context.Indent+q)*m.Indent + m.Indent*0.3
			p.Fill(ui.Rect{X: bx, Y: uy - m.Gap/2, W: 2, H: u.h + m.Gap}, t.Prose.Subtle, 1)
		}
		if u.object {
			// TODO(figures), TODO(tables): figures and tables draw their
			// own boxes; every object is a divider until then.
			if cover.Whole {
				p.Fill(ui.Rect{X: ux, Y: uy, W: m.Width - u.x, H: u.h}, t.Prose.Selection, 4)
			}
			mid := uy + u.h/2
			p.Line(ux, mid, ox+m.Width, mid, 1, t.Prose.Subtle)
			continue
		}
		var ranges map[*surfaceLayout]ui.TextInputRange
		if covered {
			ranges = pl.coverRanges(u, cover)
		}
		for _, s := range u.surfaces {
			sx, sy := ux+s.x, uy+s.y
			if s.card {
				p.Fill(ui.Rect{X: sx - cardPad(m)/2, Y: sy - cardPad(m)/2, W: s.w + cardPad(m), H: s.h + cardPad(m)}, t.Surface.Inset, 6)
			}
			if rg, ok := ranges[s]; ok {
				for _, sr := range s.layout.SelectionRects(rg) {
					sr.X += sx
					sr.Y += sy
					p.Fill(sr, t.Prose.Selection, 2)
				}
			}
			p.TextLayout(s.layout, sx, sy, t.Prose.Text)
			el.paintGhosts(p, s, sx, sy)
			el.paintBoxes(p, s, sx, sy)
		}
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

// paintCaret draws the caret in the accent. A lone caret at rest breathes:
// solid a moment after the last input, then fading through transparent
// and back; while the pointer is held, a composition is under way or a
// range is selected, it stays solid.
func (el *Element) paintCaret(p *ui.Painter, view, caret ui.Rect, collapsed bool, now time.Time, animate bool) {
	breathes := animate && collapsed && !el.pressed && el.Editor.deco.Composition == nil
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

// paintHighlights fills, behind the text, each highlight the engine
// publishes from caret to caret: an armed pair's two sides, its signs too,
// which span stops but no bytes. A highlight lies on one line.
func (el *Element) paintHighlights(p *ui.Painter, pl *pageLayout, ox, oy float32) {
	for _, h := range el.Editor.deco.Highlights {
		if h.Group != vocab.GroupArmed {
			continue
		}
		from, ok1 := pl.caretBox(h.From, el.Editor.metrics)
		to, ok2 := pl.caretBox(h.To, el.Editor.metrics)
		if !ok1 || !ok2 || to.X <= from.X {
			continue
		}
		p.Fill(ui.Rect{X: ox + from.X, Y: oy + from.Y, W: to.X - from.X, H: max(from.H, to.H)}, el.Theme.Prose.Selection, 2)
	}
}
