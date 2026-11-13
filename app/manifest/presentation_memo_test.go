package manifest

import (
	"reflect"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policies/tags"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
)

// A view's presentations, kept across edits by their keys, are those a
// fresh presenter makes: text, blocks made, moved into lists and quotes
// and taken out, rewrites and what deleting one leaves.
func TestKeptPresentationsAreFreshOnes(t *testing.T) {
	p := open(t)
	same := func(when string) {
		t.Helper()
		units := p.e.Units(1)
		ids := make([]doc.ID, len(units))
		for i, u := range units {
			ids[i] = u.ID
		}
		view, _ := p.e.View(1)
		got := p.e.Present(1, ids)
		fresh := present.NewPresenter(p.e.Program().Profile()).Present(p.e.Program(), p.e.Store(), p.e.Snapshot(), &view, ids)
		for i := range got {
			if !reflect.DeepEqual(got[i], fresh[i]) {
				t.Fatalf("%s: unit %d kept\n%+v\nfresh\n%+v", when, i, got[i], fresh[i])
			}
		}
	}
	p.do(vocab.Paste{Text: "# Head\n\nSome **strong** text (pairs [nested]) and a #tag...\n\n1. one\n2. two\n\n> quoted\n"})
	same("after the paste")
	p.typeText("more")
	same("after typing")
	p.do(vocab.Break{})
	p.typeText("- ")
	same("after a list began")
	p.typeText("x")
	p.do(vocab.Break{})
	p.typeText("y")
	same("after a new item")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("> ")
	same("after a quote began")
	p.typeText("a -")
	p.typeText("-")
	same("after a rewrite")
	p.do(backspace)
	same("after deleting the rewrite")
	for range 30 {
		p.do(left)
	}
	p.do(backspace)
	same("after joining")
	p.do(extend(left))
	p.do(extend(left))
	same("with a range")
	for range 6 {
		p.do(vocab.Undo{})
	}
	same("after undoing")
}

// An annotation an analyzing policy owns in a unit is part of what its
// facts depend on: a presentation kept without it is made again with it.
func TestAnAnnotationInAUnitPresentsItAgain(t *testing.T) {
	p := open(t)
	p.typeText("one #tag")
	unit := p.e.Units(1)[0].ID
	prog, store, snap := p.e.Program(), p.e.Store(), p.e.Snapshot()
	pr := present.NewPresenter(prog.Profile())
	view, _ := p.e.View(1)
	plain := pr.Present(prog, store, snap, &view, []doc.ID{unit})[0]
	ref, _ := vocab.FirstTextSlot(mustNode(t, snap, unit))
	view.Annotations, _ = view.Annotations.Add(policy.Annotation{Kind: policy.Armed, Owner: tags.ID,
		Anchor: policy.Between(doc.At(ref, 0), doc.At(ref, 3)), Lifetime: policy.WithView})
	noted := pr.Present(prog, store, snap, &view, []doc.ID{unit})[0]
	fresh := present.NewPresenter(prog.Profile()).Present(prog, store, snap, &view, []doc.ID{unit})[0]
	if noted == plain || !reflect.DeepEqual(noted, fresh) {
		t.Fatal("the presentation kept without the annotation stood with it")
	}
}

func mustNode(t *testing.T, r doc.Reader, id doc.ID) *doc.Node {
	t.Helper()
	n, ok := r.Node(id)
	if !ok {
		t.Fatalf("no node %v", id)
	}
	return n
}
