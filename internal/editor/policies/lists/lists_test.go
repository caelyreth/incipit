package lists

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/policies/core"
	"github.com/caelyreth/incipit/internal/editor/policies/markdown"
	"github.com/caelyreth/incipit/internal/editor/policies/navigation"
	"github.com/caelyreth/incipit/internal/editor/policies/notation"
	"github.com/caelyreth/incipit/internal/editor/policies/pairs"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func open(t *testing.T) *page {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, notation.Policy, pairs.Policy, markdown.Policy,
			Policy, navigation.Policy},
		Strict: true,
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

func outline(n *doc.Node) string {
	var b strings.Builder
	var walk func(*doc.Node)
	walk = func(n *doc.Node) {
		b.WriteString(profile.Schema().KindName(n.Kind()))
		b.WriteByte('(')
		for i, s := range n.Slots() {
			if i > 0 {
				b.WriteByte('|')
			}
			if s.IsInline() {
				s.Content().Each(func(_ int, it doc.Item) bool {
					switch {
					case it.Node != nil:
						walk(it.Node)
					case it.Marks != nil && !it.Marks.Empty():
						b.WriteString("<" + it.Text + ">")
					default:
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

func TestReshapeMakesSelectedParagraphsOneList(t *testing.T) {
	p := open(t)
	p.typeText("one")
	p.do(vocab.Break{})
	p.typeText("two")
	p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward, Extend: true})
	p.do(vocab.Command{ID: "lists.bullet"})
	p.want("page(list(item(one|) item(two|)))")
}

func TestReshapeJoinsAnAdjacentListOfTheSameStyle(t *testing.T) {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("two")
	p.do(vocab.Command{ID: "lists.bullet"})
	p.want("page(list(item(one|) item(two|)))")
}

func TestReshapeRetypesAnItemsList(t *testing.T) {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Command{ID: "lists.numbered"})
	list := p.e.Snapshot().Root().Slot(0).Child(0)
	if profile.ListStyleOf(list) != profile.Ordered {
		t.Fatalf("list style %v, want ordered", profile.ListStyleOf(list))
	}
}

func TestListCommandsAreInTheCatalog(t *testing.T) {
	p := open(t)
	for _, id := range []vocab.CommandID{"lists.bullet", "lists.numbered"} {
		if _, ok := p.e.Program().Command(id); !ok {
			t.Fatalf("program has no %s command", id)
		}
	}
}
