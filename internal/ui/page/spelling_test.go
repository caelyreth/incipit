package page

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/manifest"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policies/spelling"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// misspells is a checker that knows every word but tdie.
type misspells struct{}

func (misspells) Check(text string, _ bool) []spelling.Issue {
	if i := strings.Index(text, "tdie"); i >= 0 {
		return []spelling.Issue{{From: i, To: i + 4}}
	}
	return nil
}

func (misspells) Correction(string, int, int) string { return "" }

// A secondary click on a misspelled word offers its corrections first,
// then learning and ignoring it; a correction writes itself over it.
func TestTheMenuOverAMisspelledWordOffersItsCorrections(t *testing.T) {
	spelling.Use(misspells{})
	t.Cleanup(func() { spelling.Use(nil) })
	el := newElement(t)
	el.Editor.Guesses = func(word string) []string {
		if word != "tdie" {
			t.Errorf("guesses asked for %q", word)
		}
		return []string{"tide", "tie"}
	}
	typeText(el, "Low tdie turns")
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	tester.Frame()
	ref, _ := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	at, size, _ := el.lookUpPlace(profile.LookUpAsk{Slot: ref, From: 4, To: 8})
	tester.RightClickAt(at.X+size/2, at.Y-size/3)
	tester.Advance(100 * time.Millisecond)
	items := tester.Menu()
	learn := slices.Index(items, "Learn Spelling")
	if len(items) < 2 || items[0] != "tide" || items[1] != "tie" || learn < 0 || !slices.Contains(items, "Ignore Spelling") {
		t.Fatalf("the menu offers %q", items)
	}
	if err := tester.ChooseMenuItem("tide"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(100 * time.Millisecond)
	n, _ := el.Editor.Engine.Snapshot().Node(ref.Node)
	if got := n.Slot(ref.Slot).Content().Flat(); got != "Low tide turns" {
		t.Fatalf("corrected to %q", got)
	}
	if head := el.Editor.Selection().Head.Pos.Offset; head != 8 {
		t.Fatalf("the caret stands at %d, want after tide", head)
	}
}

type autocorrects map[string]string

func (a autocorrects) Check(text string, _ bool) []spelling.Issue {
	var out []spelling.Issue
	for w := range a {
		for at := 0; ; {
			i := strings.Index(text[at:], w)
			if i < 0 {
				break
			}
			out = append(out, spelling.Issue{From: at + i, To: at + i + len(w)})
			at += i + len(w)
		}
	}
	slices.SortFunc(out, func(x, y spelling.Issue) int { return x.From - y.From })
	return out
}

func (a autocorrects) Correction(text string, from, to int) string {
	if from < 0 || to > len(text) || from >= to {
		return ""
	}
	return a[text[from:to]]
}

func hasSeenDeco(el *Element) bool {
	for k, sd := range el.decos {
		if !k.highlight && sd.seen {
			return true
		}
	}
	return false
}

func TestMisspelledWaveWaitsUntilCaretLeavesTypedWord(t *testing.T) {
	spelling.Use(misspells{})
	t.Cleanup(func() { spelling.Use(nil) })
	el := newElement(t)
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)

	typeText(el, "ok tdie")
	tester.Frame()
	if hasSeenDeco(el) {
		t.Fatalf("expected misspelled wave suppressed while still typing the word")
	}

	typeText(el, " ")
	tester.Frame()
	if !hasSeenDeco(el) {
		t.Fatalf("expected misspelled wave once delimiter ended the word")
	}
}

func TestClickingAutocorrectedWordRestoresOriginal(t *testing.T) {
	spelling.Use(autocorrects{"tdie": "tide"})
	t.Cleanup(func() { spelling.Use(nil) })
	prog, err := manifest.Compile(policy.Values{spelling.ID + ".correct": true}, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	el := NewElement(NewEditor(engine.New(snap, prog, engine.Options{}), 1))
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	tester.Frame()

	now := tester.Now()
	for _, r := range "Low tdie " {
		el.Editor.Do(vocab.Type{Text: string(r)}, now)
	}
	el.relayout()
	tester.Frame()
	ref, _ := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	n, _ := el.Editor.Engine.Snapshot().Node(ref.Node)
	if got := n.Slot(ref.Slot).Content().Flat(); got != "Low tide " {
		t.Fatalf("autocorrected to %q, want %q", got, "Low tide ")
	}

	at, size, ok := el.lookUpPlace(profile.LookUpAsk{Slot: ref, From: 4, To: 8})
	if !ok {
		t.Fatal("missing corrected word box")
	}
	px, py := el.toPage(at.X+size/2, at.Y-size/3)
	if cur, has := el.cursorAt(px, py); !has || cur != ui.CursorPointer {
		t.Fatalf("cursor = (%v, %v), want pointer", cur, has)
	}
	tester.ClickAt(at.X+size/2, at.Y-size/3)
	tester.Advance(100 * time.Millisecond)

	n, _ = el.Editor.Engine.Snapshot().Node(ref.Node)
	if got := n.Slot(ref.Slot).Content().Flat(); got != "Low tdie " {
		t.Fatalf("after click revert = %q, want %q", got, "Low tdie ")
	}
}
