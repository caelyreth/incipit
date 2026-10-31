package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func pickingState(t *testing.T, sheets int) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	library.extraSheets = sheets
	state.ActivePage = "7"
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	return state, library, tester
}

func TestShiftClickPicksARunOfSheets(t *testing.T) {
	state, library, tester := pickingState(t, 4)
	if err := tester.Click("Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 3"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := state.pickedSheets(); !slices.Equal(got, []string{"extra-0", "extra-1", "extra-2"}) {
		t.Fatalf("picked %v", got)
	}
	if slices.Contains(library.calls, "open extra-2") {
		t.Fatalf("a shift-click opened the sheet: %v", library.calls)
	}
	if !tester.HasText("3 selected · 300 words") {
		t.Fatalf("the header does not count the picked: %v", tester.Texts())
	}
	// A shift-click from the same anchor the other way picks back to it.
	if err := tester.ClickWith(ui.Shift, "Chapter"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-0"}) {
		t.Fatalf("picked %v after picking back", got)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("Escape kept %v", state.pickedSheets())
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Sheet 4"); err != nil {
		t.Fatal(err)
	}
	if len(state.pickedSheets()) != 0 || !slices.Contains(library.calls, "open extra-3") {
		t.Fatalf("a plain click kept %v, calls %v", state.pickedSheets(), library.calls)
	}
}

func TestThePickedSheetsTakeABatchMenu(t *testing.T) {
	state, library, tester := pickingState(t, 3)
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if err := tester.RightClick("Sheet 1"); err != nil {
		t.Fatal(err)
	}
	menu := tester.Menu()
	for _, item := range []string{"Open 3 Sheets in Tabs", "Duplicate 3 Sheets", "Move To", "Move 3 Sheets to Trash"} {
		if !slices.Contains(menu, item) {
			t.Fatalf("batch menu %v lacks %q", menu, item)
		}
	}
	if err := tester.ChooseMenuItem("Move To", "Inbox"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	var moved []string
	for _, move := range library.moves {
		moved = append(moved, move.id)
	}
	if !slices.Equal(moved, []string{"7", "extra-0", "extra-1"}) {
		t.Fatalf("moved %v, want the picked in order", moved)
	}
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("the picked stayed after the move: %v", state.pickedSheets())
	}

	// A menu on a sheet not picked is its own, and lets the picked go.
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if err := tester.RightClick("Sheet 3"); err != nil {
		t.Fatal(err)
	}
	if menu := tester.Menu(); !slices.Contains(menu, "Move to Trash") {
		t.Fatalf("an unpicked sheet's menu %v", menu)
	}
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("the picked stayed for another sheet's menu: %v", state.pickedSheets())
	}
}

// Sheets picked side by side share one fill, across the space between
// them; a sheet not picked keeps the space to the list.
func TestPickedNeighboursShareOneFill(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	state.ActivePage = ""
	if err := tester.Click("Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	first, _ := tester.Find("Sheet 1")
	second, _ := tester.Find("Sheet 2")
	third, _ := tester.Find("Sheet 3")
	x := int(first.X + first.W/2)
	image := tester.Image()
	joined := image.RGBAAt(x, int((first.Y+first.H+second.Y)/2))
	inside := image.RGBAAt(x, int(first.Y+first.H/2))
	apart := image.RGBAAt(x, int((second.Y+second.H+third.Y)/2))
	if joined != inside {
		t.Fatalf("the space between picked sheets is %v, their fill %v", joined, inside)
	}
	if apart == inside {
		t.Fatalf("the fill reached past the run: %v", apart)
	}
}
