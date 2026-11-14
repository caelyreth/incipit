package search

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/internal/query"
)

func TestTheFirstMatchIsFoundInReadingOrder(t *testing.T) {
	text := func(s string) doc.Item { return doc.TextItem(s, nil) }
	heading := doc.NewNode(profile.Heading, profile.Schema().Canonical(profile.Heading, doc.Attrs{}), doc.InlineSlot(text("Plans")))
	para := profile.NewParagraph(text("Café budget"))
	root := doc.NewNode(profile.Page, doc.Attrs{}, doc.BlockSlot(heading, para))
	spot, ok := FirstMatch(root, query.NewMatcher(query.Parse("budget"), false))
	if !ok || spot.Slot != (doc.SlotRef{Node: para.ID(), Slot: 0}) || spot.Span != (query.Span{From: 6, To: 12}) {
		t.Fatalf("%v %v", spot, ok)
	}
	if _, ok := FirstMatch(root, query.NewMatcher(query.Parse("missing"), false)); ok {
		t.Fatal("found what is not there")
	}
}
