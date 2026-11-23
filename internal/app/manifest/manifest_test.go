package manifest

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policies/notation"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
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
	if r := prog.Report(); len(r.Dropped)+len(r.Shadowed) > 0 {
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

// Enter on the empty last line of a code or math block leaves it; Shift+
// Enter still adds the line, and Enter elsewhere breaks the line.
func TestEnterOnAnEmptyLastLineLeavesTheBlock(t *testing.T) {
	for _, c := range []struct{ open, want string }{
		{"```go", "page(code(x\ny) paragraph(z))"},
		{"$$", "page(math(x\ny) paragraph(z))"},
	} {
		p := open(t)
		p.typeText(c.open)
		if c.open == "```go" {
			p.do(vocab.Break{})
		}
		p.typeText("x")
		p.do(vocab.Break{})
		p.typeText("y")
		p.do(vocab.Break{})
		p.do(vocab.Break{})
		p.typeText("z")
		p.want(c.want)
	}
	p := open(t)
	p.typeText("```go")
	p.do(vocab.Break{})
	p.typeText("x")
	p.do(vocab.SoftBreak{})
	p.do(vocab.SoftBreak{})
	p.typeText("y")
	p.want("page(code(x\n\ny))")
}

// An empty block leaves the container holding it, as an empty item
// leaves its list: at its end, in its middle, splitting it, and one
// level of nesting at a time.
func TestEnterInAnEmptyBlockLeavesItsContainer(t *testing.T) {
	p := open(t)
	p.typeText("> a")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("z")
	p.want("page(quote(paragraph(a)) paragraph(z))")

	p = open(t)
	p.typeText("> a")
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Child(0).ID()}, 1))})
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.want("page(quote(paragraph(a)) paragraph() quote(paragraph(b)))")

	p = open(t)
	p.typeText("> > a")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.want("page(quote(quote(paragraph(a)) paragraph()))")
	p.do(vocab.Break{})
	p.want("page(quote(quote(paragraph(a))) paragraph())")
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

// Every product rule can run: none answers only where an earlier one
// already does.
func TestNoRuleIsShadowed(t *testing.T) {
	prog, err := Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if s := prog.Report().Shadowed; len(s) > 0 {
		t.Fatalf("shadowed rules: %v", s)
	}
}

// A formula's spelling opens and closes it, as a pair's closer closes the
// pair: navigation steps over every Closer alike. In an empty formula the
// "$" is the formulas policy's, not a close.
func TestFormulaSpellingOpensAndCloses(t *testing.T) {
	p := open(t)
	p.typeText("a$x$b")
	p.want("page(paragraph(aformula(x)b))")

	p = open(t)
	p.typeText("$$")
	p.want("page(math())")

	p = open(t)
	p.typeText("(x)")
	p.want("page(paragraph((x)))")
}

// A rewrite makes only the changes typing caused: a "---" already on the
// line stays as it stands when a letter is typed before it, while dashes
// typed next to a word still become one.
func TestTypingRewritesOnlyWhatItCauses(t *testing.T) {
	p := open(t)
	p.typeText("---")
	p.want("page(paragraph(---))")
	para := p.e.Snapshot().Root().Slot(0).Child(0).ID()
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: para}, 0))})
	p.typeText("x")
	p.want("page(paragraph(x---))")

	p = open(t)
	p.typeText("a--b")
	p.want("page(paragraph(a—b))")
}

// In source text, nesting is the lines' indentation: Tab indents the
// lines a selection touches, Shift+Tab takes it back, and with none to
// take, the container's outdent comes next.
func TestTabIndentsCode(t *testing.T) {
	nest, outdent := vocab.Command{ID: "lists.nest"}, vocab.Command{ID: "lists.outdent"}
	p := open(t)
	p.typeText("```go")
	p.do(vocab.Break{})
	p.typeText("x")
	p.do(vocab.SoftBreak{})
	p.typeText("y")
	p.do(nest)
	p.want("page(code(x\n    y))")
	p.typeText("z")
	p.want("page(code(x\n    yz))")
	p.do(vocab.SelectScope{})
	p.do(nest)
	p.want("page(code(    x\n        yz))")
	p.do(outdent)
	p.do(outdent)
	p.want("page(code(x\nyz))")
	p.do(outdent)
	p.want("page(code(x\nyz))")
}

// Shift+Enter is a new line in the same block wherever text keeps lines,
// rich text as source text; Enter splits rich text. A heading, which
// Markdown cannot write over two lines, keeps none: there Shift+Enter
// splits as Enter does.
func TestShiftEnterAddsALineInTheBlock(t *testing.T) {
	for _, tc := range []struct{ typed, want string }{
		{"x", "page(paragraph(x\ny))"},
		{"- x", "page(list(item(x\ny|)))"},
		{"# x", "page(heading(x) paragraph(y))"},
	} {
		p := open(t)
		p.typeText(tc.typed)
		p.do(vocab.SoftBreak{})
		p.typeText("y")
		p.want(tc.want)
	}
	p := open(t)
	p.typeText("x")
	p.do(vocab.SoftBreak{})
	p.do(vocab.Break{})
	p.typeText("y")
	p.want("page(paragraph(x\n) paragraph(y))")
}
