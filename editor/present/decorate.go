package present

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Highlight is a styled range: a matching pair, a scope, an armed
// structure, a pulse.
type Highlight struct {
	Group    vocab.Group
	From, To doc.Pos
	// Node is set for a highlight over a whole node.
	Node doc.ID
}

// Decorations are a view's per-view decorations, separate from every unit:
// moving the caret changes them, and changes units only where reveal
// changes.
type Decorations struct {
	Selection   doc.Selection
	Composition *policy.Composition
	Highlights  []Highlight
}

// Decorate computes a view's decorations.
// TODO(guides): guides from previews of Leave, Return, SelectScope and an
// armed deletion, once the guides policy contributes them.
func Decorate(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View) Decorations {
	d := Decorations{Selection: view.Selection, Composition: view.Composition}
	for _, dec := range prog.Decorations() {
		for _, a := range view.Annotations.Of(dec.Kind) {
			d.Highlights = append(d.Highlights, Highlight{Group: dec.Group, From: a.Anchor.From, To: a.Anchor.To})
		}
	}
	if view.Selection.Kind != doc.SelectText {
		return d
	}
	head := view.Selection.Head.Pos
	for _, pc := range prog.Paints() {
		if pc.When != policy.WhenCaretInside {
			continue
		}
		var inner *policy.Fact
		for _, f := range prog.FactsIn(store, r, view, head.Ref(), pc.Fact) {
			if f.Inside(head.Offset) {
				f := f
				inner = &f
			}
		}
		if inner != nil {
			ref := head.Ref()
			d.Highlights = append(d.Highlights, Highlight{Group: pc.Group, From: doc.At(ref, inner.From), To: doc.At(ref, inner.To)})
		}
	}
	return d
}
