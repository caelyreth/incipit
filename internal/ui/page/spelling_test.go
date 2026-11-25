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

func TestInlinePredictionGhostTextAndTabAccept(t *testing.T) {
	spelling.Use(autocorrects{"uncomp": ""})
	t.Cleanup(func() { spelling.Use(nil) })
	prog, err := manifest.Compile(policy.Values{spelling.ID + ".predict": true}, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	ed := NewEditor(engine.New(snap, prog, engine.Options{}), 1)
	ed.predictDelay = 5 * time.Millisecond
	ed.Predict = func(text string, at int, cb func(string)) {
		go func() {
			if strings.HasSuffix(text[:at], "uncomp") {
				cb("romising")
			} else {
				cb("")
			}
		}()
	}
	el := NewElement(ed)
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	el.Focus()
	tester.Frame()

	for _, r := range "it is uncomp" {
		ed.Do(vocab.Type{Text: string(r)}, tester.Now())
	}
	deadline := time.Now().Add(2 * time.Second)
	for ed.deco.Suggestion == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tester.Advance(10 * time.Millisecond)
	}
	if ed.deco.Suggestion == nil || ed.deco.Suggestion.Text != "romising" {
		t.Fatalf("expected suggestion \"romising\", got %+v", ed.deco.Suggestion)
	}
	if got := ed.Selection().Head.Pos.Offset; got != len("it is uncomp") {
		t.Fatalf("caret offset moved to %d, want %d before ghost text", got, len("it is uncomp"))
	}
	if len(ed.layout.units) == 0 || len(ed.layout.units[0].surfaces) == 0 || !ed.layout.units[0].surfaces[0].hasSugg {
		t.Fatalf("expected surface layout to record active suggestion segment")
	}

	// Tab key accepts the inline prediction.
	if !el.handle(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyTab, Time: tester.Now()}, ui.Services{}) {
		t.Fatal("expected Tab to accept inline prediction")
	}
	tester.Frame()
	if ed.deco.Suggestion != nil {
		t.Fatalf("expected Suggestion cleared after Tab accept, got %+v", ed.deco.Suggestion)
	}
	ref, _ := vocab.FirstTextSlot(ed.Engine.Snapshot().Root())
	n, _ := ed.Engine.Snapshot().Node(ref.Node)
	if got := n.Slot(ref.Slot).Content().Flat(); got != "it is uncompromising" {
		t.Fatalf("document text after Tab = %q, want %q", got, "it is uncompromising")
	}
}

func TestInlinePredictionAnimatesInChangeAndOut(t *testing.T) {
	spelling.Use(autocorrects{"uncomp": ""})
	t.Cleanup(func() { spelling.Use(nil) })
	prog, err := manifest.Compile(policy.Values{spelling.ID + ".predict": true}, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	ed := NewEditor(engine.New(snap, prog, engine.Options{}), 1)
	el := NewElement(ed)
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	el.Focus()
	tester.Frame()

	for _, r := range "it is uncomp" {
		ed.Do(vocab.Type{Text: string(r)}, tester.Now())
	}
	pos := ed.Selection().Head.Pos

	// Fade in from alpha=0 -> 1 over suggEnterDuration without moving.
	ed.Do(profile.SetSuggestion{At: pos, Text: "romising"}, tester.Now())
	tester.Frame()
	if el.sugg.alpha != 0 {
		t.Fatalf("expected initial entrance alpha=0, got %v", el.sugg.alpha)
	}
	tester.Advance(suggEnterDuration / 2)
	if el.sugg.alpha <= 0 || el.sugg.alpha >= 1 {
		t.Fatalf("expected mid entrance alpha in (0, 1), got %v", el.sugg.alpha)
	}
	tester.Advance(suggEnterDuration)
	if el.sugg.alpha != 1 {
		t.Fatalf("expected settled entrance alpha=1, got %v", el.sugg.alpha)
	}

	// Prefix trim keeps full opacity with no leaving ghost.
	ed.Do(vocab.Type{Text: "r"}, tester.Now())
	tester.Frame()
	if el.sugg.alpha != 1 || el.sugg.leaving != nil {
		t.Fatalf("expected prefix trim to keep alpha=1 and leaving=nil, got alpha=%v leaving=%+v", el.sugg.alpha, el.sugg.leaving)
	}

	// Changing to a different suggestion cross-fades the old one out and fades the new one in.
	ed.Do(profile.SetSuggestion{At: ed.Selection().Head.Pos, Text: "essed"}, tester.Now())
	tester.Frame()
	if el.sugg.leaving == nil || el.sugg.alpha != 0 {
		t.Fatalf("expected cross-fade on change (leaving=%+v, alpha=%v)", el.sugg.leaving, el.sugg.alpha)
	}
	tester.Advance(suggEnterDuration + 20*time.Millisecond)
	if el.sugg.leaving != nil || el.sugg.alpha != 1 {
		t.Fatalf("expected settled cross-fade (leaving=%+v, alpha=%v)", el.sugg.leaving, el.sugg.alpha)
	}

	// Dismissing fades out cleanly without leaving the scope.
	el.handle(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape, Time: tester.Now()}, ui.Services{})
	tester.Frame()
	if el.sugg.leaving == nil || el.sugg.leaving.alpha <= 0 {
		t.Fatalf("expected fade-out leaving ghost on dismiss, got %+v", el.sugg.leaving)
	}
	initialLeaving := el.sugg.leaving.alpha
	tester.Advance(suggExitDuration / 2)
	if el.sugg.leaving == nil || el.sugg.leaving.alpha <= 0 || el.sugg.leaving.alpha >= initialLeaving {
		t.Fatalf("expected mid-exit fading leaving ghost, got %+v", el.sugg.leaving)
	}
	tester.Advance(suggExitDuration + 20*time.Millisecond)
	if el.sugg.leaving != nil {
		t.Fatalf("expected leaving ghost gone after suggExitDuration, still %+v", el.sugg.leaving)
	}

	// Accepting via Tab commits the text directly with no leaving ghost.
	ed.Do(profile.SetSuggestion{At: ed.Selection().Head.Pos, Text: "omising"}, tester.Now())
	tester.Advance(suggEnterDuration)
	el.handle(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyTab, Time: tester.Now()}, ui.Services{})
	tester.Frame()
	if el.sugg.leaving != nil {
		t.Fatalf("did not expect leaving ghost after Tab accept, got %+v", el.sugg.leaving)
	}
}
