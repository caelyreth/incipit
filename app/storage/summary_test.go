package storage

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
)

func para(text string) *doc.Node { return profile.NewParagraph(doc.TextItem(text, nil)) }

func page(blocks ...*doc.Node) *doc.Node {
	return doc.NewNode(profile.Page, doc.Attrs{}, doc.BlockSlot(blocks...))
}

func TestASummaryTitlesThePageByItsFirstHeading(t *testing.T) {
	h := doc.NewNode(profile.Heading, profile.Schema().Canonical(profile.Heading, doc.Attrs{}), doc.InlineSlot(doc.TextItem("  A   title ", nil)))
	item := doc.NewNode(profile.Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x", nil)), doc.BlockSlot(h))
	list := doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(item))
	s := Summarize(page(para("intro words"), list))
	if !s.Equal(Summary{Title: "A title", Excerpt: "intro words", Words: 5}) {
		t.Fatalf("summary %+v", s)
	}
	if s := Summarize(page(para("none"))); s.Title != "" || s.Excerpt != "none" {
		t.Fatalf("summary %+v without a heading", s)
	}
}

func TestASummaryCountsTheTagsThePageWrites(t *testing.T) {
	code := doc.NewNode(profile.Code, doc.Attrs{}, doc.InlineSlot(doc.TextItem("#include", nil)))
	s := Summarize(page(para("#Moon tides #ocean"), code, para("#ocean and #moon, #ocean")))
	want := []Tag{{Name: "ocean", Count: 3}, {Name: "Moon", Count: 2}}
	if !slices.Equal(s.Tags, want) {
		t.Fatalf("tags %+v, want %+v", s.Tags, want)
	}
}
