package table

import (
	"time"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/motion"
	"github.com/egoist/mygo/ui"
)

// paint draws the grid beneath its cells' text, in each of its parts:
// the header's inset, the lines between the cells, and its edge; and the
// lines that left, as they were, in the rooms they still have.
func (Kind) Paint(h Host, p *ui.Painter, u *Unit, ux, uy float32) float32 {
	t := u.Table
	if t == nil {
		return 0
	}
	for _, Part := range t.parts(u.Box) {
		Part.X += ux
		Part.Y += uy
		t.paintGrid(h, p, Part, u.Frame, ux, uy, true, true)
	}
	for axis, rooms := range t.gone {
		for _, g := range rooms {
			room := g.room
			if axis == 1 {
				room = t.across(room)
			}
			paintGone(h, p, axis == 0, g, room, ux, uy)
		}
	}
	return 0
}

// paintGone draws a line that left as it was kept, its cells' text within
// the room it still has, fading out.
func paintGone(h Host, p *ui.Painter, rows bool, g goneRoom, room ui.Rect, ux, uy float32) {
	ku := g.kept.u
	kt := ku.Table
	if kt == nil || g.alpha < 0.002 || room.W < 0.5 || room.H < 0.5 {
		return
	}
	was := kt.line(ku, rows, g.kept.index)
	dx, dy := room.X-was.X, float32(0)
	if rows {
		dx, dy = 0, room.Y-was.Y
	}
	room.X += ux
	room.Y += uy
	p.Clip(room, 0, func() {
		for i, s := range ku.Surfaces {
			cc := kt.cells[i]
			if cc.Row < 0 || !(rows && cc.Row == g.kept.index || !rows && cc.Column == g.kept.index) {
				continue
			}
			sx, sy := ux+dx+s.X, uy+dy+s.Y
			s.Paint(p, sx, sy)
			// It fades into what the grid has under it: the header's
			// inset, or the page.
			ground := h.Theme().Surface.Page
			if kt.header && cc.Row == 0 {
				ground = h.Theme().Surface.Inset
			}
			p.Fill(ui.Rect{X: sx, Y: sy, W: s.W, H: s.H}, ground.Alpha(1-g.alpha), 0)
		}
	})
}

// furthest is the line gliding furthest from its place, over those
// gliding with it; false while none glides.
func (t *Layout) furthest() (axis, index int, ok bool) {
	lm := t.in.lines
	if lm == nil {
		return 0, 0, false
	}
	far := float32(0.5)
	for a := range lm.offset {
		for i, o := range lm.offset[a] {
			if o = max(o, -o); o > far {
				far, axis, index, ok = o, a, i, true
			}
		}
	}
	return axis, index, ok
}

// paintGrid draws the grid within frame, its corners rounded by radius and
// the unit drawn at ux, uy: the header's inset, the lines between its rows
// and between its columns as asked, and the frame's edge.
func (t *Layout) paintGrid(h Host, p *ui.Painter, frame ui.Rect, radius, ux, uy float32, rows, columns bool) {
	line := h.Theme().Border.Default
	p.Clip(frame, radius, func() {
		if t.header && len(t.rows) > 0 {
			p.Fill(ui.Rect{X: frame.X, Y: uy + t.rows[0], W: frame.W, H: t.heights[0]}, h.Theme().Surface.Inset, 0)
		}
		for r := range t.rows {
			if at := uy + t.rows[r]; rows && at > frame.Y+0.5 && at < frame.Y+frame.H-0.5 {
				p.Fill(ui.Rect{X: frame.X, Y: at, W: frame.W, H: 1}, line, 0)
			}
		}
		for c := range t.columns {
			if at := ux + t.left - t.scroll + t.columns[c]; columns && at > frame.X+0.5 && at < frame.X+frame.W-0.5 {
				p.Fill(ui.Rect{X: at, Y: frame.Y, W: 1, H: frame.H}, line, 0)
			}
		}
	})
	p.Stroke(frame, line, radius, 1)
}

