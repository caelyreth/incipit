package engine

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
)

func (e *Engine) presenter(view uint64) *present.Presenter {
	p, ok := e.presenters[view]
	if !ok {
		p = present.NewPresenter()
		e.presenters[view] = p
	}
	return p
}

// Units lists the page's presentation units in order.
func (e *Engine) Units(view uint64) []present.UnitRef {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.presenter(view).Units(e.snap)
}

// Present returns the presentations of units, memoised: an unchanged
// unit returns the same pointer.
func (e *Engine) Present(view uint64, ids []doc.ID) []*present.Presentation {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[view]
	if !ok {
		return make([]*present.Presentation, len(ids))
	}
	return e.presenter(view).Present(e.prog, e.store, e.snap, v, ids)
}

// Decorations returns a view's decorations.
func (e *Engine) Decorations(view uint64) present.Decorations {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[view]
	if !ok {
		return present.Decorations{}
	}
	d := present.Decorate(e.prog, e.store, e.snap, v)
	d.Covers = e.covers(view, v)
	d.Guides = e.guides(view, v)
	return d
}

// guides previews each guide's command at a collapsed caret and shows
// those that would act. The caller holds mu.
func (e *Engine) guides(viewID uint64, v *policy.View) []present.Guide {
	if v.Selection.Kind != doc.SelectText || !v.Selection.IsCollapsed() || v.Composition != nil {
		return nil
	}
	var out []present.Guide
	for _, g := range e.prog.Guides() {
		cmd, ok := e.prog.Command(g.Command)
		if !ok {
			continue
		}
		res, err := e.process(viewID, vocab.Input{Intent: cmd.Intent, At: e.at}, false)
		if err != nil || !res.preview.Available {
			continue
		}
		out = append(out, present.Guide{Command: g.Command, At: res.preview.Selection.Head})
	}
	return out
}

// covers is what a view's selection covers in each unit it reaches: the
// stops between its ends in page order, and the blocks whose start the
// selection's range holds. The caller holds mu.
func (e *Engine) covers(viewID uint64, v *policy.View) []present.Cover {
	sel := v.Selection
	if sel.Kind == doc.SelectText && sel.IsCollapsed() {
		return nil
	}
	pr := e.presenter(viewID)
	units := pr.Units(e.snap)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	pres := pr.Present(e.prog, e.store, e.snap, v, ids)
	if sel.Kind == doc.SelectNode {
		for _, p := range pres {
			if p != nil && p.Unit == sel.Node {
				return []present.Cover{{Unit: p.Unit, Whole: true}}
			}
		}
		return nil
	}
	// The selection's ends, by their place in page order; a selection
	// whose ends are not both presented stops covers nothing.
	lo, hi, seq := -1, -1, 0
	for _, p := range pres {
		for _, st := range p.Stops {
			switch st.Caret {
			case sel.Anchor:
				lo = seq
			case sel.Head:
				hi = seq
			}
			seq++
		}
	}
	if lo < 0 || hi < 0 {
		return nil
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	from, to := policy.SelectionRange(e.snap, e.stopsFor(viewID)(e.snap, v), sel)
	var out []present.Cover
	seq = 0
	for _, p := range pres {
		c := present.Cover{Unit: p.Unit}
		for i, st := range p.Stops {
			if seq >= lo && seq <= hi {
				n := len(c.Spans)
				if n > 0 && c.Spans[n-1].Surface == st.Surface && c.Spans[n-1].To == i-1 {
					c.Spans[n-1].To = i
				} else {
					c.Spans = append(c.Spans, present.CoverSpan{Surface: st.Surface, From: i, To: i})
				}
			}
			seq++
		}
		c.Whole = e.holdsStart(p, from, to)
		if len(c.Spans) > 0 || c.Whole {
			out = append(out, c)
		}
	}
	return out
}

// holdsStart reports whether a range holds a unit's block from before its
// start: its marker and structure are then in it. The block is the item a
// marker belongs to, or the unit itself.
func (e *Engine) holdsStart(p *present.Presentation, from, to doc.Pos) bool {
	block := p.Unit
	if mk := p.Context.Marker; mk != nil && mk.Item != 0 {
		block = mk.Item
	}
	parent, slot, index, ok := vocab.Index(e.snap, block)
	if !ok {
		return false
	}
	gap := doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: index})
	return doc.Compare(e.snap, doc.CaretAt(from), gap) <= 0 && doc.Compare(e.snap, doc.CaretAt(to), gap) > 0
}

// Copy is the slice and plain text of what a view's selection covers: the
// range every edit of the selection reads, so a copy from before a marker
// holds its block.
func (e *Engine) Copy(viewID uint64) (vocab.Slice, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[viewID]
	if !ok || (v.Selection.Kind == doc.SelectText && v.Selection.IsCollapsed()) {
		return vocab.Slice{}, ""
	}
	from, to := policy.SelectionRange(e.snap, e.stopsFor(viewID)(e.snap, v), v.Selection)
	s := vocab.Copy(e.snap, from, to)
	return s, vocab.PlainText(s)
}
