package page

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/native/motion"
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

	client       *client
	scroll       float32
	scrollTarget float32
	viewport     ui.Rect
	column       float32 // the column's left, in the element
	pressed      bool
	focusPending bool
	lastClicks   int

	// The caret glides between the places it is drawn at.
	caret      [3]motion.Spring // x, y, height, in page coordinates
	caretShown bool
}

// caretFrequency keeps the caret up with typing.
const caretFrequency = 38

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
	ed.SetStyle(el.Theme.Surface, Theme(el.Theme), m)
	el.column = max((w-m.Width)/2, 0)
	return ed.Relayout()
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
	animate := !c.Preferences().ReduceMotion
	if ed.reveal {
		el.revealCaret(pl)
		ed.reveal = false
	}
	if el.stepScroll(animate) {
		c.AnimationFrame()
	}
	if el.stepCaret(pl, c.Now(), animate) {
		c.AnimationFrame()
	}
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
				el.clampScroll()
			}
			el.viewport = r
			el.paint(p, r, box.Focused())
		})
}

// toPage is an element point in page coordinates.
func (el *Element) toPage(x, y float32) (float32, float32) {
	return x - el.column, y - el.Top + el.scroll
}

// fromPage is a page rectangle in element coordinates.
func (el *Element) fromPage(r ui.Rect) ui.Rect {
	r.X += el.column
	r.Y += el.Top - el.scroll
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

func (el *Element) maxScroll() float32 {
	pl := el.Editor.layout
	if pl == nil {
		return 0
	}
	return max(pl.height+el.Top+el.Bottom-el.viewport.H, 0)
}

func (el *Element) clampScroll() {
	el.scrollTarget = min(max(el.scrollTarget, 0), el.maxScroll())
	el.scroll = min(max(el.scroll, 0), el.maxScroll()+el.viewport.H)
}

// revealCaret scrolls so the caret's line shows with room about it.
func (el *Element) revealCaret(pl *pageLayout) {
	if el.viewport.H <= 0 {
		return
	}
	r, ok := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if !ok {
		return
	}
	room := min(el.Prose.LineHeight()*3, el.viewport.H/4)
	top := r.Y + el.Top - room
	bottom := r.Y + r.H + el.Top + room - el.viewport.H
	switch {
	case top < el.scrollTarget:
		el.scrollTarget = top
	case bottom > el.scrollTarget:
		el.scrollTarget = bottom
	}
	el.clampScroll()
}

// stepScroll eases the scroll toward its target; it reports whether it
// still moves.
func (el *Element) stepScroll(animate bool) bool {
	d := el.scrollTarget - el.scroll
	if !animate || math.Abs(float64(d)) < 0.5 {
		el.scroll = el.scrollTarget
		return false
	}
	el.scroll += d * 0.3
	return true
}

// --- animation ------------------------------------------------------------

// stepCaret moves the drawn caret toward the selection head's place; it
// reports whether it still moves.
func (el *Element) stepCaret(pl *pageLayout, now time.Time, animate bool) bool {
	r, ok := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if !ok {
		el.caretShown = false
		return false
	}
	target := [3]float32{r.X, r.Y, r.H}
	if !el.caretShown || !animate {
		for i := range el.caret {
			el.caret[i] = motion.NewSpring(target[i], now)
		}
		el.caretShown = true
		return false
	}
	moving := false
	for i := range el.caret {
		el.caret[i].Step(target[i], now, caretFrequency)
		moving = moving || el.caret[i].Moving()
	}
	return moving
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
		el.scrollTarget += ev.DY
		el.clampScroll()
		el.scroll = el.scrollTarget
		services.Invalidate()
		return true
	case ui.InputKeyDown:
		if ed.deco.Composition != nil {
			// The input method has the keys while it composes.
			return false
		}
		if in, ok := keyIntent(ev); ok && el.run(in, services, at) {
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
	t := el.Theme
	sel := ed.Selection()
	covers := map[doc.ID]present.Cover{}
	for _, c := range ed.deco.Covers {
		covers[c.Unit] = c
	}
	ox, oy := r.X+el.column, r.Y+el.Top-el.scroll
	top, bottom := r.Y, r.Y+r.H
	m := ed.metrics
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
	if focused && sel.Kind == doc.SelectText && el.caretShown {
		cr := ui.Rect{X: el.caret[0].Value() + ox, Y: el.caret[1].Value() + oy, W: 2, H: el.caret[2].Value()}
		p.Fill(cr, t.Accent.Foreground, 1)
	}
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
