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

func (knows) Correction(string, int, int) string { return "" }

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

type corrector struct {
	knows
	fix map[string]string
}

func (c corrector) Correction(text string, from, to int) string {
	if from < 0 || to > len(text) || from >= to {
		return ""
	}
	return c.fix[text[from:to]]
}

func checkingCorrect(t *testing.T, on bool, fix map[string]string, words ...string) *page {
	spelling.Use(corrector{knows: knows(words), fix: fix})
	t.Cleanup(func() { spelling.Use(nil) })
	return openWith(t, policy.Values{spelling.ID + ".correct": on})
}

func (p *page) typeFast(s string) {
	p.t.Helper()
	for _, r := range s {
		p.now = p.now.Add(100 * time.Millisecond)
		if _, err := p.e.Do(1, vocab.Input{Intent: vocab.Type{Text: string(r)}, At: p.now}); err != nil {
			p.t.Fatal(err)
		}
	}
}

func TestAutocorrectIsOffByDefaultAndReplacesOnDelimiterWhenEnabled(t *testing.T) {
	p := checkingCorrect(t, false, map[string]string{"teh": "the"}, "teh")
	p.typeFast("teh ")
	p.want("page(paragraph(teh ))")
	if p.painted(profile.GroupCorrected) != 0 {
		t.Fatalf("did not expect corrected highlight when option is off")
	}

	p = checkingCorrect(t, true, map[string]string{"teh": "the", "wnet": "went"}, "teh", "wnet")
	p.typeFast("teh ")
	p.want("page(paragraph(the ))")
	if p.painted(profile.GroupCorrected) != 1 {
		t.Fatalf("expected 1 GroupCorrected paint after space, got %d", p.painted(profile.GroupCorrected))
	}
	p.typeFast("wnet.")
	p.want("page(paragraph(the went.))")
	if p.painted(profile.GroupCorrected) != 2 {
		t.Fatalf("expected 2 GroupCorrected paints after punctuation, got %d", p.painted(profile.GroupCorrected))
	}
}

func TestAutocorrectUndoRestoresOriginalBeforeUndoingTyping(t *testing.T) {
	p := checkingCorrect(t, true, map[string]string{"teh": "the"}, "teh")
	p.typeFast("teh ")
	p.want("page(paragraph(the ))")

	p.do(vocab.Undo{})
	p.want("page(paragraph(teh ))")

	p.do(vocab.Undo{})
	p.want("page(paragraph())")
}

func TestAutocorrectAnnotationExpiresAfterDeadlineAndCanRevertOnClick(t *testing.T) {
	p := checkingCorrect(t, true, map[string]string{"teh": "the"}, "teh")
	p.typeText("teh ")
	p.want("page(paragraph(the ))")
	if p.painted(profile.GroupCorrected) != 1 {
		t.Fatalf("expected GroupCorrected paint")
	}

	// Timer after 4s clears the temporary underline.
	p.now = p.now.Add(4*time.Second + time.Millisecond)
	if _, err := p.e.Do(1, vocab.Input{Intent: vocab.Timer{Deadline: p.now}, At: p.now}); err != nil {
		t.Fatal(err)
	}
	if p.painted(profile.GroupCorrected) != 0 {
		t.Fatalf("expected GroupCorrected to expire, still had %d", p.painted(profile.GroupCorrected))
	}

	// Clicking an active autocorrected word restores the original without re-correcting it.
	p = checkingCorrect(t, true, map[string]string{"teh": "the"}, "teh")
	p.typeText("teh ")
	p.want("page(paragraph(the ))")
	ref, _ := vocab.FirstTextSlot(p.e.Snapshot().Root())
	p.do(profile.RevertCorrection{At: doc.CaretAt(doc.At(ref, 1))})
	p.want("page(paragraph(teh ))")
	if p.painted(profile.GroupCorrected) != 0 {
		t.Fatalf("expected GroupCorrected dropped after revert, got %d", p.painted(profile.GroupCorrected))
	}
}

func TestAutocorrectSkipsNonProseAndIgnoredWords(t *testing.T) {
	p := checkingCorrect(t, true, map[string]string{"teh": "the"}, "teh")
	p.typeFast("#teh ")
	p.want("page(paragraph(#teh ))")

	p.typeFast("`a b`")
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.typeFast(" teh ")
	p.want("page(paragraph(#teh <a b teh >))")

	// Skip in inline formula ($...$).
	p = checkingCorrect(t, true, map[string]string{"teh": "the"}, "teh")
	p.typeFast("$teh $ ")
	p.want("page(paragraph(formula(teh ) ))")

	// Ignored word stays uncorrected too.
	p = checkingCorrect(t, true, map[string]string{"teh": "the"}, "teh")
	p.typeText("teh")
	p.do(profile.IgnoreSpelling{})
	p.typeText(" ")
	p.want("page(paragraph(teh ))")

	// Code block skips autocorrect.
	p = checkingCorrect(t, true, map[string]string{"teh": "the"}, "teh")
	p.typeText("```")
	p.do(vocab.Break{})
	p.typeText("teh ")
	p.want("page(code(teh ))")
}

func TestTypingWordTracksCurrentWordUntilDelimiterOrCaretLeaves(t *testing.T) {
	p := checking(t, "wnet", "teh")
	p.typeText("wn")
	if p.painted(profile.GroupTypingWord) != 1 {
		t.Fatalf("expected GroupTypingWord while typing word, got %d", p.painted(profile.GroupTypingWord))
	}
	p.typeText("et")
	if p.painted(profile.GroupTypingWord) != 1 {
		t.Fatalf("expected GroupTypingWord on word end, got %d", p.painted(profile.GroupTypingWord))
	}
	p.typeText(" ")
	if p.painted(profile.GroupTypingWord) != 0 {
		t.Fatalf("expected GroupTypingWord cleared on delimiter, got %d", p.painted(profile.GroupTypingWord))
	}

	p.typeText("teh")
	if p.painted(profile.GroupTypingWord) != 1 {
		t.Fatalf("expected GroupTypingWord while typing second word, got %d", p.painted(profile.GroupTypingWord))
	}
	p.do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Backward})
	if p.painted(profile.GroupTypingWord) != 0 {
		t.Fatalf("expected GroupTypingWord cleared when caret leaves word, got %d", p.painted(profile.GroupTypingWord))
	}
}
