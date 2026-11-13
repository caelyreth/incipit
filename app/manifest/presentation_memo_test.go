package manifest

import (
	"reflect"
	"slices"
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
			// Read as assistive technology would, so both are made whole.
			got[i].AccessStops()
			fresh[i].AccessStops()
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
	noted.AccessStops()
	fresh.AccessStops()
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
