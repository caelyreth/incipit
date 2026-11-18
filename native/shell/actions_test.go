package shell

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/native/stage"
	"github.com/egoist/mygo/ui"
)

// The trash is emptied from the pill's actions, which stand beside it:
// Empty Trash asking
// once more before it runs, and the actions closing once it has.
func TestTheTrashEmptiesFromThePillsActions(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	if err := tester.Click("Show Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.ContainsFunc(state.fieldActions(), func(a fieldAction) bool { return a.id == emptyTrashKey }) {
		t.Fatal("A trash holding something should be one to empty.")
	}
	if _, ok := tester.Find("Empty Trash"); ok {
		t.Fatal("The actions should stand only once opened.")
	}
	if err := tester.Click("Actions"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	pill, _ := tester.Find("Search")
	empty, ok := tester.Find("Empty Trash")
	if !ok || empty.X < pill.X+pill.W || abs(empty.Y+empty.H/2-(pill.Y+pill.H/2)) > 1 {
		t.Fatalf("Empty Trash should stand on the pill's line past its end (%v), at %v.", pill, empty)
	}
	if err := tester.Click("Empty Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if slices.Contains(library.calls, "empty trash") {
		t.Fatal("One press should only ask; the trash should not empty yet.")
	}
	if err := tester.Click("Empty Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Contains(library.calls, "empty trash") || state.field.acting {
		t.Fatalf("Pressed again, the trash should empty and the actions close: %v, open %v.", library.calls, state.field.acting)
	}
}

// One thing opens around the pill at a time: the actions put the offered
// keys away, and Escape closes them, the keys coming back.
func TestTheActionsAndTheKeysTakeTurns(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	if _, ok := tester.Find("Tag"); !ok {
		t.Fatal("The open pill should offer its keys.")
	}
	if err := tester.Click("Actions"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if _, ok := tester.Find("New Sheet"); !ok {
		t.Fatal("At home, the actions should offer a new sheet.")
	}
	if _, ok := tester.Find("Tag"); ok {
		t.Fatal("With the actions open, the keys should be put away.")
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if _, ok := tester.Find("New Sheet"); ok || state.field.acting {
		t.Fatal("Escape should close the actions.")
	}
	if _, ok := tester.Find("Tag"); !ok {
		t.Fatal("The actions closed, the keys should come back.")
	}
}

// The open pill offers no key for the trash: the lane's button is the way
// in, and $trash, written, still stands first among the conditions.
func TestThePillOffersNoTrashKey(t *testing.T) {
	if slices.Contains(filterKeys, "trash") {
		t.Fatal("The open pill should not offer the trash.")
	}
	if keyRank("trash") > keyRank("in") {
		t.Fatal("$trash, written, should still stand before what it is in.")
	}
}

// The actions stand centred on the pill's line, about its end, and close
// on a press anywhere else in the field, the pill too.
func TestTheActionsStandAboutThePillsEnd(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	if err := tester.Click("Actions"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	pill, _ := tester.Find("Search")
	first, _ := tester.Find("New Sheet")
	second, _ := tester.Find("New Group")
	mid := pill.Y + pill.H/2
	if abs((first.Y+first.H/2+second.Y+second.H/2)/2-mid) > 1 || first.X < pill.X+pill.W {
		t.Fatalf("The actions should stand centred on the pill's line (%v), past its end: %v, %v.", pill, first, second)
	}
	tester.ClickAt(pill.X-200, pill.Y+pill.H+200)
	settle(tester)
	if state.field.acting {
		t.Fatal("A press elsewhere should close the actions.")
	}
	if err := tester.Click("Actions"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.ClickAt(pill.X+pill.W/3, pill.Y+pill.H/2)
	settle(tester)
	if state.field.acting {
		t.Fatal("A press on the pill should close the actions too.")
	}
}

// Five actions stand in an arc: the middle furthest out, those over and
// under it alike, each further one nearer the pill.
func TestFiveActionsStandInAnArc(t *testing.T) {
	at := arc(5, stage.Rect{X: 100, Y: 0, W: 400, H: 40}, 28, 12, 6)
	if at[2].X != 512 || at[2].Y != 6 {
		t.Fatalf("The middle should stand on the pill's line, gap past its end: %v.", at[2])
	}
	for i := range 2 {
		if at[i].X != at[4-i].X || at[i].X >= at[i+1].X || at[i+1].Y-at[i].Y != 34 {
			t.Fatalf("The arc should be even about the line, nearer the pill further out: %v.", at)
		}
	}
}
