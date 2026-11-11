package manifest

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/notation"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

type page struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func open(t *testing.T) *page { return openWith(t, nil) }

// openWith opens a page under settings values.
func openWith(t *testing.T, values policy.Values) *page {
	prog, err := Compile(values, true)
	if err != nil {
		t.Fatal(err)
	}
	if r := prog.Report(); len(r.Dropped) > 0 {
		t.Logf("report:\n%s", r)
	}
	snap, _ := profile.EmptyPage(1, 5, doc.NewInterner())
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
	var walk func(n *doc.Node)
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

func TestTypedMarkdownBecomesStructure(t *testing.T) {
	cases := []struct{ typed, want string }{
		{"## Title", "page(heading(Title))"},
		{"> quoted", "page(quote(paragraph(quoted)))"},
		{"- item", "page(list(item(item|)))"},
		{"a *b* c", "page(paragraph(a <b> c))"},
		{"a **b** c", "page(paragraph(a <b> c))"},
		{"x `y` z", "page(paragraph(x <y> z))"},
		{"p $x$ q", "page(paragraph(p formula(x) q))"},
	}
	for _, c := range cases {
		p := open(t)
		p.typeText(c.typed)
		p.want(c.want)
	}
}

func TestFencesOnEnter(t *testing.T) {
	p := open(t)
	p.typeText("```go")
	p.do(vocab.Break{})
	p.typeText("x")
	p.want("page(code(x))")
	p = open(t)
	p.typeText("---")
	p.do(vocab.Break{})
	p.typeText("y")
	p.want("page(divider() paragraph(y))")
}

func TestLeaveAndReturnFromFormula(t *testing.T) {
	p := open(t)
	p.typeText("a $x")
	p.do(vocab.Leave{})
	p.typeText("b")
	p.want("page(paragraph(a formula(x)b))")
	p = open(t)
	p.typeText("a $x")
	p.do(vocab.Leave{})
	p.do(vocab.Return{})
	p.typeText("y")
	p.want("page(paragraph(a formula(xy)))")
}

// reveals are notation's reveal modes, to run a test under each.
var reveals = []string{notation.RevealShown, notation.RevealHidden}

// revealed opens a page whose notation reveals as reveal says.
func revealed(t *testing.T, reveal string) *page {
	return openWith(t, policy.Values{notation.ID + ".reveal": reveal})
}