// over fades the sides of a table wider than its room where more of it
// lies past them, as far as it does, up to a line's width.
func (Kind) Over(h Host, p *ui.Painter, u *Unit, ux, uy float32) {
	t := u.Table
	if t == nil || t.overflow <= 0 {
		return
	}
	reach := h.Metrics().Line()
	page := h.Theme().Surface.Page
	box := ui.Rect{X: ux + u.Box.X, Y: uy + u.Box.Y, W: u.Box.W, H: u.Box.H}
	if past := min(t.scroll, reach); past > 0.5 {
		p.FillGradient(ui.Rect{X: box.X, Y: box.Y, W: past, H: box.H}, ui.LinearGradient{From: page, To: page.Alpha(0), Angle: 90}, 0)
	}
	if past := min(t.overflow-t.scroll, reach); past > 0.5 {
		p.FillGradient(ui.Rect{X: box.X + box.W - past, Y: box.Y, W: past, H: box.H}, ui.LinearGradient{From: page.Alpha(0), To: page, Angle: 90}, 0)
	}
}

// cover is a cell's box.
func (Kind) Cover(u *Unit, s *Surface) (ui.Rect, bool) {
	t := u.Table
	if t == nil || s.Index >= len(t.cells) || t.cells[s.Index].Row < 0 {
		return ui.Rect{}, false
	}
	return t.cell(t.cells[s.Index]), true
}

// tableScroll is how far a table wider than its room is scrolled across:
// Settle draws a table scrolled as far as it is, within what it scrolls;
// a paint steps its scroll at its own time.
func (Kind) Settle(h Host, u *Unit, frame bool) bool {
	t := u.Table
	if t == nil {
		return false
	}
	ts := h.Scroll(u.Pres.Unit, false)
	if ts == nil && t.overflow <= 0 {
		t.scrollTo(u, 0)
		return false
	}
	ts = h.Scroll(u.Pres.Unit, true)
	moving := false
	if frame {
		_, pulling := ts.Over.Step(h.Now(), h.Animate())
		var gliding bool
		ts.X, gliding = ts.Glide.Step(ts.X, h.Now(), h.Animate())
		moving = pulling || gliding
	}
	ts.X = max(0, min(ts.X, t.overflow))
	t.scrollTo(u, ts.X-ts.Over.Offset())
	return moving
}

// Wheel scrolls a table wider than its room across by dx, by hand, with
// the pointer over it.
func (Kind) Wheel(h Host, u *Unit, x, y float32, ev ui.InputEvent, dx float32) bool {
	t := u.Table
	if t == nil || t.overflow <= 0 || y < u.Y || y >= u.Bottom() || x < u.X+u.Box.X || x >= u.X+u.Box.X+u.Box.W {
		return false
	}
	ts := h.Scroll(u.Pres.Unit, true)
	ts.Glide.Cancel()
	ts.X = components.OverscrollPanel.Scroll(&ts.Over, ts.X, dx, 0, t.overflow, u.Box.W, h.UIFontSize(), ev)
	t.scrollTo(u, ts.X-ts.Over.Offset())
	return true
}

// Reveal glides a table the caret is in across, as far as brings
// the caret's cell into view clear of the fades at its edges (Over), or,
// in a cell wider than that, the caret.
func (Kind) Reveal(h Host, u *Unit, s Surface, caretX float32, now time.Time) {
	if u.Table == nil || u.Table.overflow <= 0 {
		return
	}
	t := u.Table
	if s.Index >= len(t.cells) || t.cells[s.Index].Row < 0 {
		return
	}
	ts := h.Scroll(u.Pres.Unit, true)
	// Across the grid, from what the table shows.
	m := h.Metrics()
	reach := m.Line()
	c := t.cells[s.Index].Column
	from, w := t.columns[c]-reach, t.widths[c]+2*reach
	if w > u.Box.W {
		from, w = t.base[s.Index]-t.left+caretX-reach, 2*reach
	}
	target, changed := motion.Reveal(ts.X, u.Box.W, from, w, false, false)
	if !changed {
		return
	}
	target = max(0, min(target, t.overflow))
	if target != ts.X || ts.Glide.Moving() {
		ts.Glide.To(ts.X, target, now)
	}
}
