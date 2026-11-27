package scopes

import (
	"github.com/caelyreth/incipit/internal/editor/present"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/policies/core"
	"github.com/caelyreth/incipit/internal/editor/policies/guides"
	"github.com/caelyreth/incipit/internal/editor/policies/lists"
	"github.com/caelyreth/incipit/internal/editor/policies/markdown"
	"github.com/caelyreth/incipit/internal/editor/policies/navigation"
	"github.com/caelyreth/incipit/internal/editor/policies/notation"
	"github.com/caelyreth/incipit/internal/editor/policies/pairs"
	"github.com/caelyreth/incipit/internal/editor/policies/tables"
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
			lists.Policy, navigation.Policy, Policy, guides.Policy, tables.Policy},
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

func (p *page) selection() doc.Selection {
	p.t.Helper()
	v, ok := p.e.View(1)
	if !ok {
		p.t.Fatal("no view")
	}
	return v.Selection
}

func (p *page) painted(group vocab.Group) int {
	n := 0
	for _, h := range p.e.Decorations(1).Paints {
		if h.Group == group {
			n++
		}
	}
	return n
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

func TestSelectScopeReachesAMarkFillingItsParagraph(t *testing.T) {
	p := open(t)
	p.typeText("**bold**")
	for range 2 {
		p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	}
	p.do(vocab.SelectScope{})
	first := p.selection()
	p.do(vocab.SelectScope{})
	second := p.selection()
	if first.Anchor == second.Anchor && first.Head == second.Head {
		t.Fatalf("the mark and its paragraph have one selection %+v", first)
	}
	if first.Anchor.Offset != 0 || first.Head.Offset != 4 || second.Anchor.Offset != 0 || second.Head.Offset != 4 {
		t.Fatalf("selections %+v and %+v do not cover the mark and paragraph", first, second)
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

// Select All's key shows where it would select next once a scope is
// selected, over the wider scope it would take; at a caret, which it would
// turn into a range, it shows nowhere.
func TestSelectScopeHasAGuide(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	selectAll := func() (present.Guide, bool) {
		for _, g := range p.e.Decorations(1).Guides {
			if g.Command == "core.selectAll" {
				return g, true
			}
		}
		return present.Guide{}, false
	}
	if g, ok := selectAll(); ok {
		t.Fatalf("a caret shows Select All at %+v", g)
	}
	p.do(vocab.SelectScope{})
	g, ok := selectAll()
	if !ok || len(g.Covers) == 0 {
		t.Fatalf("a selected scope shows Select All as %+v, %v; want what it would select", g, ok)
	}
}

// The innermost textual scope is underlined while a collapsed caret is
// inside it, and the decoration follows the caret out.
func TestTheActiveScopeIsHighlighted(t *testing.T) {
	p := open(t)
	p.typeText("a *b")
	if p.painted(vocab.GroupScope) != 1 {
		t.Fatalf("scope highlights = %d", p.painted(vocab.GroupScope))
	}
	p.do(vocab.Leave{})
	if p.painted(vocab.GroupScope) != 0 {
		t.Fatalf("the scope stayed highlighted outside it: %d", p.painted(vocab.GroupScope))
	}
}

// A delimiter that only names a possible block prefix or an empty open
// run is not a textual scope yet.
func TestAnUnopenedDelimiterIsNotAScope(t *testing.T) {
	for _, input := range []string{"*", "**"} {
		p := open(t)
		p.typeText(input)
		if got := p.painted(vocab.GroupScope); got != 0 {
			t.Fatalf("%q painted %d scope highlights", input, got)
		}
	}
}

// Select All in a cell selects its text, then the table whole: the rows a
// table holds its cells in only arrange them, as its columns do, and are
// no scope, so a row is no step, however many the table has, and an empty
// cell steps to the table at once.
func TestSelectScopeInACellSelectsTheTableNext(t *testing.T) {
	for _, rows := range []int{1, 3} {
		for _, typed := range []string{"cell", ""} {
			p := open(t)
			p.typeText("above")
			p.do(vocab.Break{})
			p.do(profile.InsertTable{Rows: rows, Columns: 3})
			p.typeText(typed)
			if typed != "" {
				p.do(vocab.SelectScope{})
				if sel := p.selection(); sel.Kind != doc.SelectText || sel.IsCollapsed() {
					t.Fatalf("%d rows: the first Select All selects %+v, want the cell's text", rows, sel)
				}
			}
			p.do(vocab.SelectScope{})
			sel := p.selection()
			if sel.Kind != doc.SelectNode {
				t.Fatalf("%d rows, %q typed: Select All from the cell selects %+v, want the table", rows, typed, sel)
			}
			if n, ok := p.e.Snapshot().Node(sel.Node); !ok || n.Kind() != profile.Table {
				t.Fatalf("%d rows: Select All selects %+v, want the table", rows, sel)
			}
		}
	}
}

// A table's text is in its cells: Escape there selects the table, as it
// selects any object from its text, and again enters it.
func TestDismissInACellSelectsTheTable(t *testing.T) {
	p := open(t)
	p.typeText("above")
	p.do(vocab.Break{})
	p.do(profile.InsertTable{Rows: 2, Columns: 2})
	p.typeText("cell")
	p.do(vocab.Dismiss{})
	sel := p.selection()
	if n, ok := p.e.Snapshot().Node(sel.Node); sel.Kind != doc.SelectNode || !ok || n.Kind() != profile.Table {
		t.Fatalf("Escape in a cell selects %+v, want the table", sel)
	}
	p.do(vocab.Dismiss{})
	if sel := p.selection(); sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		t.Fatalf("Escape on the selected table selects %+v, want a caret in it", sel)
	}
}

// span is the selection's ordered extent, by offset, and whether it lies
// in one text slot.
func (p *page) span() (from, to int, one bool) {
	f, t := p.selection().Ordered(p.e.Snapshot())
	return f.Offset, t.Offset, f.Pos.Ref() == t.Pos.Ref()
}

func (p *page) wantSpan(step string, from, to int) {
	p.t.Helper()
	if f, t, _ := p.span(); f != from || t != to {
		p.t.Fatalf("%s: selection is %d..%d, want %d..%d", step, f, t, from, to)
	}
}

// Expand climbs from the word at the caret through its sentence to the
// block, and Shrink comes back down the same rungs to the caret.
func TestExpandClimbsTheLadderAndShrinkRetracesIt(t *testing.T) {
	p := open(t)
	p.typeText("Ab cd. Ef gh")
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	caret := p.selection()

	p.do(vocab.ExpandSelection{})
	p.wantSpan("word", 10, 12)
	p.do(vocab.ExpandSelection{})
	p.wantSpan("sentence", 7, 12)
	p.do(vocab.ExpandSelection{})
	p.wantSpan("block", 0, 12)

	p.do(vocab.ShrinkSelection{})
	p.wantSpan("back to the sentence", 7, 12)
	p.do(vocab.ShrinkSelection{})
	p.wantSpan("back to the word", 10, 12)
	p.do(vocab.ShrinkSelection{})
	if got := p.selection(); got.Anchor != caret.Anchor || got.Head != caret.Head {
		t.Fatalf("shrinking all the way made %+v, want the caret %+v", got, caret)
	}
	p.do(vocab.ShrinkSelection{})
	if got := p.selection(); got.Head != caret.Head || !got.IsCollapsed() {
		t.Fatalf("shrinking past the caret moved it to %+v", got)
	}
}

// A caret at a word's end takes that word; one in the space between words
// has none, and climbs to its sentence.
func TestExpandTakesTheWordEndingAtTheCaret(t *testing.T) {
	p := open(t)
	p.typeText("ab cd")
	p.do(vocab.ExpandSelection{})
	p.wantSpan("word before the caret", 3, 5)

	p = open(t)
	p.typeText("ab  cd")
	for range 3 {
		p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	}
	p.do(vocab.ExpandSelection{})
	p.wantSpan("no word at the space", 0, 6)
}

// Shrink only comes back from where a climb left the selection: once
// anything else selects, there is nothing to come back to.
func TestShrinkForgetsAClimbMovedAwayFrom(t *testing.T) {
	p := open(t)
	p.typeText("ab cd")
	p.do(vocab.ExpandSelection{})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true})
	before := p.selection()
	p.do(vocab.ShrinkSelection{})
	if got := p.selection(); got != before {
		t.Fatalf("shrink after moving made %+v, want %+v left alone", got, before)
	}
}

// Select All still grows from the block, past the word and the sentence,
// and Shrink comes back from it as well.
func TestSelectScopeSkipsTheTextRungsAndShrinksBack(t *testing.T) {
	p := open(t)
	p.typeText("Ab cd. Ef")
	caret := p.selection()
	p.do(vocab.SelectScope{})
	p.wantSpan("select all", 0, 9)
	p.do(vocab.ShrinkSelection{})
	if got := p.selection(); got.Head != caret.Head || !got.IsCollapsed() {
		t.Fatalf("shrink from select all made %+v, want the caret", got)
	}
}
