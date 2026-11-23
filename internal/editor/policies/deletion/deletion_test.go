package deletion

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policies/core"
	"github.com/caelyreth/neverland/internal/editor/policies/formulas"
	"github.com/caelyreth/neverland/internal/editor/policies/guides"
	"github.com/caelyreth/neverland/internal/editor/policies/history"
	"github.com/caelyreth/neverland/internal/editor/policies/lists"
	"github.com/caelyreth/neverland/internal/editor/policies/markdown"
	"github.com/caelyreth/neverland/internal/editor/policies/navigation"
	"github.com/caelyreth/neverland/internal/editor/policies/notation"
	"github.com/caelyreth/neverland/internal/editor/policies/pairs"
	"github.com/caelyreth/neverland/internal/editor/policies/scopes"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func open(t *testing.T) *page {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy, formulas.Policy,
			lists.Policy, navigation.Policy, scopes.Policy, Policy, history.Policy, guides.Policy},
		Resolve: []policy.Order{policy.Over(formulas.Policy, navigation.Policy, vocab.RoleCloser)},
		Strict:  true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 5, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true})
	e.OpenView(1)
	return &page{t: t, e: e, now: time.Unix(10, 0)}
}

func (p *page) do(in vocab.Intent) {
	p.t.Helper()
	p.now = p.now.Add(time.Second)
	if _, err := p.e.Do(1, vocab.Input{Intent: in, At: p.now}); err != nil {
		p.t.Fatal(err)
	}
}

func (p *page) typeText(s string) {
	p.t.Helper()
	for _, r := range s {
		p.do(vocab.Type{Text: string(r)})
	}
}

func (p *page) count(kind doc.Kind) int {
	n := 0
	p.e.Snapshot().Root().Walk(func(node *doc.Node) bool {
		if node.Kind() == kind {
			n++
		}
		return true
	})
	return n
}

func (p *page) armed() int {
	v, _ := p.e.View(1)
	n := 0
	for _, a := range v.Annotations.Of(policy.Armed) {
		if a.Owner == ID {
			n++
		}
	}
	return n
}

func TestDeletingASelectedObjectRemovesItAtOnce(t *testing.T) {
	p := open(t)
	p.typeText("---")
	p.do(vocab.Break{})
	divider := p.e.Snapshot().Root().Slot(0).Child(0)
	p.do(vocab.Place{Node: divider.ID()})

	// Selected whole, it already shows itself whole, as an armed
	// structure does: the delete removes it.
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(profile.Divider) != 0 || p.armed() != 0 {
		t.Fatalf("the delete left %d dividers and %d arms", p.count(profile.Divider), p.armed())
	}
}

func TestDeletingASelectedFormulaRemovesTheInlineStructure(t *testing.T) {
	p := open(t)
	p.typeText("a $x")
	p.do(vocab.SelectScope{})
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(profile.Formula) != 1 || p.armed() != 1 {
		t.Fatalf("first delete left %d formulas and %d arms", p.count(profile.Formula), p.armed())
	}
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(profile.Formula) != 0 {
		t.Fatal("the second delete kept the formula")
	}
	para := p.e.Snapshot().Root().Slot(0).Child(0)
	if got := para.Slot(0).Content().Text(); got != "a " {
		t.Fatalf("paragraph text %q, want %q", got, "a ")
	}
}

// first is the text of the page's first block.
func (p *page) first() string {
	return p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Content().Text()
}

// endOfFirst puts the caret at the end of the page's first block.
func (p *page) endOfFirst() {
	p.t.Helper()
	b := p.e.Snapshot().Root().Slot(0).Child(0)
	ref := doc.SlotRef{Node: b.ID(), Slot: 0}
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, b.Slot(0).Len()))})
}

// A container is no whole: what stands beside the caret at its edge is its
// first block, whose text a delete forward joins up, as a delete back from
// below joins into its last block's.
func TestDeletingForwardIntoAContainerJoinsItsFirstBlock(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    doc.Kind
		lines   []string
		left    int
		wantTop string
	}{
		{"list", profile.List, []string{"- a", "b"}, 1, "abovea"},
		{"one item", profile.List, []string{"- a"}, 0, "abovea"},
		{"quote", profile.Quote, []string{"> q", "r"}, 1, "aboveq"},
	} {
		p := open(t)
		p.typeText("above")
		p.do(vocab.Break{})
		for i, line := range tc.lines {
			if i > 0 {
				p.do(vocab.Break{})
			}
			p.typeText(line)
		}
		p.endOfFirst()
		p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
		if p.armed() != 0 {
			t.Fatalf("%s: a delete forward armed the container", tc.name)
		}
		if got := p.first(); got != tc.wantTop {
			t.Fatalf("%s: the first block reads %q, want %q", tc.name, got, tc.wantTop)
		}
		if got := p.count(tc.kind); got != tc.left {
			t.Fatalf("%s: %d containers left, want %d", tc.name, got, tc.left)
		}
	}
}

// Before the first item's marker, Backspace crosses the boundary above
// the list, which joins the item into the block before it, as before a
// heading's.
func TestDeletingBackBeforeTheFirstMarkerJoinsTheBlockAbove(t *testing.T) {
	p := open(t)
	p.typeText("above")
	p.do(vocab.Break{})
	p.typeText("- a")
	// Past the "a" and the marker's two glyphs: the stop before the marker.
	for range 3 {
		p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	}
	if v, _ := p.e.View(1); v.Selection.Head.Pos.Offset != 0 || v.Selection.Head.Stop >= 0 {
		t.Fatalf("the caret is at %+v, want the stop before the marker", v.Selection.Head)
	}
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if got := p.first(); got != "abovea" || p.count(profile.List) != 0 {
		t.Fatalf("the first block reads %q with %d lists, want %q and none", got, p.count(profile.List), "abovea")
	}
}
