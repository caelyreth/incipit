package shell

import (
	"slices"
	"testing"
)

// The trash stands first as a card of its own, saying what it holds;
// its Empty button asks once more, then empties it.
func TestTheTrashHasItsOwnCardThatEmptiesIt(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	if err := tester.Click("Show Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); len(got) == 0 || got[0] != "trash" {
		t.Fatalf("The trash's card should stand first, nearest the pill: %v.", got)
	}
	if _, ok := tester.Find("Trash"); !ok {
		t.Fatal("The trash's card should say what it is.")
	}
	if err := tester.Click("Empty Trash…"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if slices.Contains(library.calls, "empty trash") {
		t.Fatal("One click should only ask; the trash should not empty yet.")
	}
	if err := tester.Click("Empty Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Contains(library.calls, "empty trash") {
		t.Fatalf("Clicked again, the trash should empty: %v.", library.calls)
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
