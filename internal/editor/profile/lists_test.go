package profile

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
)

// TestIntoListJoinsAChainOfLists turns the block after two adjacent lists
// of one style into an item: it joins the list before it, which then
// joins the one before that, moving items out of a list the edit already
// changed (the crash of 2026-11-23), and the one after.
func TestIntoListJoinsAChainOfLists(t *testing.T) {
	item := func(text string) *doc.Node {
		return doc.NewNode(Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem(text, nil)), doc.BlockSlot())
	}
	list := func(items ...*doc.Node) *doc.Node { return doc.NewNode(List, doc.Attrs{}, doc.BlockSlot(items...)) }
	s := page(t, list(item("a")), list(item("b"), item("c")), para("H"), list(item("d")))
	root, _ := s.Node(1)
	s = edit(t, s, IntoList(root.Slot(0).Child(2).ID(), doc.Attrs{}))
	root, _ = s.Node(1)
	got := outline(root)
	if want := "page(list(item(a|) item(b|) item(c|) item(H|) item(d|)))"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
