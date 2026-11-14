package manifest

import (
	"reflect"
	"slices"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/tags"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
)

// checkKept checks that what the engine keeps of a view's page, its
// units and their presentations, is what a fresh presenter and a fresh
// fact store make of it.
func checkKept(t *testing.T, e *engine.Engine, when string) {
	t.Helper()
	snap := e.Snapshot()
	prog := e.Program()
	pr := present.NewPresenter(prog.Profile())
	units := e.Units(1)
	if fresh := pr.Units(snap); !reflect.DeepEqual(units, fresh) {
		t.Fatalf("%s: units kept\n%+v\nfresh\n%+v", when, units, fresh)
	}
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	view, _ := e.View(1)
	got := e.Present(1, ids)
	fresh := pr.Present(prog, policy.NewStore(), snap, &view, ids)
	for i := range got {
		// Read as assistive technology would, so both are made whole.
		got[i].AccessOffsets()
		fresh[i].AccessOffsets()
		if !reflect.DeepEqual(got[i], fresh[i]) {
			t.Fatalf("%s: unit %d kept\n%+v\nfresh\n%+v", when, i, got[i], fresh[i])
		}
	}
}

// A view's presentations, kept across edits by their keys, are those a
// fresh presenter makes: text, blocks made, moved into lists and quotes
// and taken out, rewrites and what deleting one leaves.
func TestKeptPresentationsAreFreshOnes(t *testing.T) {
	p := open(t)
	same := func(when string) {
		t.Helper()
		checkKept(t, p.e, when)
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
	noted.AccessOffsets()
	fresh.AccessOffsets()
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

// A text slot's runs are cut at every ghost and every styled edge, and
// each piece wears the groups of the styled spans covering it, in the
// order the policies style them, after its own and its marks'.
func TestRunsAreCutAtGhostsAndStyledEdges(t *testing.T) {
	p := open(t)
	for range 3 {
		p.typeText("Some **strong *both* text** and a #tag... `code` **#in** done")
		p.do(vocab.Break{})
	}
	prog, store, snap := p.e.Program(), p.e.Store(), p.e.Snapshot()
	view, _ := p.e.View(1)
	units := p.e.Units(1)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	styledPieces := 0
	for _, pres := range p.e.Present(1, ids) {
		for _, s := range pres.Surfaces {
			type span struct {
				from, to int
				group    vocab.Group
			}
			var spans []span
			edges := map[int]bool{}
			for _, st := range prog.Styled() {
				for _, f := range prog.FactsIn(store, snap, &view, s.Flow.Slot, st.Kind) {
					if f.Outer.From.At < f.Outer.To.At {
						spans = append(spans, span{f.Outer.From.At, f.Outer.To.At, st.Group})
						edges[f.Outer.From.At], edges[f.Outer.To.At] = true, true
					}
				}
			}
			for _, it := range s.Flow.Items {
				if it.Kind == present.GhostItem {
					edges[it.From] = true
				}
			}
			for _, it := range s.Flow.Items {
				if it.Kind != present.TextRun {
					continue
				}
				for at := it.From + 1; at < it.To; at++ {
					if edges[at] {
						t.Fatalf("run %d-%d %q crosses an edge at %d", it.From, it.To, it.Text, at)
					}
				}
				var want []vocab.Group
				for _, sp := range spans {
					if sp.from <= it.From && it.To <= sp.to {
						want = append(want, sp.group)
					}
				}
				if got := it.Groups[len(it.Groups)-len(want):]; !slices.Equal(got, want) {
					t.Fatalf("run %q wears %v, want it to end with %v", it.Text, it.Groups, want)
				}
				if len(want) > 0 {
					styledPieces++
				}
			}
		}
	}
	if styledPieces < 6 {
		t.Fatalf("only %d styled pieces: the page styles too little to test", styledPieces)
	}
}

// The facts the engine's store answers for a slot are a fresh store's,
// across text, blocks made and moved, rewrites and an annotation in it.
func TestKeptFactsAreFreshOnes(t *testing.T) {
	p := open(t)
	prog := p.e.Program()
	var kinds []policy.FactID
	kinds = append(kinds, prog.Projecting()...)
	for _, st := range prog.Styled() {
		kinds = append(kinds, st.Kind)
	}
	same := func(when string, view policy.View) {
		t.Helper()
		snap := p.e.Snapshot()
		for _, u := range p.e.Units(1) {
			n := mustNode(t, snap, u.ID)
			ref, ok := vocab.FirstTextSlot(n)
			if !ok {
				continue
			}
			for _, k := range kinds {
				got := prog.FactsIn(p.e.Store(), snap, &view, ref, k)
				want := prog.FactsIn(policy.NewStore(), snap, &view, ref, k)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: kind %v in %v: kept %v, fresh %v", when, k, ref, got, want)
				}
			}
		}
	}
	view := func() policy.View { v, _ := p.e.View(1); return v }
	p.typeText("# Head *em*")
	p.do(vocab.Break{})
	p.typeText("1. one **b** #tag")
	same("after a list", view())
	p.do(vocab.Break{})
	p.typeText("two `c`")
	same("after a second item", view())
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("> quote -")
	p.typeText("-")
	same("after a rewrite", view())
	for range 12 {
		p.do(left)
	}
	p.do(backspace)
	same("after a join", view())
	v := view()
	ref, _ := vocab.FirstTextSlot(mustNode(t, p.e.Snapshot(), p.e.Units(1)[0].ID))
	v.Annotations, _ = v.Annotations.Add(policy.Annotation{Kind: policy.Armed, Owner: tags.ID,
		Anchor: policy.Between(doc.At(ref, 0), doc.At(ref, 2)), Lifetime: policy.WithView})
	same("with an annotation", v)
	same("without it again", view())
}

// What a block's place spells stands before its first line: a line that
// a line typed before it moves down is read without the item's marker,
// as a fresh store reads it.
func TestALineMovedDownBearsNoMarker(t *testing.T) {
	p := open(t)
	p.typeText("- c**é")
	p.do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Backward})
	p.do(vocab.Type{Text: "\n"})
	checkKept(t, p.e, "after a line typed before the text")
}
