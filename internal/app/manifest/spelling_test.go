package manifest

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policies/spelling"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// knows is a checker that knows every word but its own.
type knows []string

func (k knows) Check(text string, _ bool) []spelling.Issue {
	var out []spelling.Issue
	for _, w := range k {
		for at := 0; ; {
			i := strings.Index(text[at:], w)
			if i < 0 {
				break
			}
			out = append(out, spelling.Issue{From: at + i, To: at + i + len(w)})
			at += i + len(w)
		}
	}
	slices.SortFunc(out, func(a, b spelling.Issue) int { return a.From - b.From })
	return out
}

// checking opens a page whose checker does not know words.
func checking(t *testing.T, words ...string) *page {
	spelling.Use(knows(words))
	t.Cleanup(func() { spelling.Use(nil) })
	return open(t)
}

// misspelled is the text of the caret's unit that waves, a piece per word.
func (p *page) misspelled() []string {
	p.t.Helper()
	v, _ := p.e.View(1)
	pres := p.e.Present(1, []doc.ID{v.Selection.Head.Pos.Node})[0]
	var out []string
	last := -1
	for _, it := range pres.Surfaces[0].Flow.Items {
		if it.Kind != present.TextRun || !slices.Contains(it.Groups, profile.GroupMisspelled) {
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

func TestAMisspelledWordWavesButNotInATag(t *testing.T) {
	p := checking(t, "wnet", "teh")
	p.typeText("I wnet to teh #teh shore")
	if got := p.misspelled(); !slices.Equal(got, []string{"wnet", "teh"}) {
		t.Fatalf("misspelled %q", got)
	}
}

func TestAnIgnoredWordStopsWavingAndALearnedOneAsksTheHost(t *testing.T) {
	p := checking(t, "wnet", "teh")
	p.typeText("wnet teh wnet")
	p.do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Backward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.do(profile.IgnoreSpelling{})
	if got := p.misspelled(); !slices.Equal(got, []string{"teh"}) {
		t.Fatalf("after ignoring, misspelled %q", got)
	}
	p.do(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward})
	p.do(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward})
	p.now = p.now.Add(time.Second)
	pub, err := p.e.Do(1, vocab.Input{Intent: profile.LearnSpelling{}, At: p.now})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(pub.Effects, policy.Effect{Kind: policy.EffectLearnWord, Data: profile.LearnWord{Word: "teh"}}) {
		t.Fatalf("effects %+v", pub.Effects)
	}
	if got := p.misspelled(); len(got) != 0 {
		t.Fatalf("after learning, misspelled %q", got)
	}
	p.want("page(paragraph(wnet teh wnet))")
}
