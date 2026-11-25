package page

import (
	"math"
	"strings"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/egoist/mygo/ui"
)

func (sl *surfaceLayout) u16Of(off, stop int) int {
	// A stop a ghost at the offset holds, its edges included, is in it.
	for _, sg := range sl.segs {
		if sg.kind == segGhost && sg.from == off && stop >= sg.stopFrom && stop <= sg.stopFrom+sg.stops {
			if stop == sg.stopFrom+sg.stops {
				return sg.u1
			}
			return sg.u0 + utf16Len(sg.text[:graphemeOffset(sg.text, stop-sg.stopFrom)])
		}
	}
	for _, sg := range sl.segs {
		switch sg.kind {
		case segGhost, segComposition:
			continue
		default:
			if off < sg.from || off > sg.to {
				continue
			}
			if sg.kind != segText {
				if off == sg.from {
					return sg.u0
				}
				return sg.u1
			}
			return sg.u0 + utf16Len(sg.text[:off-sg.from])
		}
	}
	if len(sl.segs) == 0 {
		return 0
	}
	return sl.segs[len(sl.segs)-1].u1
}

// graphemeOffset is the byte offset after the first k graphemes of s.
func graphemeOffset(s string, k int) int {
	off := 0
	for g := graphemes.FromString(s); k > 0 && g.Next(); k-- {
		off = g.End()
	}
	return off
}

// nearest is the surface's stop nearest a UTF-16 index.
func (sl *surfaceLayout) nearest(u int) *stopAt {
	var best *stopAt
	bd := math.MaxInt
	for i := range sl.stops {
		d := sl.stops[i].u - u
		if d < 0 {
			d = -d
		}
		if d < bd {
			best, bd = &sl.stops[i], d
		}
	}
	return best
}

// caretRect is a stop's caret in the surface.
func (sl *surfaceLayout) caretRect(u int) ui.Rect {
	return sl.layout.CaretAt(ui.TextCaretPosition{Index: u})
}

// lineBounds is where a line's text starts and ends: a line a newline
// ends stops before it; a wrapped line's end is the next line's start,
// which the caret takes at this line's end.
func (sl *surfaceLayout) lineBounds(line ui.TextLine) (start, end int) {
	start, end = line.Start, line.End
	if end > start && end <= len(sl.text) && strings.HasSuffix(sl.text[:byteIndex(sl.text, end)], "\n") {
		end--
	}
	return start, end
}

// lineOf is the line of the layout holding u.
func (sl *surfaceLayout) lineOf(u int) (ui.TextLine, int, int) {
	lines := sl.layout.Lines()
	for i, l := range lines {
		if u < l.End || i == len(lines)-1 {
			return l, i, len(lines)
		}
	}
	return ui.TextLine{}, 0, 0
}

// --- page geometry --------------------------------------------------------

// origin is a surface's top left in page coordinates.
func (u *unitLayout) origin(s *surfaceLayout) (float32, float32) {
	return u.x + s.x, u.y + s.y
}

// beside is an object's place before it or after it: its gap stop, or,
// where text stands beside it, the gap's place, which the engine settles
// on that text.
func (u *unitLayout) beside(after bool) doc.Caret {
	side := 0
	if after {
		side = 1
	}
	if g := u.gaps[side]; g != nil {
		return g.caret()
	}
	pos := *u.pres.Gap
	pos.Offset += side
	return doc.CaretAt(pos)
}

// spotKind is what a place on the page is, which says how anything
// marking it is drawn.
type spotKind uint8

const (
	// spotText is a caret in text shown: a bar as tall as its line's
	// glyphs.
	spotText spotKind = iota
	// spotGapBefore and spotGapAfter are carets between blocks, at an
	// object's gap: a bar across its top, or under all of it.
	spotGapBefore
	spotGapAfter
	// spotObject is a place inside what an object draws rather than in
	// text shown, as in a closed card: it has no caret of its own there,
	// and is the object, outlined.
	spotObject
	// spotOpening is a line an object opens under what it draws as the
	// caret enters it, as a figure's empty caption: the band it opens in.
	spotOpening
	// spotRange is a range of the page: the rows what it covers stands
	// in (span).
	spotRange
)

// span is where the rows covers paint stand, in page coordinates: across
// all of them, and the height of the first.
func (pl *pageLayout) span(covers []present.Cover) (ui.Rect, float32, bool) {
	var r ui.Rect
	head, ok := float32(0), false
	for _, c := range covers {
		u := pl.unitOf(c.Unit)
		if u == nil {
			continue
		}
		for _, pc := range pl.pieces(u, c) {
			for _, row := range pc.rows {
				b := row.box
				if !ok {
					r, head, ok = b, b.H, true
					continue
				}
				if b.Y < r.Y {
					head = b.H
				}
				x0, y0 := min(r.X, b.X), min(r.Y, b.Y)
				r = ui.Rect{X: x0, Y: y0, W: max(r.X+r.W, b.X+b.W) - x0, H: max(r.Y+r.H, b.Y+b.H) - y0}
			}
		}
	}
	return r, head, ok
}

