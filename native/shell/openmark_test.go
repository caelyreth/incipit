package shell

import "testing"

// A sheet open on the desk is marked by a strip at its row's leading
// edge and a badge at its foot's end carrying the desk's number for its
// column, and is never filled, as a hover left behind would be.
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
	badge, ok := tester.Find("Open on the desk, column 2")
	if !ok || badge.X < row.X+row.W/2 || badge.X+badge.W > row.X+row.W || badge.Y < row.Y+row.H/2 || badge.Y+badge.H > row.Y+row.H {
		t.Fatalf("Chapter should carry the badge for column 2 at its foot's end (%v): %v, %v.", row, badge, ok)
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
