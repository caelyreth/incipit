package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A formula the editor says is being edited opens; under a held pointer
// what shows stays as it was.
func TestAnEditedFormulaOpens(t *testing.T) {
	text := "a $x$ b"
	flow, err := surface.NewFlow(projection.NewVersion(0, 0), text, []surface.StyleRun{{
		Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
	}}, lineBlocks(text))
	if err == nil {
		flow, err = ownedBy(flow, asObjects([]surface.Formula{{
			Range: model.NewUTF16Range(2, 5), Text: model.NewUTF16Range(3, 4), Source: "x",
		}}))
	}
	if err != nil {
		t.Fatal(err)
	}
	element := &Element{}
	f := surface.ObjectsOf[surface.Formula](flow)[0]
	open := func(edited ...model.UTF16Range) bool {
		shown, _ := element.formulasShown(flow, edited, time.Unix(0, 0), false)
		return shown[textlayout.KeyOf(f)].Open
	}
	if open() {
		t.Fatal("a formula no one edits opened")
	}
	if !open(f.Range) {
		t.Fatal("an edited formula stayed set")
	}
	element.pointer = pointerSelecting
	if !open() {
		t.Fatal("the formula closed under a held pointer")
	}
	element.pointer = pointerIdle
	if open() {
		t.Fatal("the formula stayed open once the pointer let go")
	}
}

func formulaPage(t *testing.T, typed string) (*controller.DocumentController, *Element, *ui.Tester) {
	t.Helper()
	document, err := controller.NewTextDocument("", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	input := NewBridge(document, nil, func() time.Time { return time.Unix(1_700_000_000, 0) })
	settings := theme.DefaultSettings()
	element := NewElement(input, settings.Semantic(), theme.NewProseStyle(settings.ProseFontSize), time.Unix(1_700_000_000, 0))
	element.Animate = true
	tester := ui.NewTester(element.View, 600, 400)
	for _, r := range typed {
		if _, err := document.ReplaceSelection(string(r), kernel.EventOriginTyping); err != nil {
			t.Fatal(err)
		}
	}
	tester.Frame()
	return document, element, tester
}

func settle(tester *ui.Tester) {
	for range 3 {
		tester.Advance(time.Second)
		tester.Frame()
	}
}

// Typing a formula edits it on its card at once; Escape leaves it after
// it, the card closing, and Tab there opens it again with the caret at its
// source's end. Arrows pass over it, and out of its source past its ends.
func TestTypingAFormulaEditsItOnItsCard(t *testing.T) {
	document, element, tester := formulaPage(t, "a $x^2")
	settle(tester)
	cards := element.current.FormulaCards()
	if len(cards) != 1 || cards[0].Shown != 1 {
		t.Fatalf("typing a formula opened %+v", cards)
	}
	if got := document.Selection().Caret(); got != 6 {
		t.Fatalf("caret at %d, not the source's end", got)
	}
	key := func(key ui.Key) {
		t.Helper()
		if !element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: key}) {
			t.Fatalf("key %v was not taken", key)
		}
		settle(tester)
	}
	key(ui.KeyEscape)
	if got := document.Selection().Caret(); got != 7 {
		t.Fatalf("Escape left the caret at %d", got)
	}
	if cards := element.current.FormulaCards(); len(cards) != 0 {
		t.Fatalf("the card stayed after Escape: %+v", cards)
	}
	key(ui.KeyTab)
	if got := document.Selection().Caret(); got != 6 || len(element.current.FormulaCards()) != 1 {
		t.Fatalf("Tab after the formula left the caret at %d, cards %+v", got, element.current.FormulaCards())
	}
	key(ui.KeyRight)
	if got := document.Selection().Caret(); got != 7 {
		t.Fatalf("Right from the source's end went to %d", got)
	}
	key(ui.KeyLeft)
	if got := document.Selection().Caret(); got != 2 {
		t.Fatalf("Left over the formula went to %d", got)
	}
	key(ui.KeyRight)
	if got := document.Selection().Caret(); got != 7 {
		t.Fatalf("Right over the formula went to %d", got)
	}
}

// $$ opens a formula block edited on its card; Escape leaves it for the
// line after it, the card closing.
func TestEscapeLeavesAFormulaBlockForTheLineAfter(t *testing.T) {
	document, element, tester := formulaPage(t, "p\n$$x\n\nq")
	if err := document.SetSelection(controller.UTF16Range{Start: 5, End: 5}); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if cards := element.current.FormulaCards(); len(cards) != 1 || !cards[0].Formula.Display {
		t.Fatalf("the caret in a block's source opened %+v", cards)
	}
	element.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	settle(tester)
	if got := document.Selection().Caret(); got != 10 || len(element.current.FormulaCards()) != 0 {
		t.Fatalf("Escape left the caret at %d in %q, cards %+v", got, document.Text(), element.current.FormulaCards())
	}
}

// While a formula is edited it shows what it last set soundly, faint, or
// nothing before it has set anything; a half typed command is never set.
// One emptied shows nothing, and forgets what it was.
func TestAFormulaBeingTypedKeepsWhatItLastSet(t *testing.T) {
	element := &Element{}
	shows := func(source string) (string, bool) {
		text := "a $" + source + "$"
		flow, err := surface.NewFlow(projection.NewVersion(0, 0), text, []surface.StyleRun{{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
		}}, lineBlocks(text))
		if err == nil {
			end := model.UTF16Offset(len(text))
			flow, err = ownedBy(flow, asObjects([]surface.Formula{{
				Node: 1, Range: model.NewUTF16Range(2, end), Text: model.NewUTF16Range(3, end-1), Source: source,
			}}))
		}
		if err != nil {
			t.Fatal(err)
		}
		f := surface.ObjectsOf[surface.Formula](flow)[0]
		shown, _ := element.formulasShown(flow, []model.UTF16Range{f.Range}, time.Unix(0, 0), false)
		show := shown[textlayout.KeyOf(f)]
		if show.Stale {
			return show.Last, true
		}
		return source, false
	}
	for _, step := range []struct {
		typed, shown string
		stale        bool
	}{
		{"\\f", "", true}, {"x", "x", false}, {"x+", "x+", false}, {"x+\\fr", "x+", true},
		{"x+\\frac{1}{2}", "x+\\frac{1}{2}", false}, {"", "", false}, {"\\fr", "", true},
	} {
		if got, stale := shows(step.typed); got != step.shown || stale != step.stale {
			t.Fatalf("typing %q shows %q (stale %v), want %q (%v)", step.typed, got, stale, step.shown, step.stale)
		}
	}
}
