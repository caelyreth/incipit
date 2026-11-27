package page

import (
	"math"
	"slices"
	"unicode/utf16"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

func (pl *pageLayout) hit(x, y float32) (vocab.Place, bool) {
	if len(pl.units) == 0 {
		return vocab.Place{}, false
	}
	// The unit under y, or the nearest across the gap y falls in.
	var u *unitLayout
	for i, cand := range pl.units {
		if y < cand.bottom() {
			u = cand
			if i > 0 && y < cand.y {
				prev := pl.units[i-1]
				if y-prev.bottom() < cand.y-y {
					u = prev
				}
			}
			break
		}
	}
	if u == nil {
		u = pl.units[len(pl.units)-1]
		y = u.bottom() - 1
	}
	if y < u.y {
		y = u.y
	}
	var s *surfaceLayout
	if u.kind != nil {
		if v := u.view; v.W > 0 {
			// Past what the object shows, a point is read at its edge.
			x = min(max(x, u.x+v.X), u.x+v.X+v.W-1)
		}
		s = u.surfaceAt(x, y)
	}
	if s == nil && u.kind != nil && (!anyShown(u) || y < u.y+u.box.Y+u.box.H) {
		// On what the object draws: the object, and its place on the half
		// the point is in, which a drag extends to.
		if u.pres.Gap == nil {
			return vocab.Place{}, false
		}
		return vocab.Place{Stop: u.beside(y >= u.y+u.box.Y+u.box.H/2), Node: u.pres.Unit}, true
	}
	for _, cand := range u.surfaces {
		if s != nil {
			break
		}
		ox, oy := u.origin(cand)
		if cand.card && cand.shown && x >= ox-cardPad(Metrics{}) && y >= oy && y < oy+cand.h {
			s = cand
		}
	}
	if s == nil {
		s = u.nearestSurface(x, y)
	}
	if s == nil {
		return vocab.Place{}, false
	}
	ox, oy := u.origin(s)
	ly := min(max(y-oy, 0), s.h-1)
	if c, id, ok := s.boxAt(x-ox, ly); ok {
		return vocab.Place{Stop: c, Node: id}, true
	}
	at := s.layout.PositionAt(ui.Point{X: x - ox, Y: ly})
	st := s.nearest(at.Index)
	if st == nil {
		return vocab.Place{}, false
	}
	return vocab.Place{Stop: st.caret(), Over: over(s.layout, at, x-ox)}, true
}

// over is the side of boundary at the grapheme under x lies on, in
// logical order: the one between at and its neighbour on x's side on the
// line, or, past the line's edge, the one on the other side; zero on an
// empty line. Read from the layout's own carets, it holds in either
// direction of text.
func over(l *ui.TextLayout, at ui.TextCaretPosition, x float32) vocab.Direction {
	edge := l.CaretAt(at)
	side := 1
	if x < edge.X {
		side = -1
	}
	for _, d := range []int{side, -side} {
		next := l.MoveCaret(at, d)
		if next.Index == at.Index || l.CaretAt(next).Y != edge.Y {
			continue
		}
		if next.Index > at.Index {
			return vocab.Forward
		}
		return vocab.Backward
	}
	return 0
}

// boxAt is what a click on an inline box hits: an object, selected
// whole, with the place on the half of it the point is in, which a drag
// extends from, as a block object's gaps are; another inline node's
// source, entered where it lands.
func (sl *surfaceLayout) boxAt(x, y float32) (doc.Caret, doc.ID, bool) {
	for i, sg := range sl.segs {
		if sg.kind != segBox || sg.node == nil {
			continue
		}
		r, ok := sl.objectRect(i)
		if !ok || x < r.X || x >= r.X+r.W || y < r.Y || y >= r.Y+r.H {
			continue
		}
		if sg.inline == nil {
			return doc.Caret{}, 0, false
		}
		at := sl.stopBefore(sg.u0)
		if x >= r.X+r.W/2 {
			at = sl.stopBefore(sg.u1)
		}
		return at, sg.node.ID(), true
	}
	return doc.Caret{}, 0, false
}

// stopBefore is the surface's stop at UTF-16 index u.
func (sl *surfaceLayout) stopBefore(u int) doc.Caret {
	if st := sl.nearest(u); st != nil {
		return st.caret()
	}
	return doc.Caret{}
}

// cardOpen reports whether a box's card, surface i, is open.
func cardOpen(p *present.Presentation, i int) bool {
	return i >= 0 && i < len(p.Surfaces) && p.Surfaces[i].Open
}

// anyShown reports whether any of a unit's surfaces is drawn.
func anyShown(u *unitLayout) bool {
	for _, s := range u.surfaces {
		if s.shown {
			return true
		}
	}
	return false
}

// objectRect is the laid-out rectangle of the inline object segs[i]
// stands for: the objects are laid out in the order their segments run.
func (sl *surfaceLayout) objectRect(i int) (ui.Rect, bool) {
	objs := sl.layout.Objects()
	n := 0
	for j, sg := range sl.segs {
		if sg.kind != segBox {
			continue
		}
		if j == i {
			if n < len(objs) {
				return objs[n].Rect, true
			}
			return ui.Rect{}, false
		}
		n++
	}
	return ui.Rect{}, false
}

// coverRanges are the UTF-16 runs a published cover spans in each of a
// unit's surfaces, one per surface.
func (pl *pageLayout) coverRanges(u *unitLayout, c present.Cover) map[*surfaceLayout]ui.TextInputRange {
	// A run of one surface broken by another's stops, as a line's by its
	// formula's card, is one run.
	runs := map[int][2]int{}
	for _, sp := range c.Spans {
		if sp.Surface < 0 || sp.Surface >= len(u.surfaces) || sp.To >= len(u.pres.Stops) {
			continue
		}
		r, ok := runs[sp.Surface]
		if !ok {
			r = [2]int{sp.From, sp.To}
		}
		runs[sp.Surface] = [2]int{min(r[0], sp.From), max(r[1], sp.To)}
	}
	out := map[*surfaceLayout]ui.TextInputRange{}
	for i, r := range runs {
		from, ok1 := pl.lookup(u.pres.Stops[r[0]].Caret)
		to, ok2 := pl.lookup(u.pres.Stops[r[1]].Caret)
		if ok1 && ok2 && from.stop.u <= to.stop.u {
			out[u.surfaces[i]] = ui.TextInputRange{Start: from.stop.u, End: to.stop.u}
		}
	}
	return out
}

// byteIndex is the UTF-8 offset in s of UTF-16 index u.
func byteIndex(s string, u int) int {
	n := 0
	for i, r := range s {
		if n >= u {
			return i
		}
		n += utf16.RuneLen(r)
	}
	return len(s)
}

// vertical is where a move up or down from a caret lands at goal x: the
// line above or below in its surface; past its surface's edge, the next
// row of the page. An object's rows are its gap before, its caption's
// lines and its gap after, so motion passes through each; a text unit is
// entered on its nearest line. False past the page's first or last row.
func (pl *pageLayout) vertical(c doc.Caret, goalX float32, up bool) (doc.Caret, bool) {
	pc, ok := pl.lookup(c)
	if !ok {
		return c, false
	}
	u := pc.unit
	if s := pc.surface; s != nil {
		_, oy := u.origin(s)
		lines := s.layout.Lines()
		_, i, n := s.lineOf(pc.stop.u)
		if up && i > 0 {
			return pl.hitSurface(u, s, goalX, oy+lines[i-1].Rect.Y+lines[i-1].Rect.H/2), true
		}
		if !up && i+1 < n {
			return pl.hitSurface(u, s, goalX, oy+lines[i+1].Rect.Y+lines[i+1].Rect.H/2), true
		}
	}
	if u.kind != nil {
		rows := pl.objectRows(u)
		k := slices.IndexFunc(rows, func(r row) bool { return r.has(pc) })
		if k >= 0 {
			if up {
				k--
			} else {
				k++
			}
			if k >= 0 && k < len(rows) {
				return rows[k].at(pl, u, goalX, up), true
			}
		}
	}
	i := slices.Index(pl.units, u)
	step := 1
	if up {
		step = -1
	}
	next := i + step
	for next >= 0 && next < len(pl.units) && pl.units[next].kind != nil && len(pl.objectRows(pl.units[next])) == 0 {
		// An object with no place of its own to stand, as a divider
		// between texts, is passed over.
		next += step
	}
	if next < 0 || next >= len(pl.units) {
		return c, false
	}
	n := pl.units[next]
	if n.kind != nil {
		rows := pl.objectRows(n)
		k := 0
		if up {
			k = len(rows) - 1
		}
		t := rows[k].at(pl, n, goalX, up)
		if t == c && len(rows) > 1 {
			// Objects side by side share the gap between them.
			t = rows[k-step].at(pl, n, goalX, up)
		}
		return t, true
	}
	y := n.y + 2
	if up {
		y = n.y + n.h - 2
	}
	h, ok := pl.hit(goalX, y)
	return h.Stop, ok
}

// row is one row an object's vertical motion passes: a gap, or a band of
// its surfaces side by side, as a table's row of cells, or one alone.
type row struct {
	gap  *stopAt
	band []*surfaceLayout
}

func (r row) has(pc place) bool {
	if r.gap != nil {
		return pc.stop == r.gap
	}
	return slices.Contains(r.band, pc.surface)
}

// at is where motion arriving in the row lands at goal x: in the band's
// surface nearest across, on its first line going down, its last going up.
func (r row) at(pl *pageLayout, u *unitLayout, goalX float32, up bool) doc.Caret {
	if r.gap != nil {
		return r.gap.caret()
	}
	s, bd := r.band[0], float32(math.Inf(1))
	for _, cand := range r.band {
		a := u.area(cand)
		if d := max(a.X-goalX, 0, goalX-(a.X+a.W)); d < bd {
			s, bd = cand, d
		}
	}
	_, oy := u.origin(s)
	lines := s.layout.Lines()
	l := lines[0].Rect
	if up {
		l = lines[len(lines)-1].Rect
	}
	return pl.hitSurface(u, s, goalX, oy+l.Y+l.H/2)
}

// objectRows are the rows an object's vertical motion passes, in order:
// its gaps around its surfaces, those it shows side by side, their extents
// overlapping, in one band. A surface's own lines are passed in it, before
// motion leaves its band.
func (pl *pageLayout) objectRows(u *unitLayout) []row {
	var rows []row
	if u.gaps[0] != nil {
		rows = append(rows, row{gap: u.gaps[0]})
	}
	for _, s := range u.surfaces {
		// An own slot not shown is passed through all the same, as
		// horizontal motion passes it: arriving opens it.
		if n := len(rows); n > 0 && rows[n-1].gap == nil && besides(rows[n-1].band, s) {
			rows[n-1].band = append(rows[n-1].band, s)
			continue
		}
		rows = append(rows, row{band: []*surfaceLayout{s}})
	}
	if u.gaps[1] != nil {
		rows = append(rows, row{gap: u.gaps[1]})
	}
	return rows
}

// besides reports whether s stands beside every surface of a band: each
// shown in its object's own room, not a card, and over the same height.
func besides(band []*surfaceLayout, s *surfaceLayout) bool {
	for _, b := range band {
		if !b.shown || b.card || !s.shown || s.card || s.y >= b.y+b.h || b.y >= s.y+s.h {
			return false
		}
	}
	return true
}

// area is the room a surface of u takes, in the page: the box its object
// sets it in, or its own extent.
func (u *unitLayout) area(s *surfaceLayout) ui.Rect {
	if u.kind != nil {
		if r, ok := u.kind.cover(u, s); ok {
			r.X += u.x
			r.Y += u.y
			return r
		}
	}
	x, y := u.origin(s)
	return ui.Rect{X: x, Y: y, W: s.w, H: s.h}
}

// surfaceAt is the surface of u shown at a page point, but a card, which
// stands over its line.
func (u *unitLayout) surfaceAt(x, y float32) *surfaceLayout {
	for _, s := range u.surfaces {
		if r := u.area(s); s.shown && !s.card && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return s
		}
	}
	return nil
}

// nearestSurface is the surface of u shown nearest a page point, but a
// card; its first when it shows none.
func (u *unitLayout) nearestSurface(x, y float32) *surfaceLayout {
	var best *surfaceLayout
	bd := float32(math.Inf(1))
	for _, s := range u.surfaces {
		if !s.shown || s.card {
			continue
		}
		r := u.area(s)
		dx := max(r.X-x, 0, x-(r.X+r.W))
		dy := max(r.Y-y, 0, y-(r.Y+r.H))
		if d := dx*dx + dy*dy; d < bd {
			best, bd = s, d
		}
	}
	if best == nil && len(u.surfaces) > 0 {
		best = u.surfaces[0]
	}
	return best
}

// hitSurface is the stop of a surface nearest a page point.
func (pl *pageLayout) hitSurface(u *unitLayout, s *surfaceLayout, x, y float32) doc.Caret {
	ox, oy := u.origin(s)
	ly := min(max(y-oy, 0), s.h-1)
	if c, _, ok := s.boxAt(x-ox, ly); ok {
		return c
	}
	if st := s.nearest(s.layout.IndexAt(ui.Point{X: x - ox, Y: ly})); st != nil {
		return st.caret()
	}
	return doc.Caret{}
}
