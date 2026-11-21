package shell

import "testing"

// A sheet open on the desk carries the desk's number for its column; one
// the field does not show has no row to mark.
func TestAnOpenSheetCarriesItsDesksNumber(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Open = []Page{{ID: "1", Column: "1", Title: "Untitled"}, {ID: "7", Column: "7", Title: "Open chapter"}}
	inFront(state, "7")
	settle(tester)
	state.ShowField()
	settle(tester)
	row, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("Chapter's row should show.")
	}
	if _, ok := tester.Find("Open on the desk, column 2"); !ok {
		t.Fatalf("Chapter should carry the badge for column 2 (%v).", row)
	}
	if _, ok := tester.Find("Open on the desk, column 1"); ok {
		t.Fatal("A sheet the field does not show has no row to mark.")
	}
}
