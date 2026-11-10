package scopes

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
			lists.Policy, navigation.Policy, Policy, guides.Policy},
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

func (p *page) selection() doc.Selection {
	p.t.Helper()
	v, ok := p.e.View(1)
	if !ok {
		p.t.Fatal("no view")
	}
	return v.Selection
}

func TestSelectScopeGrowsFromTheInnermostScope(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	p.do(vocab.SelectScope{})
	sel := p.selection()
	if sel.Kind != doc.SelectText {
		t.Fatalf("selection kind %v", sel.Kind)
	}
	from, to := sel.Ordered(p.e.Snapshot())
	if from.Offset != 3 || to.Offset != 4 || from.Pos.Ref() != to.Pos.Ref() {
		t.Fatalf("first scope is %v..%v, want the pair's text 3..4", from, to)
	}

	p.do(vocab.SelectScope{})
	from, to = p.selection().Ordered(p.e.Snapshot())
	if from.Offset != 0 || to.Offset != 5 {
		t.Fatalf("second scope is %d..%d, want the paragraph and page 0..5", from.Offset, to.Offset)
	}
	out, err := p.e.Preview(1, vocab.Input{Intent: vocab.SelectScope{}, At: p.now})
	if err != nil {
		t.Fatal(err)
	}
	if out.Available {
		t.Fatal("SelectScope stayed available outside the page")
	}
}

func TestSelectScopeGrowsThroughAContainer(t *testing.T) {
	p := open(t)
	p.typeText("> a")
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(vocab.SelectScope{})
	from, to := p.selection().Ordered(p.e.Snapshot())
	if from.Pos.Ref() != to.Pos.Ref() || from.Offset != 0 || to.Offset != 1 {
		t.Fatalf("first scope is %d..%d, want paragraph b", from.Offset, to.Offset)
	}
	p.do(vocab.SelectScope{})
	from, to = p.selection().Ordered(p.e.Snapshot())
	if from.Pos.Ref() == to.Pos.Ref() || from.Offset != 0 || to.Offset != 1 {
		t.Fatalf("the quote spans %d..%d, want both paragraphs", from.Offset, to.Offset)
	}
}

func TestDismissCollapsesASelectionAtItsHead(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	p.do(vocab.SelectScope{})
	head := p.selection().Head
	p.do(vocab.Dismiss{})
	sel := p.selection()
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() || sel.Head != head {
		t.Fatalf("dismiss made %+v, want the caret at %v", sel, head)
	}
}

func TestSelectScopeHasAGuide(t *testing.T) {
	p := open(t)
	p.typeText("text")
	guides := p.e.Decorations(1).Guides
	found := false
	for _, g := range guides {
		found = found || g.Command == "core.selectAll"
	}
	if !found {
		t.Fatalf("guides %+v have no Select All", guides)
	}
}

func TestWithoutThePolicySelectScopeIsUnavailable(t *testing.T) {
	prog, err := policy.Compile(policy.Manifest{Enabled: []*policy.Policy{core.Policy}, Strict: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := vocab.EmptyPage(1, 5, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	out, err := e.Preview(1, vocab.Input{Intent: vocab.SelectScope{}, At: time.Unix(10, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if out.Available {
		t.Fatal("core selected the page without the scopes policy")
	}
}
