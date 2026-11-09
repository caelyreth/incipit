package vocab

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
)

func page(t *testing.T, blocks ...*doc.Node) *doc.Snapshot {
	t.Helper()
	snap, err := doc.NewSnapshot(Schema(), doc.NewInterner(), doc.NewNodeWithID(1, Page, doc.Attrs{}, "", doc.BlockSlot()), 3)
	if err != nil {
		t.Fatal(err)
	}
	return edit(t, snap, Ops(doc.Insert{Parent: 1, Nodes: blocks}))
}

func edit(t *testing.T, s *doc.Snapshot, e Edit) *doc.Snapshot {
	t.Helper()
	tx := s.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := e(tx); err != nil {
		t.Fatal(err)
	}
	out, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func para(text string) *doc.Node { return NewParagraph(doc.TextItem(text, nil)) }

// outline renders the tree as kinds and text.
func outline(n *doc.Node) string {
	var b strings.Builder
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		spec, _ := Schema().Kind(n.Kind())
		b.WriteString(spec.Name)
		b.WriteByte('(')
		for i, s := range n.Slots() {
			if i > 0 {
				b.WriteByte('|')
			}
			if s.IsInline() {
				b.WriteString(s.Content().Text())
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

func nth(s doc.Reader, i int) doc.ID { return s.Root().Slot(0).Child(i).ID() }

func TestSplitAndJoin(t *testing.T) {
	s := page(t, para("hello world"))
	id := nth(s, 0)
	split := edit(t, s, Split(doc.Pos{Node: id, Offset: 5}, Paragraph, doc.Attrs{}))
	if got := outline(split.Root()); got != "page(paragraph(hello) paragraph( world))" {
		t.Fatal(got)
	}
	joined := edit(t, split, Join(doc.SlotRef{Node: id}, nth(split, 1)))
	if got := outline(joined.Root()); got != "page(paragraph(hello world))" {
		t.Fatal(got)
	}
}

func TestDeleteRangeAcrossContainers(t *testing.T) {
	quote := NewBlock(Quote, doc.Attrs{})
	s := page(t, para("one"), quote, para("four"))
	s = edit(t, s, Ops(doc.Insert{Parent: nth(s, 1), Nodes: []*doc.Node{para("two"), para("three")}}))
	q, _ := s.Node(nth(s, 1))
	three := q.Slot(0).Child(1).ID()
	out := edit(t, s, DeleteRange(doc.Pos{Node: nth(s, 0), Offset: 2}, doc.Pos{Node: three, Offset: 3}))
	// "on" + "ee": the quote's last paragraph joins the first, and the
	// emptied quote is removed.
	if got := outline(out.Root()); got != "page(paragraph(onee) paragraph(four))" {
		t.Fatal(got)
	}
}

func TestWrapLiftAndSink(t *testing.T) {
	s := page(t, para("a"), para("b"))
	wrapped := edit(t, s, Wrap([]doc.ID{nth(s, 0), nth(s, 1)}, Quote, doc.Attrs{}))
	if got := outline(wrapped.Root()); got != "page(quote(paragraph(a) paragraph(b)))" {
		t.Fatal(got)
	}
	lifted := edit(t, wrapped, Lift(nth(s, 0)))
	if got := outline(lifted.Root()); got != "page(paragraph(a) quote(paragraph(b)))" {
		t.Fatal(got)
	}
	list := NewBlock(List, doc.Attrs{})
	l := page(t, list)
	items := []*doc.Node{NewBlock(Item, doc.Attrs{}), NewBlock(Item, doc.Attrs{})}
	l = edit(t, l, Ops(doc.Insert{Parent: nth(l, 0), Nodes: items}))
	ln, _ := l.Node(nth(l, 0))
	sunk := edit(t, l, Sink(ln.Slot(0).Child(1).ID()))
	if got := outline(sunk.Root()); got != "page(list(item(|list(item(|)))))" {
		t.Fatal(got)
	}
}

func TestPasteBlocksJoinsOpenEdges(t *testing.T) {
	src := page(t, para("alpha"), para("beta"))
	slice := Copy(src, doc.Pos{Node: nth(src, 0), Offset: 2}, doc.Pos{Node: nth(src, 1), Offset: 2})
	dst := page(t, para("[]"))
	out := edit(t, dst, InsertSlice(doc.Pos{Node: nth(dst, 0), Offset: 1}, slice))
	if got := outline(out.Root()); got != "page(paragraph([pha) paragraph(be]))" {
		t.Fatal(got)
	}
	code := page(t, NewBlock(Code, doc.Attrs{}))
	out = edit(t, code, InsertSlice(doc.Pos{Node: nth(code, 0)}, slice))
	if got := outline(out.Root()); got != "page(code(pha\nbe))" {
		t.Fatal(got)
	}
}

func TestEmptyPageIsDeterministic(t *testing.T) {
	a, err := EmptyPage(1, 42, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := EmptyPage(1, 42, doc.NewInterner())
	if nth(a, 0) != nth(b, 0) {
		t.Fatal("the same seed gave different ids")
	}
}

func TestRetypeFitsTextToTheNewSlot(t *testing.T) {
	s := page(t, NewBlock(Code, doc.Attrs{}))
	id := nth(s, 0)
	s = edit(t, s, Ops(doc.EditText{At: doc.SlotRef{Node: id}, Items: []doc.Item{doc.TextItem("one\ntwo", nil)}}))
	// A heading keeps no lines, and its level is stored only off its
	// default.
	h := edit(t, s, Retype(id, Heading, doc.Attrs{}.With(FieldLevel, doc.Int(1))))
	n, _ := h.Node(id)
	if got := outline(h.Root()); got != "page(heading(one two))" {
		t.Fatal(got)
	}
	if n.Attrs().Len() != 0 {
		t.Fatalf("the default level is stored: %v", n.Attrs())
	}
	if err := h.Validate(); err != nil {
		t.Fatal(err)
	}
	// Into code the marks drop.
	strong := s.Marks().Empty().With(doc.Mark{Kind: Strong})
	m := page(t, NewParagraph(doc.TextItem("a", strong), doc.TextItem(" b", nil)))
	c := edit(t, m, Retype(nth(m, 0), Code, doc.Attrs{}))
	cn, _ := c.Node(nth(m, 0))
	if got := outline(c.Root()); got != "page(code(a b))" {
		t.Fatal(got)
	}
	if ms := cn.Slot(0).Content().MarksAt(1); ms != nil && ms.Has(Strong) {
		t.Fatal("code keeps a mark")
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestWrapRunLiftsToWhatTheContainerHolds(t *testing.T) {
	list := NewBlock(List, doc.Attrs{})
	s := page(t, para("a"), list, para("z"))
	items := []*doc.Node{NewBlock(Item, doc.Attrs{}), NewBlock(Item, doc.Attrs{})}
	s = edit(t, s, Ops(doc.Insert{Parent: nth(s, 1), Nodes: items}))
	ln, _ := s.Node(nth(s, 1))
	item := ln.Slot(0).Child(1).ID()
	// An item lifts to its list.
	run, ok := WrapRun(s, []doc.ID{item}, Quote)
	if !ok || len(run) != 1 || run[0] != nth(s, 1) {
		t.Fatalf("run %v %v", run, ok)
	}
	// From the paragraph before to the item, the run spans both siblings.
	run, ok = WrapRun(s, []doc.ID{nth(s, 0), item}, Quote)
	if !ok || len(run) != 2 || run[0] != nth(s, 0) || run[1] != nth(s, 1) {
		t.Fatalf("run %v %v", run, ok)
	}
	out := edit(t, s, Wrap(run, Quote, doc.Attrs{}))
	if got := outline(out.Root()); got != "page(quote(paragraph(a) list(item(|) item(|))) paragraph(z))" {
		t.Fatal(got)
	}
}

func TestMarksThatExcludeEachOtherNeverShareASet(t *testing.T) {
	s := page(t, para("x"))
	set := s.Marks().Empty().With(doc.Mark{Kind: Strong}).With(doc.Mark{Kind: Emphasis})
	code := WithMark(set, doc.Mark{Kind: CodeMark})
	if code.Has(Strong) || code.Has(Emphasis) || !code.Has(CodeMark) {
		t.Fatal("code keeps the marks it excludes")
	}
	// The exclusion holds both ways: the newest mark wins.
	strong := WithMark(code, doc.Mark{Kind: Strong})
	if strong.Has(CodeMark) || !strong.Has(Strong) {
		t.Fatal("strong joins code")
	}
	if !Excludes(Strong, CodeMark) || Excludes(Strong, Emphasis) {
		t.Fatal("exclusion is not symmetric")
	}
}

func item(text string, body ...*doc.Node) *doc.Node {
	return doc.NewNode(Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem(text, nil)), doc.BlockSlot(body...))
}

func list(items ...*doc.Node) *doc.Node {
	return doc.NewNode(List, doc.Attrs{}, doc.BlockSlot(items...))
}

// inOrder is the n-th node in tree order, counting the page as 0.
func inOrder(s *doc.Snapshot, n int) *doc.Node {
	var out *doc.Node
	i := 0
	var walk func(x *doc.Node)
	walk = func(x *doc.Node) {
		if i == n {
			out = x
		}
		i++
		for _, sl := range x.Slots() {
			if !sl.IsInline() {
				for _, c := range sl.Children() {
					walk(c)
				}
			}
		}
	}
	walk(s.Root())
	return out
}

func TestUnlistSplitsTheListAroundTheItem(t *testing.T) {
	s := page(t, list(item("a"), item("b", para("in")), item("c")))
	s = edit(t, s, Unlist(inOrder(s, 3).ID()))
	if got, want := outline(s.Root()), "page(list(item(a|)) paragraph(b) paragraph(in) list(item(c|)))"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestOutdentFollowsTheHoldingItem(t *testing.T) {
	s := page(t, list(item("a", list(item("b"), item("c")))))
	s = edit(t, s, Outdent(inOrder(s, 4).ID()))
	if got, want := outline(s.Root()), "page(list(item(a|) item(b|list(item(c|)))))"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestARangeFromAGapUnmarksTheItemItEndsIn(t *testing.T) {
	s := page(t, list(item("a"), item("bc"), item("d", list(item("e")))))
	l := inOrder(s, 1)
	b := inOrder(s, 3)
	s = edit(t, s, DeleteRange(doc.Pos{Node: l.ID(), Slot: 0, Offset: 0}, doc.At(doc.SlotRef{Node: b.ID()}, 1)))
	if got, want := outline(s.Root()), "page(paragraph(c) list(item(d|list(item(e|)))))"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestARangeIntoANestedItemTakesTheItemsItOpens(t *testing.T) {
	s := page(t, para("xy"), list(item("a"), item("d", list(item("ef"), item("g")))))
	x := inOrder(s, 1)
	e := inOrder(s, 6)
	s = edit(t, s, DeleteRange(doc.At(doc.SlotRef{Node: x.ID()}, 1), doc.At(doc.SlotRef{Node: e.ID()}, 1)))
	if got, want := outline(s.Root()), "page(paragraph(xf) list(item(g|)))"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	s = page(t, list(item("a"), item("d", list(item("ef")))))
	l := inOrder(s, 1)
	e = inOrder(s, 5)
	s = edit(t, s, DeleteRange(doc.Pos{Node: l.ID()}, doc.At(doc.SlotRef{Node: e.ID()}, 1)))
	if got, want := outline(s.Root()), "page(paragraph(f))"; got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
