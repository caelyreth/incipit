package notation

import (
	"slices"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func (p *page) block() *doc.Node {
	p.t.Helper()
	return p.e.Snapshot().Root().Slot(0).Child(0)
}

// ghosts are the spellings of the block's ghosts, in flow order.
func (p *page) ghosts() []string {
	p.t.Helper()
	pres := p.e.Present(1, []doc.ID{p.block().ID()})[0]
	var out []string
	for _, it := range pres.Surfaces[0].Flow.Items {
		if it.Kind == present.GhostItem {
			out = append(out, string(it.Spelling))
		}
	}
	return out
}

func (p *page) head() doc.Caret {
	p.t.Helper()
	v, _ := p.e.View(1)
	return v.Selection.Head
}

func TestACodeBlockSpellsItsFencesAndWritesItsInfoInTheOpeningOne(t *testing.T) {
	p := open(t)
	p.typeText("```")
	p.do(vocab.Break{})
	if b := p.block(); b.Kind() != profile.Code {
		t.Fatalf("kind %v, want code", b.Kind())
	}
	if got, want := p.ghosts(), []string{"```\n", "\n```"}; !slices.Equal(got, want) {
		t.Fatalf("ghosts %q, want %q", got, want)
	}
	p.typeText("x")
	// Back from the code's start is the fence line's end, where the
	// info string is written.
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.typeText("gp")
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.typeText("o`")
	if got := profile.CodeInfo(p.block()); got != "go" {
		t.Fatalf("info %q, want go", got)
	}
	if got, want := p.ghosts(), []string{"```go\n", "\n```"}; !slices.Equal(got, want) {
		t.Fatalf("ghosts %q, want %q", got, want)
	}
	// Return goes on into the code, which keeps its text.
	p.do(vocab.Break{})
	if h := p.head(); h.Pos.Offset != 0 || h.Stop != 0 {
		t.Fatalf("caret %+v, want the code's start", h)
	}
	p.typeText("y")
	if got := p.block().Slot(0).Content().Text(); got != "yx" {
		t.Fatalf("text %q, want yx", got)
	}
}

func TestTextTypedInTheClosingFenceEndsTheCode(t *testing.T) {
	p := open(t)
	p.typeText("```")
	p.do(vocab.Break{})
	p.typeText("a")
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.typeText("b")
	if got := p.block().Slot(0).Content().Text(); got != "ab" {
		t.Fatalf("text %q, want ab", got)
	}
	if h := p.head(); h.Pos.Offset != 2 || h.Stop != 0 {
		t.Fatalf("caret %+v, want after b", h)
	}
	// Deleting a backtick of the closing fence unfences the block.
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if b := p.block(); b.Kind() != profile.Paragraph {
		t.Fatalf("kind %v, want a paragraph", b.Kind())
	}
}

func TestTheClosingFenceFollowsTheLastLine(t *testing.T) {
	p := open(t)
	p.typeText("```")
	p.do(vocab.Break{})
	p.typeText("a")
	p.do(vocab.Break{})
	if got, want := p.ghosts(), []string{"```\n", "\n```"}; !slices.Equal(got, want) {
		t.Fatalf("ghosts %q, want %q", got, want)
	}
	p.typeText("b")
	pres := p.e.Present(1, []doc.ID{p.block().ID()})[0]
	items := pres.Surfaces[0].Flow.Items
	if last := items[len(items)-1]; last.Kind != present.GhostItem || last.From != 3 {
		t.Fatalf("last item %+v, want the closing fence after b", last)
	}
}
