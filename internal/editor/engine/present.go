package engine

import (
	"slices"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/vocab"
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

// Sketch returns stand-ins for units a renderer places without their
// text, far from its view (present.Presenter.Sketch).
func (e *Engine) Sketch(view uint64, ids []doc.ID) []*present.Presentation {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.presenter(view).Sketch(e.snap, ids)
}

// UnitAt is the index in Units of the unit holding pos's stops, as
// present.Presenter.UnitAt finds it.
func (e *Engine) UnitAt(view uint64, pos doc.Pos) (int, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.presenter(view).UnitAt(e.snap, pos)
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
	if v.Composition != nil || v.Suggestion != nil {
		return nil
	}
	point := v.Selection.IsCollapsed()
	look, _ := e.prog.Profile().(vocab.Presentation)
	claimedKey := map[string]bool{}
	var out []present.Guide
	for _, g := range e.prog.Guides() {
		cmd, ok := e.prog.Command(g.Command)
		if !ok || cmd.Key != "" && claimedKey[cmd.Key] {
			continue
		}
		res, err := e.process(viewID, vocab.Input{Intent: cmd.Intent, At: e.at}, false)
		to := res.preview.Selection
		if err != nil || !res.preview.Available || to.IsCollapsed() != point {
			continue
		}
		if point && len(res.preview.Steps) > 0 && res.snap != nil && look != nil {
			curIndent, ok1 := containerIndent(look, e.snap, v.Selection.Head.Pos.Node)
			nextIndent, ok2 := containerIndent(look, res.snap, to.Head.Pos.Node)
			if ok1 && ok2 && nextIndent != curIndent {
				// An indent moves the whole unit: its guide stands at the
				// unit's start, on its first line, wherever the caret is.
				at := v.Selection.Head
				if unit := unitIDFor(e.snap, pres, at.Pos.Node); unit != 0 {
					at = doc.Caret{Pos: doc.Pos{Node: unit}}
				}
				out = append(out, present.Guide{Command: g.Command, At: at, Shift: nextIndent - curIndent})
				if cmd.Key != "" {
					claimedKey[cmd.Key] = true
				}
				continue
			}
		}
		if to == v.Selection {
			continue
		}
		guide := present.Guide{Command: g.Command, At: to.Head}
		if point {
			if _, exists := e.snap.Node(to.Head.Pos.Node); !exists {
				guide.After = unitIDFor(e.snap, pres, v.Selection.Head.Pos.Node)
			}
		} else {
			if len(res.preview.Steps) > 0 {
				continue
			}
			pt, ok := present.SelectionPaint(e.snap, pres, to)
			if !ok {
				continue
			}
			guide.Covers = pt.Covers
		}
		if cmd.Key != "" {
			claimedKey[cmd.Key] = true
		}
		out = append(out, guide)
	}
	return out
}

func containerIndent(look vocab.Presentation, r doc.Reader, id doc.ID) (int, bool) {
	anc := doc.Ancestors(r, id)
	if len(anc) == 0 {
		return 0, false
	}
	slices.Reverse(anc)
	path := append(anc[1:], id)
	ctx := vocab.Container{}
	for _, pid := range path {
		n, ok := r.Node(pid)
		if !ok {
			return 0, false
		}
		c := look.Contain(r, n, ctx)
		if pid == id && c.Unit {
			ctx = c.Own
			break
		}
		if !c.Open {
			break
		}
		ctx = c.Inner
	}
	return ctx.Indent, true
}

func unitIDFor(r doc.Reader, pres []*present.Presentation, id doc.ID) doc.ID {
	hasUnit := func(target doc.ID) bool {
		return slices.ContainsFunc(pres, func(p *present.Presentation) bool { return p.Unit == target })
	}
	if hasUnit(id) {
		return id
	}
	for _, aid := range doc.Ancestors(r, id) {
		if hasUnit(aid) {
			return aid
		}
	}
	return 0
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
