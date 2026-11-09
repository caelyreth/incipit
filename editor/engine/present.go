package engine

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
)

func (e *Engine) presenter(view uint64) *present.Presenter {
	p, ok := e.presenters[view]
	if !ok {
		p = present.NewPresenter()
		e.presenters[view] = p
	}
	return p
}

// SetPresentOptions sets a view's presentation inputs: reveal mode and
// ghost layout.
func (e *Engine) SetPresentOptions(view uint64, o present.Options) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.presentOpt[view] = o
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
	return e.presenter(view).Present(e.prog, e.store, e.snap, v, e.presentOpt[view], ids)
}

// Decorations returns a view's decorations.
func (e *Engine) Decorations(view uint64) present.Decorations {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[view]
	if !ok {
		return present.Decorations{}
	}
	return present.Decorate(e.prog, e.store, e.snap, v)
}
