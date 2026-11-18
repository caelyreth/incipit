package shell

import "testing"

// A sheet open on the desk is marked by a tab at its row's leading edge,
// carrying the desk's number for its column, and is never filled, as a
// hover left behind would be.
func TestAnOpenSheetCarriesItsDesksNumber(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.Open = []Page{{ID: "1", Title: "Untitled"}, {ID: "7", Title: "Open chapter"}}
	state.ActivePage = "7"
	settle(tester)
	state.ShowField()
	settle(tester)
	row, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("Chapter's row should show.")
	}
	tab, ok := tester.Find("Open on the desk, column 2")
	if !ok || tab.X >= row.X || tab.Y < row.Y || tab.Y+tab.H > row.Y+row.H {
		t.Fatalf("Chapter should carry the tab for column 2 at its leading edge (%v): %v, %v.", row, tab, ok)
	}
	if _, ok := tester.Find("Open on the desk, column 1"); ok {
		t.Fatal("A sheet the field does not show has no row to mark.")
	}
	img := tester.Image()
	page := state.Settings.Semantic().Surface.Page
	if px := img.RGBAAt(int(row.X+row.W-4), int(row.Y+row.H-4)); px.R != page.R || px.G != page.G || px.B != page.B {
		t.Fatalf("The open sheet's row should stand on the card's ground, not %v.", px)
	}
}
