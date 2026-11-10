package engine

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
)

// stopModel is a view's stop model over one document state, read from the
// presentation (§13.5): each unit's stop map, in page order. Notation is
// presented the same wherever the caret is, so every unit's stops are
// those the view shows.
type stopModel struct {
	pr    *present.Presenter
	prog  *policy.Program
	store *policy.Store
	r     doc.Reader
	view  *policy.View
	opts  present.Options
}

// stopsFor returns the stop model builder of a view.
func (e *Engine) stopsFor(viewID uint64) policy.StopsFunc {
	pr := e.presenter(viewID)
	opts := e.presentOptions(viewID)
	prog, store := e.prog, e.store
	return func(r doc.Reader, view *policy.View) policy.Stops {
		p := pr
		if _, ok := r.(*doc.Snapshot); !ok {
			// A transaction changes its nodes in place, so the view's
			// presentations, memoised by node pointer, must not see it.
			p = present.NewPresenter()
		}
		return &stopModel{pr: p, prog: prog, store: store, r: r, view: view, opts: opts}
	}
}

// Stops is the stop model of a view over the current document, as policies
// read it: for tests, and for hosts that need logical order.
func (e *Engine) Stops(viewID uint64) (policy.Stops, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[viewID]
	if !ok {
		return nil, false
	}
	view := *v
	return e.stopsFor(viewID)(e.snap, &view), true
}

// unit presents unit i as the view shows it.
func (m *stopModel) unit(i int) *present.Presentation {
	return m.pr.Unit(m.prog, m.store, m.r, m.view, m.opts, i)
}

// flowStops are a unit's stops that ordinary motion reaches from outside
// its cards.
func flowStops(p *present.Presentation) []present.Stop {
	if p == nil {
		return nil
	}
	out := make([]present.Stop, 0, len(p.Stops))
	for _, s := range p.Stops {
		if s.Class != present.StopEntry {
			out = append(out, s)
		}
	}
	return out
}

// locate finds c among stops: exactly, or else the place it would stand,
// as the index of the last stop before it (-1 when none is).
func locate(r doc.Reader, stops []present.Stop, c doc.Caret) (int, bool) {
	before := -1
	for i, s := range stops {
		if s.Caret == c {
			return i, true
		}
		if s.Caret.Pos.Ref() == c.Pos.Ref() && doc.Compare(r, s.Caret, c) < 0 {
			before = i
		}
	}
	return before, false
}

func (m *stopModel) Step(c doc.Caret, dir vocab.Direction) (doc.Caret, bool) {
	if dir == 0 {
		dir = vocab.Forward
	}
	i, ok := m.pr.UnitAt(m.r, c.Pos)
	if !ok {
		return c, false
	}
	p := m.unit(i)
	if p == nil {
		return c, false
	}
	// Within a card, motion stays among the card's stops; its edges lead
	// out beside its box, which is the stop next to the card's in order.
	stops := p.Stops
	j, exact := locate(m.r, stops, c)
	inCard := exact && stops[j].Class == present.StopEntry
	surface := -1
	if inCard {
		surface = stops[j].Surface
	}
	reach := func(s present.Stop) bool {
		return s.Class != present.StopEntry || s.Surface == surface
	}
	k := j
	if !exact && dir < 0 {
		// Not a stop itself: the stop before it is the first backward.
		k++
	}
	for {
		k += int(dir)
		if k < 0 || k >= len(stops) {
			break
		}
		if reach(stops[k]) && stops[k].Caret != c {
			return stops[k].Caret, true
		}
	}
	// Past the unit's edge: the nearest stop of the next unit with any.
	for u := i + int(dir); u >= 0; u += int(dir) {
		next := m.unit(u)
		if next == nil {
			break
		}
		fs := flowStops(next)
		if dir < 0 {
			for x := len(fs) - 1; x >= 0; x-- {
				if fs[x].Caret != c {
					return fs[x].Caret, true
				}
			}
			continue
		}
		for _, s := range fs {
			if s.Caret != c {
				return s.Caret, true
			}
		}
	}
	return c, false
}

func (m *stopModel) Edge(dir vocab.Direction) (doc.Caret, bool) {
	units := m.pr.Units(m.r)
	if dir < 0 {
		for i := range units {
			if fs := flowStops(m.unit(i)); len(fs) > 0 {
				return fs[0].Caret, true
			}
		}
		return doc.Caret{}, false
	}
	for i := len(units) - 1; i >= 0; i-- {
		if fs := flowStops(m.unit(i)); len(fs) > 0 {
			return fs[len(fs)-1].Caret, true
		}
	}
	return doc.Caret{}, false
}

// Normalize is total: a caret that is a stop stays; one at a position
// whose stop it is not takes the stop there nearest it toward the
// boundary, stop 0, so a place whose projected items went keeps its side
// of the content; any other takes the last stop before it in its unit, or
// the nearest stop of the page when no unit holds it, such as a gap
// between two text blocks.
func (m *stopModel) Normalize(c doc.Caret) doc.Caret {
	i, ok := m.pr.UnitAt(m.r, c.Pos)
	if !ok {
		return m.nearest(c)
	}
	p := m.unit(i)
	if p == nil || len(p.Stops) == 0 {
		return m.nearest(c)
	}
	best := -1
	for k, s := range p.Stops {
		if s.Caret == c {
			return c
		}
		if s.Caret.Pos != c.Pos {
			continue
		}
		toward := s.Caret.Stop <= c.Stop
		if c.Stop < 0 {
			toward = s.Caret.Stop >= c.Stop
		}
		if toward && (best < 0 || abs(s.Caret.Stop-c.Stop) < abs(p.Stops[best].Caret.Stop-c.Stop)) {
			best = k
		}
	}
	if best >= 0 {
		return p.Stops[best].Caret
	}
	if j, _ := locate(m.r, p.Stops, doc.CaretAt(c.Pos)); j >= 0 {
		return p.Stops[j].Caret
	}
	return p.Stops[0].Caret
}

func (m *stopModel) Around(pos doc.Pos) (first, last int) {
	i, ok := m.pr.UnitAt(m.r, pos)
	if !ok {
		return 0, 0
	}
	if p := m.unit(i); p != nil {
		for _, s := range p.Stops {
			if s.Caret.Pos == pos {
				first, last = min(first, s.Caret.Stop), max(last, s.Caret.Stop)
			}
		}
	}
	return first, last
}

func (m *stopModel) Bound(c doc.Caret) doc.Pos {
	i, ok := m.pr.UnitAt(m.r, c.Pos)
	if !ok {
		return c.Pos
	}
	if p := m.unit(i); p != nil {
		for _, s := range p.Stops {
			if s.Caret == c {
				return s.Bound(m.r)
			}
		}
	}
	return c.Pos
}

// nearest is the first flow stop at or after c in page order, or the
// page's last stop when none is.
func (m *stopModel) nearest(c doc.Caret) doc.Caret {
	units := m.pr.Units(m.r)
	var last *doc.Caret
	for i := range units {
		fs := flowStops(m.unit(i))
		if len(fs) == 0 {
			continue
		}
		if doc.Compare(m.r, fs[len(fs)-1].Caret, c) >= 0 {
			for _, s := range fs {
				if doc.Compare(m.r, s.Caret, c) >= 0 {
					return s.Caret
				}
			}
		}
		end := fs[len(fs)-1].Caret
		last = &end
	}
	if last != nil {
		return *last
	}
	return c
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
