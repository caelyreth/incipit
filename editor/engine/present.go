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
		p = present.NewPresenter(e.prog.Profile())
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

// Decorations returns a view's decorations over every unit.
func (e *Engine) Decorations(view uint64) present.Decorations {
	units := e.Units(view)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	return e.DecorationsOf(view, e.Present(view, ids))
}

// DecorationsOf returns a view's decorations over the units presented as
// pres, which a renderer has just pulled with Present: they are not
// presented again.
func (e *Engine) DecorationsOf(view uint64, pres []*present.Presentation) present.Decorations {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[view]
	if !ok {
		return present.Decorations{}
	}
	d := present.Decorate(e.prog, e.store, e.snap, v, e.stopsFor(view), pres)
	d.Guides = e.guides(view, v, pres)
	return d
}

// guides previews each guide's command and shows those that would take
// the selection elsewhere in the shape it has: a caret to another caret,
// a range, selecting only, to another range, which the guide covers. A
// command that would turn one into the other offers nothing to show
// where it acts. The caller holds mu.
func (e *Engine) guides(viewID uint64, v *policy.View, pres []*present.Presentation) []present.Guide {
	if v.Composition != nil {
		return nil
	}
	point := v.Selection.IsCollapsed()
	var out []present.Guide
	for _, g := range e.prog.Guides() {
		cmd, ok := e.prog.Command(g.Command)
		if !ok {
			continue
		}
		res, err := e.process(viewID, vocab.Input{Intent: cmd.Intent, At: e.at}, false)
		to := res.preview.Selection
		if err != nil || !res.preview.Available || to == v.Selection || to.IsCollapsed() != point {
			continue
		}
		guide := present.Guide{Command: g.Command, At: to.Head}
		if !point {
			if len(res.preview.Steps) > 0 {
				continue
			}
			pt, ok := present.SelectionPaint(e.snap, pres, to)
			if !ok {
				continue
			}
			guide.Covers = pt.Covers
		}
		out = append(out, guide)
	}
	return out
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
	var s vocab.Slice
	if sp, ok := e.spanned(v.Selection); ok {
		s, _ = sp.Copy(v.Selection)
	} else {
		from, to := policy.SelectionRange(e.snap, e.stopsFor(viewID)(e.snap, v), v.Selection)
		s = vocab.Copy(e.snap, from, to)
	}
	return s, vocab.PlainText(e.snap.Schema(), s)
}

// spanned is the object a selection of its own shape selects in, as a
// cell range is its table's.
func (e *Engine) spanned(sel doc.Selection) (vocab.Spanned, bool) {
	if sel.Kind != doc.SelectCells {
		return nil, false
	}
	n, ok := e.snap.Node(sel.Cells.Table)
	if !ok {
		return nil, false
	}
	sp, ok := e.prog.Profile().Role(e.snap, n, vocab.RoleSpanned.ID)
	if !ok {
		return nil, false
	}
	return sp.(vocab.Spanned), true
}
