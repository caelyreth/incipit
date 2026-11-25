package page

import (
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// A long page is presented and laid out near its view only (§14.2): the
// units within keepViews views of what the element shows, and of the
// selection's ends, wherever the page is scrolled. Every other unit is a
// sketch: a stand-in the engine gives without its text
// (present.Presentation.Sketch), placed at the height it was last laid out
// at, or at an estimate where it never was, holding no presentation of its
// text, no shaping and no stops. Coming near, a sketch is presented and
// laid out, and where its height was an estimate the page above the view
// grows or shrinks by the difference: the element moves its scroll by as
// much, so what it shows stays where it is (anchorShift).

// window is the band of the page the element shows, in page coordinates,
// set before each layout; all lays out every unit, as a page laid out
// still or read by assistive technology. Unset, as an editor no element
// shows, it lays out every unit.
type window struct {
	top, bottom float32
	all, set    bool
}

// SetWindow tells the editor which band of the page its element shows,
// from top to bottom in page coordinates, or that it lays out all of it.
func (ed *Editor) SetWindow(top, bottom float32, all bool) {
	ed.window = window{top: top, bottom: bottom, all: all, set: true}
}

// whole reports whether the window lays out every unit.
func (w window) whole() bool { return !w.set || w.all || w.bottom <= w.top }

// band is the stretch of the page laid out around the window.
func (w window) band() (float32, float32) {
	v := w.bottom - w.top
	return w.top - keepViews*v, w.bottom + keepViews*v
}

// covers reports whether a layout laid out over [from, to] of a page h
// high still covers the window: it is laid out again once the window
// comes within a view of the band's edge.
func (w window) covers(laid window, h float32) bool {
	if w.whole() || laid.whole() {
		return w.whole() == laid.whole()
	}
	v := w.bottom - w.top
	from, to := laid.band()
	return (from <= 0 || w.top-v >= from) && (to >= h || w.bottom+v <= to)
}

// measured is how a unit was last laid out: its node and the measures it
// was laid out at, its height and its shape (shapeOf).
type measured struct {
	node    *doc.Node
	metrics Metrics
	theme   uintptr
	h       float32
	shape   int
}

// estimated is a sketch's height estimated from its node's text.
type estimated struct {
	node    *doc.Node
	metrics Metrics
	h       float32
}

// heightOf is where a sketch is placed: its height as last laid out
// while its node and measures are those it was laid out with, else an
// estimate; exact reports which.
func (lc *layoutCache) heightOf(p *present.Presentation, th present.Theme, m Metrics) (float32, bool) {
	km := keyMetrics(p, m)
	if ms, ok := lc.measured[p.Unit]; ok && ms.node == p.Node && ms.metrics == km && ms.theme == lc.themeID {
		return ms.h, true
	}
	if e, ok := lc.estimates[p.Unit]; ok && e.node == p.Node && e.metrics == km {
		return e.h, false
	}
	h := estimate(p, th, m)
	lc.estimates[p.Unit] = estimated{node: p.Node, metrics: km, h: h}
	return h, false
}

// estimate is a unit's height from its text alone: as many lines as its
// letters fill at an average advance, a wide letter twice a narrow one;
// an object a few lines of body text.
func estimate(p *present.Presentation, th present.Theme, m Metrics) float32 {
	size := m.Body
	if s := th.Resolve([]vocab.Group{p.Group}).Size; s != 0 {
		size = m.Body * s
	}
	line := max(lineHeight(m, size), 1)
	if p.Object || p.Node == nil {
		return 3 * lineHeight(m, m.Body)
	}
	width := max(m.Width-indentOf(p.Context, m), size)
	var advance float32
	for _, r := range p.Node.Text() {
		switch {
		case r == '\n':
			// A line break ends a line of its own.
			advance = float32(math.Ceil(float64(advance/width))) * width
		case r > utf8.RuneSelf && (unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)):
			advance += size
		default:
			advance += size * 0.5
		}
	}
	lines := max(float32(math.Ceil(float64(advance/width))), 1)
	return lines * line
}

