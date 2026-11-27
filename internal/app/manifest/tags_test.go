package manifest

import (
	"slices"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// tagged is the text of the caret's unit that reads as tags, a piece per
// tag.
func (p *page) tagged() []string {
	p.t.Helper()
	v, _ := p.e.View(1)
	pres := p.e.Present(1, []doc.ID{v.Selection.Head.Pos.Node})[0]
	var out []string
	last := -1
	for _, it := range pres.Surfaces[0].Flow.Items {
		if it.Kind != present.TextRun || !slices.Contains(it.Groups, profile.GroupTag) {
			continue
		}
		if it.From == last {
			out[len(out)-1] += it.Text
		} else {
			out = append(out, it.Text)
		}
		last = it.To
	}
	return out
}

func TestATagReadsAsOneWhereverTheCaretIs(t *testing.T) {
	p := open(t)
	p.typeText("Tides #ocean-life, C# and #1 (#moon)")
	want := []string{"#ocean-life", "#moon"}
	if got := p.tagged(); !slices.Equal(got, want) {
		t.Fatalf("tags %q, want %q", got, want)
	}
	p.do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Backward})
	if got := p.tagged(); !slices.Equal(got, want) {
		t.Fatalf("with the caret moved, tags %q, want %q", got, want)
	}
}

func TestHashesAndASpaceStillMakeAHeading(t *testing.T) {
	p := open(t)
	p.typeText("## Title")
	p.want("page(heading(Title))")

	p = open(t)
	p.typeText("#idea first")
	p.want("page(paragraph(#idea first))")
	if got := p.tagged(); !slices.Equal(got, []string{"#idea"}) {
		t.Fatalf("tags %q", got)
	}
}

func TestPickingATagNamesItAndEndsItsWord(t *testing.T) {
	p := open(t)
	p.typeText("see #oc")
	p.do(profile.PickTag{Name: "ocean"})
	p.typeText("now")
	p.want("page(paragraph(see #ocean now))")

	// A space already after the tag ends its word: the caret goes past it.
	p = open(t)
	p.typeText("#oc tail")
	p.do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Backward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.do(profile.PickTag{Name: "ocean"})
	p.typeText("x")
	p.want("page(paragraph(#ocean xtail))")
}

func TestATagIsNotWrittenInCode(t *testing.T) {
	p := open(t)
	p.typeText("run `#main` now")
	if got := p.tagged(); len(got) != 0 {
		t.Fatalf("tags %q in code", got)
	}
}
