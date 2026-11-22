package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// toolsPage is a focused page holding text, under a tester.
func toolsPage(t *testing.T, text string) (*Element, *ui.Tester, doc.SlotRef) {
	t.Helper()
	el := newElement(t)
	el.Keys = keymap.Default(keymap.Declared(el.Editor.Engine.Program().Catalog()))
	tt := ui.NewTester(el.View, 900, 600)
	typeText(el, text)
	el.Focus()
	tt.Frame()
	ref, ok := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	if !ok {
		t.Fatal("the page has no text")
	}
	return el, tt, ref
}

func selectSpan(t *testing.T, el *Element, ref doc.SlotRef, from, to int) {
	t.Helper()
	now := time.Now()
	if !el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, from))}, now) ||
		!el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, to)), Extend: true}, now) {
		t.Fatal("could not select")
	}
}

func rest(tt *ui.Tester) {
	tt.Frame()
	tt.Advance(toolsRest * 2)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
}

// A selection come to rest raises its tools; a press on one formats it,
// the tools staying, and Escape puts them away.
func TestASelectionsToolsRiseOverItAndStay(t *testing.T) {
	el, tt, ref := toolsPage(t, "hello world")
	selectSpan(t, el, ref, 6, 11)
	tt.Frame()
	if _, ok := tt.Find("Strong"); ok {
		t.Fatal("The tools rose before the selection came to rest.")
	}
	rest(tt)
	strong, ok := tt.Find("Strong")
	if !ok {
		t.Fatalf("The tools did not rise over the selection: %v", tt.Texts())
	}
	// They stand clear of the selection: under it with no room above, at
	// the view's top, and over it with room.
	line := func() ui.Rect {
		r, _ := el.Editor.layout.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
		return el.fromPage(r)
	}
	if at := line(); strong.Y < at.Y+at.H {
		t.Fatalf("With no room above, the tools at %v should stand under the line at %v.", strong, at)
	}
	el.Top = 200
	rest(tt)
	strong, _ = tt.Find("Strong")
	if at := line(); strong.Y+strong.H > at.Y {
		t.Fatalf("With room above, the tools at %v should stand over the line at %v.", strong, at)
	}
	if err := tt.Click("Strong"); err != nil {
		t.Fatal(err)
	}
	rest(tt)
	if !el.modelOf().marks[0].on {
		t.Fatal("Strong did not mark the selection.")
	}
	if _, ok := tt.Find("Strong"); !ok {
		t.Fatal("The tools went with the first one pressed.")
	}
	tt.Key(0, ui.KeyEscape)
	rest(tt)
	if _, ok := tt.Find("Strong"); ok {
		t.Fatal("Escape left the tools up.")
	}
	if sel := el.Editor.Selection(); sel.IsCollapsed() {
		t.Fatal("The Escape that put the tools away took the selection too.")
	}
}

// Typing over the selection puts the tools away with it.
func TestTypingPutsTheToolsAway(t *testing.T) {
	el, tt, ref := toolsPage(t, "hello world")
	selectSpan(t, el, ref, 0, 5)
	rest(tt)
	if _, ok := tt.Find("Strong"); !ok {
		t.Fatal("The tools did not rise.")
	}
	el.Editor.Do(vocab.Type{Text: "y"}, time.Now())
	rest(tt)
	if _, ok := tt.Find("Strong"); ok {
		t.Fatal("The tools stayed over text typed.")
	}
}

// Show Formatting calls the tools up at a caret: they name its block's
// kind and turn it into another, and a mark toggled at the caret shows
// as it would type.
func TestShowFormattingAtTheCaret(t *testing.T) {
	el, tt, _ := toolsPage(t, "hello")
	el.ToggleSelectionTools()
	tt.Frame()
	if _, ok := tt.Find("Strong"); !ok {
		t.Fatal("Show Formatting did not call the tools up at the caret.")
	}
	if _, ok := tt.Find("Paragraph"); !ok {
		t.Fatalf("The tools do not name the paragraph's kind: %v", tt.Texts())
	}
	if err := tt.Click("Strong"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !el.modelOf().marks[0].on {
		t.Fatal("Strong toggled at the caret does not show as on.")
	}
	el.runTool("blocks.heading2", ui.Services{})
	tt.Frame()
	if m := el.modelOf(); m.block != "blocks.heading2" {
		t.Fatalf("Turned into a heading, the tools name %s.", m.block)
	}
	if n, ok := el.Editor.Engine.Snapshot().Node(el.Editor.Selection().Head.Pos.Node); !ok || n.Kind() != profile.Heading {
		t.Fatal("Heading 2 did not turn the block into a heading.")
	}
	el.ToggleSelectionTools()
	rest(tt)
	if _, ok := tt.Find("Strong"); ok {
		t.Fatal("Show Formatting again left the tools up.")
	}
}

// The tools' Expand climbs the selection's ladder, and the tools follow
// it up.
func TestTheToolsClimbTheSelection(t *testing.T) {
	el, tt, ref := toolsPage(t, "Tide pools. Shore birds.")
	selectSpan(t, el, ref, 0, 4)
	rest(tt)
	if err := tt.Click("Expand selection"); err != nil {
		t.Fatalf("%v: %v", err, tt.Texts())
	}
	rest(tt)
	f, to := el.Editor.Selection().Ordered(el.Editor.Engine.Snapshot())
	if f.Pos.Offset != 0 || to.Pos.Offset != 11 {
		t.Fatalf("Expand from the word selected %d..%d, want its sentence 0..11.", f.Pos.Offset, to.Pos.Offset)
	}
	if _, ok := tt.Find("Strong"); !ok {
		t.Fatal("The tools did not follow the selection up.")
	}
}