// nearUnits says which of units are laid out: every unit where the window
// lays out all; else those whose place, as the last layout and the
// sketches' heights have them, falls within the window's band, and those
// around the selection's ends within it.
func (ed *Editor) nearUnits(units []present.UnitRef, sketches []*present.Presentation, th present.Theme, m Metrics) []bool {
	near := make([]bool, len(units))
	w := ed.window
	if w.whole() {
		for i := range near {
			near[i] = true
		}
		return near
	}
	from, to := w.band()
	v := w.bottom - w.top
	// The selection's ends, and a view either side of the head, so a
	// caret moved by line or by page finds the lines beside its own.
	var ends []int
	sel := ed.Selection()
	for _, pos := range []doc.Pos{sel.Head.Pos, sel.Anchor.Pos} {
		if sel.Kind == doc.SelectNode {
			pos = doc.Pos{Node: sel.Node}
		}
		if i, ok := ed.Engine.UnitAt(ed.View, pos); ok {
			ends = append(ends, i)
		}
	}
	if sel.Kind == doc.SelectNode {
		for i, u := range units {
			if u.ID == sel.Node {
				ends = append(ends, i)
			}
		}
	}
	for id := range ed.asks {
		for i, u := range units {
			if u.ID == id {
				near[i] = true
			}
		}
	}
	y := float32(0)
	ys := make([]float32, len(units))
	hs := make([]float32, len(units))
	var prev *present.Presentation
	for i, p := range sketches {
		if p == nil {
			continue
		}
		if prev != nil {
			y += m.Gap * ed.cache.gapBetween(prev, p, th)
		}
		prev = p
		var h float32
		if u, ok := ed.cache.units[p.Unit]; ok {
			h = u.h
		} else if u, ok := ed.cache.sketches[p.Unit]; ok && u.pres == p {
			h = u.h
		} else {
			h, _ = ed.cache.heightOf(p, th, m)
		}
		ys[i], hs[i] = y, h
		if y+h >= from && y <= to {
			near[i] = true
		}
		y += h
	}
	for k, i := range ends {
		near[i] = true
		if k > 0 {
			continue
		}
		// Around the head.
		for j := i - 1; j >= 0 && ys[j]+hs[j] >= ys[i]-v; j-- {
			near[j] = true
		}
		for j := i + 1; j < len(units) && ys[j] <= ys[i]+hs[i]+v; j++ {
			near[j] = true
		}
	}
	return near
}

// stood is a unit as a layout placed it above the window: its node and
// height.
type stood struct {
	id   doc.ID
	node *doc.Node
	h    float32
}

// above are the units of pl wholly above the window, in order, as they
// stand before the layout that follows changes them.
func above(pl *pageLayout, w window) []stood {
	if pl == nil || w.whole() {
		return nil
	}
	var out []stood
	for _, u := range pl.units {
		if u.top+u.h > w.top {
			break
		}
		out = append(out, stood{id: u.pres.Unit, node: u.pres.Node, h: u.h})
	}
	return out
}

// grewAbove is how much taller the units before stand in pl, counting only
// those still drawn from the same node, so laid out again at another
// height than they were placed at, not changed: the page above the
// window grew by as much without anything in it changing.
func grewAbove(before []stood, pl *pageLayout) float32 {
	d := float32(0)
	for i, b := range before {
		if i >= len(pl.units) {
			break
		}
		u := pl.units[i]
		if u.pres.Unit != b.id {
			// The page's units changed: what follows is not the same page.
			break
		}
		if u.pres.Node == b.node {
			d += u.h - b.h
		}
	}
	return d
}

// setWindow tells the editor what the element shows of the page: every
// unit while it is laid out still or read by assistive technology, and
// from the page's top to a saved offset's view while that offset waits to
// be restored, so it is restored exactly.
func (el *Element) setWindow() {
	ed := el.Editor
	h := el.viewport.H
	switch {
	case el.still || el.assistive || h <= 0:
		ed.SetWindow(0, 0, true)
	case el.restoreScroll != nil:
		ed.SetWindow(0, max(*el.restoreScroll-el.Top, 0)+h, false)
	default:
		top := el.offset() - el.Top
		ed.SetWindow(top, top+h, false)
	}
}

// leftBand reports whether the view moved out of what the page was laid
// out around.
func (el *Element) leftBand() bool {
	ed := el.Editor
	if ed.layout == nil {
		return false
	}
	el.setWindow()
	return !ed.window.covers(ed.laid, ed.layout.rest)
}