type caretMetric struct {
	rect        ui.Rect
	kind        spotKind
	baseline    float32
	fontSize    float32
	hasBaseline bool
}

func (cm caretMetric) scale(body float32) float32 {
	if body > 0 && cm.fontSize > 0 {
		return cm.fontSize / body
	}
	return 1
}

// spot is where a place stands, in page coordinates, and what it is.
func (pl *pageLayout) spot(c doc.Caret) (ui.Rect, spotKind, bool) {
	cm, ok := pl.spotMetrics(c)
	return cm.rect, cm.kind, ok
}

// caretMetrics is where c stands in page coordinates along with its line's
// baseline and resolved base font size when it sits in shown text.
func (pl *pageLayout) caretMetrics(c doc.Caret) (caretMetric, bool) {
	return pl.spotMetrics(c)
}

func (pl *pageLayout) spotMetrics(c doc.Caret) (caretMetric, bool) {
	pc, ok := pl.lookup(c)
	if !ok {
		return caretMetric{}, false
	}
	if pc.surface == nil {
		u := pc.unit
		if pc.stop == u.gaps[0] {
			return caretMetric{rect: ui.Rect{X: u.x + u.box.X, Y: u.y - gapBarRoom - 1, W: u.box.W, H: 2}, kind: spotGapBefore}, true
		}
		return caretMetric{rect: ui.Rect{X: u.x + u.box.X, Y: u.bottom() + gapBarRoom - 1, W: u.box.W, H: 2}, kind: spotGapAfter}, true
	}
	if !pc.surface.shown {
		// A surface not shown stands in for its object (paints.go).
		return caretMetric{rect: standIn(pc.unit, pc.surface), kind: spotObject}, true
	}
	at := pc.stop.u
	if pc.surface.hasMarked && at == pc.surface.marked.Start {
		at = pc.surface.compSel
	}
	cr := pc.surface.caretRect(at)
	r := glyphBox(pc.surface, cr)
	x, y := pc.unit.origin(pc.surface)
	r.X += x
	r.Y += y
	if v := pc.unit.view; v.W > 0 {
		// Past what its unit shows, a place stands at the edge it lies
		// past.
		r.X = min(max(r.X, pc.unit.x+v.X), pc.unit.x+v.X+v.W-r.W)
	}
	cm := caretMetric{rect: r, kind: spotText, fontSize: pc.surface.fontSize}
	if l, ok := lineAt(pc.surface, cr); ok {
		cm.baseline = y + l.Baseline
		cm.hasBaseline = true
	}
	return cm, true
}

// caretBox is where a caret stands, in page coordinates.
func (pl *pageLayout) caretBox(c doc.Caret, m Metrics) (ui.Rect, bool) {
	r, _, ok := pl.spot(c)
	return r, ok
}

// glyphBox is r on its line of sl, from the line's ascent to its descent:
// the box its glyphs fill, without the leading between lines. A caret, a
// text paint and an inline object's outline all stand on it.
func glyphBox(sl *surfaceLayout, r ui.Rect) ui.Rect {
	if l, ok := lineAt(sl, r); ok {
		r.Y, r.H = l.Baseline-l.Ascent, l.Ascent+l.Descent
	}
	return r
}

// lineAt is the line of sl r's middle falls in.
func lineAt(sl *surfaceLayout, r ui.Rect) (ui.TextLine, bool) {
	for _, l := range sl.layout.Lines() {
		if mid := r.Y + r.H/2; mid >= l.Rect.Y && mid < l.Rect.Y+l.Rect.H {
			return l, true
		}
	}
	return ui.TextLine{}, false
}

// lookup finds a caret's place. The engine settles every caret on a
// presented stop, so the place is exact.
func (pl *pageLayout) lookup(c doc.Caret) (place, bool) {
	if u, ok := pl.slots[c.Pos.Ref()]; ok {
		if pc, ok := u.place(c); ok {
			return pc, true
		}
	}
	pc, ok := pl.gaps[c]
	return pc, ok
}

// place finds a caret among the unit's stops in its slots, indexing them
// the first time, once for the unit's layout.
func (u *unitLayout) place(c doc.Caret) (place, bool) {
	if u.at == nil {
		u.at = map[doc.Caret]place{}
		for _, s := range orderedStops(u) {
			if _, dup := u.at[s.stop.caret()]; s.surface != nil && !dup {
				u.at[s.stop.caret()] = place{unit: u, surface: s.surface, stop: s.stop}
			}
		}
	}
	pc, ok := u.at[c]
	return pc, ok
}

// hit is what a page point is on, as the place a click there sends: the
// stop nearest it, the object there, if any, and the side of the stop the
// grapheme under it lies on.
