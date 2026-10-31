package shell

import (
	"slices"
	"testing"
	"time"

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

func TestAPressElsewhereLetsThePickedGo(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if len(state.pickedSheets()) != 3 {
		t.Fatalf("picked %v", state.pickedSheets())
	}
	// Another shift-click is the run's to change, not a press elsewhere.
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if len(state.pickedSheets()) != 2 {
		t.Fatalf("picked %v after narrowing the run", state.pickedSheets())
	}
	// The page, right of the list.
	tester.ClickAt(1100, 500)
	tester.Frame()
	if len(state.pickedSheets()) != 0 {
		t.Fatalf("a press on the page kept %v", state.pickedSheets())
	}
}

// The open sheet, picked, is one of the run: no grey of its own, not even
// as it is pressed.
func TestTheOpenSheetTakesTheRunsFill(t *testing.T) {
	_, _, tester := pickingState(t, 2)
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	open, _ := tester.Find("Chapter")
	other, _ := tester.Find("Sheet 1")
	tester.Press(open.X+open.W/2, open.Y+open.H/2)
	for range 12 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	image := tester.Image()
	x := int(open.X + open.W - 12)
	if got, want := image.RGBAAt(x, int(open.Y+open.H/2)), image.RGBAAt(x, int(other.Y+other.H/2)); got != want {
		t.Fatalf("the pressed open sheet's fill is %v, the run's %v", got, want)
	}
}

// As the run lets go, the open sheet and the hovered one go straight from
// the run's fill to their own, never through none.
func TestARunLetsGoStraightToTheRowsOwnFill(t *testing.T) {
	_, _, tester := pickingState(t, 2)
	open, _ := tester.Find("Chapter")
	hovered, _ := tester.Find("Sheet 2")
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	// By a row's edge, which the run's fill leaves first.
	at := func(row ui.Rect) (int, int) { return int(row.X + row.W - 10), int(row.Y + 6) }
	light := func(row ui.Rect) int {
		c := tester.Image().RGBAAt(at(row))
		return int(c.R) + int(c.G) + int(c.B)
	}
	rows := []ui.Rect{open, hovered}
	var frames [][]int
	for frame := range 24 {
		if frame == 1 {
			tester.Key(0, ui.KeyEscape)
		}
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
		frames = append(frames, []int{light(open), light(hovered)})
	}
	for index := range rows {
		from, to := frames[0][index], frames[len(frames)-1][index]
		low, high := min(from, to)-2, max(from, to)+2
		for frame, values := range frames {
			if values[index] < low || values[index] > high {
				t.Fatalf("row %d went from %d to %d through %d at frame %d", index, from, to, values[index], frame)
			}
		}
	}
}

func TestCmdClickPicksOrLetsGoOfOneSheet(t *testing.T) {
	state, library, tester := pickingState(t, 3)
	// The open sheet is the selection a Cmd-click adds to.
	if err := tester.ClickWith(ui.Cmd, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-1"}) {
		t.Fatalf("picked %v", got)
	}
	if err := tester.ClickWith(ui.Cmd, "Chapter"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"extra-1"}) {
		t.Fatalf("picked %v after letting the open sheet go", got)
	}
	settle(tester)
	if !tester.HasText("1 selected · 100 words") {
		t.Fatalf("the header does not count one picked: %v", tester.Texts())
	}
	// A shift-click runs from the last Cmd-clicked sheet, keeping what was
	// picked apart; another reshapes that run alone.
	if err := tester.ClickWith(ui.Shift, "Sheet 3"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-0", "extra-1", "extra-2"}) {
		t.Fatalf("picked %v after a shift-click", got)
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 1"); err != nil {
		t.Fatal(err)
	}
	if got := state.pickedSheets(); !slices.Equal(got, []string{"7", "extra-0", "extra-1"}) {
		t.Fatalf("picked %v after reshaping the run", got)
	}
	for _, call := range library.calls {
		if call != "open 7" && len(call) > 4 && call[:4] == "open" {
			t.Fatalf("picking opened a sheet: %v", library.calls)
		}
	}
}
