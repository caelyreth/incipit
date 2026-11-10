package markdown

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/lists"
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
		Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, Policy, lists.Policy, navigation.Policy},
		Strict:  true,
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

func outline(n *doc.Node) string {
	var b strings.Builder
	var walk func(*doc.Node)
	walk = func(n *doc.Node) {
		b.WriteString(vocab.Schema().KindName(n.Kind()))
		b.WriteByte('(')
		for i, s := range n.Slots() {
			if i > 0 {
				b.WriteByte('|')
			}
			if s.IsInline() {
				s.Content().Each(func(_ int, it doc.Item) bool {
					if it.Node != nil {
						walk(it.Node)
					} else {
						b.WriteString(it.Text)
					}
					return true
				})
				continue
			}
			for j, c := range s.Children() {
				if j > 0 {
					b.WriteByte(' ')
				}
				walk(c)
			}
		}
		b.WriteByte(')')
	}
	walk(n)
	return b.String()
}

func (p *page) want(s string) {
	p.t.Helper()
	if got := outline(p.e.Snapshot().Root()); got != s {
		p.t.Fatalf("got  %s\nwant %s", got, s)
	}
}

func TestTypedListJoinsTheListBeforeIt(t *testing.T) {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("- two")
	p.want("page(list(item(one|) item(two|)))")
}

func TestTypedListJoinsTheListAfterIt(t *testing.T) {
	p := open(t)
	p.typeText("one")
	p.do(vocab.Break{})
	p.typeText("- two")
	p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward})
	p.typeText("- ")
	p.want("page(list(item(one|) item(two|)))")
}

func TestTypedListKeepsADifferentStyleApart(t *testing.T) {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("1. two")
	p.want("page(list(item(one|)) list(item(two|)))")
}
