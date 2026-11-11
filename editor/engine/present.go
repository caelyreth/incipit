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

// Decorations returns a view's decorations.
func (e *Engine) Decorations(view uint64) present.Decorations {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[view]
	if !ok {
		return present.Decorations{}
	}
	pr := e.presenter(view)
	units := pr.Units(e.snap)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	d := present.Decorate(e.prog, e.store, e.snap, v, pr.Present(e.prog, e.store, e.snap, v, ids))
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
