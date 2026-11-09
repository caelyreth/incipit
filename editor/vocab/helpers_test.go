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
