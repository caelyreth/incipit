package statistics

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
)

func paragraph(text string) *doc.Node {
	return profile.NewParagraph(doc.TextItem(text, nil))
}

func testPage() *doc.Node {
	return doc.NewNode(profile.Page, doc.Attrs{},
		doc.BlockSlot(paragraph("Hello, world"), paragraph("世界")))
}

func TestCountsCountWordsCharactersAndCJK(t *testing.T) {
	got := CountsOf(testPage())
	if got.Words != 3 || got.Characters != 14 || got.CJK != 2 {
		t.Fatalf("counts = %+v", got)
	}
	if got := CountText("hello 世界\nnext"); got != (Counts{Words: 3, Characters: 12, CJK: 2}) {
		t.Fatalf("selected text counts = %+v", got)
	}
}

func TestThePolicySummaryMatchesADirectCount(t *testing.T) {
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile, Enabled: []*policy.Policy{core.Policy, Policy}, Strict: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = prog
	store := policy.NewStore()
	root := testPage()
	if Summary == nil {
		t.Fatal("the policy did not declare its page summary")
	}
	if got, want := policy.SummaryOf(store, Summary, root), CountsOf(root); got != want {
		t.Fatalf("summary = %+v, direct = %+v", got, want)
	}
	if got := Page(store, root); got != CountsOf(root) {
		t.Fatalf("page = %+v", got)
	}
}
