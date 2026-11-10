package deletion

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/guides"
	"github.com/caelyreth/neverland/editor/policies/lists"
	"github.com/caelyreth/neverland/editor/policies/markdown"
	"github.com/caelyreth/neverland/editor/policies/navigation"
	"github.com/caelyreth/neverland/editor/policies/notation"
	"github.com/caelyreth/neverland/editor/policies/pairs"
	"github.com/caelyreth/neverland/editor/policies/scopes"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func open(t *testing.T) *page {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{
		Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy,
			lists.Policy, navigation.Policy, scopes.Policy, Policy, guides.Policy},
		Strict: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := vocab.EmptyPage(1, 5, doc.NewInterner())
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
	for _, a := range v.Annotations.Of(vocab.Armed) {
		if a.Owner == ID {
			n++
		}
	}
	return n
}

func TestDeletingASelectedStructureArmsThenRemoves(t *testing.T) {
	p := open(t)
	p.typeText("---")
	p.do(vocab.Break{})
	divider := p.e.Snapshot().Root().Slot(0).Child(0)
	p.do(vocab.Place{Node: divider.ID()})

	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(vocab.Divider) != 1 || p.armed() != 1 {
		t.Fatalf("first delete left %d dividers and %d arms", p.count(vocab.Divider), p.armed())
	}
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(vocab.Divider) != 0 || p.armed() != 0 {
		t.Fatalf("second delete left %d dividers and %d arms", p.count(vocab.Divider), p.armed())
	}
}

func TestAnyOtherIntentDisarmsAStructure(t *testing.T) {
	p := open(t)
	p.typeText("---")
	p.do(vocab.Break{})
	divider := p.e.Snapshot().Root().Slot(0).Child(0)
	p.do(vocab.Place{Node: divider.ID()})
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.armed() != 1 {
		t.Fatal("the structure did not arm")
	}
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.armed() != 0 || p.count(vocab.Divider) != 1 {
		t.Fatalf("moving left %d arms and %d dividers", p.armed(), p.count(vocab.Divider))
	}
}

func TestDeletingASelectedFormulaRemovesTheInlineStructure(t *testing.T) {
	p := open(t)
	p.typeText("a $x")
	p.do(vocab.SelectScope{})
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(vocab.Formula) != 1 || p.armed() != 1 {
		t.Fatalf("first delete left %d formulas and %d arms", p.count(vocab.Formula), p.armed())
	}
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(vocab.Formula) != 0 {
		t.Fatal("the second delete kept the formula")
	}
	para := p.e.Snapshot().Root().Slot(0).Child(0)
	if got := para.Slot(0).Content().Text(); got != "a " {
		t.Fatalf("paragraph text %q, want %q", got, "a ")
	}
}

func TestDeletingAListScopeUsesTheStructureRole(t *testing.T) {
	p := open(t)
	p.typeText("- a")
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(vocab.SelectScope{})
	p.do(vocab.SelectScope{})
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(vocab.List) != 1 || p.armed() != 1 {
		t.Fatalf("first delete left %d lists and %d arms", p.count(vocab.List), p.armed())
	}
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if p.count(vocab.List) != 0 {
		t.Fatal("the second delete kept the list")
	}
}
