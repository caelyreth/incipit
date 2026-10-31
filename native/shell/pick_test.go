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

// Picked sheets are for what is done to them: resting between two rows
// calls up no create area, and one open goes.
func TestPickingKeepsTheCreateAreasShut(t *testing.T) {
	state, _, tester := pickingState(t, 2)
	first, _ := tester.Find("Chapter")
	second, _ := tester.Find("Sheet 1")
	hover := func() {
		tester.Move(first.X+first.W/2, (first.Y+first.H+second.Y)/2)
		for range 30 {
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
		}
	}
	hover()
	if !state.nav.sheetsGap.open {
		t.Fatal("resting between rows did not call up a create area")
	}
	if err := tester.ClickWith(ui.Shift, "Sheet 2"); err != nil {
		t.Fatal(err)
	}
	hover()
	if state.nav.sheetsGap.open {
		t.Fatal("a create area opened between rows while sheets were picked")
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

// fillColumn reads, down the middle of the list's rows, where the picked
// fill is.
type fillColumn struct {
	t      *testing.T
	tester *ui.Tester
	fill   ui.Color
	x      int
}

func newFillColumn(t *testing.T, state *State, tester *ui.Tester) fillColumn {
	row, _ := tester.Find("Chapter")
	return fillColumn{t: t, tester: tester, fill: state.Settings.Semantic().Accent.Muted, x: int(row.X + row.W/2)}
}

func (f fillColumn) filled(y int) bool {
	c := f.tester.Image().RGBAAt(f.x, y)
	return c.R == f.fill.R && c.G == f.fill.G && c.B == f.fill.B
}

func (f fillColumn) center(name string) int {
	row, ok := f.tester.Find(name)
	if !ok {
		f.t.Fatalf("no %q", name)
	}
	return int(row.Y + row.H/2)
}

// clickAway clicks name with mods, then takes the pointer off the list, so
// no row under it shows its hover.
func clickAway(t *testing.T, tester *ui.Tester, mods ui.Modifiers, name string) {
	t.Helper()
	if err := tester.ClickWith(mods, name); err != nil {
		t.Fatal(err)
	}
	tester.Move(1100, 500)
}

// frames runs the next frames, calling check on each.
func (f fillColumn) frames(check func(frame int)) {
	for frame := range 24 {
		f.tester.Advance(16 * time.Millisecond)
		f.tester.Frame()
		check(frame)
	}
}

// A run that grows is one fill whose edge glides over the rows it takes,
// never a row filling apart from it.
func TestARunGrowsAsOneFill(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	clickAway(t, tester, ui.Shift, "Sheet 1")
	settle(tester)
	column := newFillColumn(t, state, tester)
	top, end := column.center("Chapter"), column.center("Sheet 3")
	clickAway(t, tester, ui.Shift, "Sheet 3")
	edges := map[int]bool{}
	previous := 0
	column.frames(func(frame int) {
		edge := top
		for edge+1 <= end+40 && column.filled(edge+1) {
			edge++
		}
		for y := edge + 1; y <= end; y++ {
			if column.filled(y) {
				t.Fatalf("frame %d: fill at %d apart from the run, which ends at %d", frame, y, edge)
			}
		}
		if edge < previous {
			t.Fatalf("frame %d: the edge went back from %d to %d", frame, previous, edge)
		}
		previous = edge
		edges[edge] = true
	})
	if previous < end {
		t.Fatalf("the run ended at %d, short of Sheet 3 at %d", previous, end)
	}
	if len(edges) < 4 {
		t.Fatalf("the edge jumped rather than glided: %v", edges)
	}
}

// Letting go of a sheet inside a run parts it there, the rows either side
// filled all the while; picking it again joins the two.
func TestARunPartsAndJoinsWhereASheetIsCmdClicked(t *testing.T) {
	state, _, tester := pickingState(t, 3)
	clickAway(t, tester, ui.Shift, "Sheet 2")
	settle(tester)
	column := newFillColumn(t, state, tester)
	first, middle, last := column.center("Chapter"), column.center("Sheet 1"), column.center("Sheet 2")
	for _, step := range []struct {
		name   string
		middle bool
	}{{"parts", false}, {"joins", true}} {
		clickAway(t, tester, ui.Cmd, "Sheet 1")
		column.frames(func(frame int) {
			if !column.filled(first) || !column.filled(last) {
				t.Fatalf("%s, frame %d: a row either side lost the fill", step.name, frame)
			}
		})
		settle(tester)
		if column.filled(middle) != step.middle {
			t.Fatalf("%s: the middle row filled %v", step.name, column.filled(middle))
		}
		for y := first; step.middle && y <= last; y++ {
			if !column.filled(y) {
				t.Fatalf("joined, the run is not one fill at %d", y)
			}
		}
	}
}

// A picked row under the pointer is a step stronger than the run, of the
// accent, not the grey of a row's hover.
func TestAPickedRowsHoverIsAStepOfTheAccent(t *testing.T) {
	state, _, tester := pickingState(t, 2)
	clickAway(t, tester, ui.Shift, "Sheet 2")
	settle(tester)
	column := newFillColumn(t, state, tester)
	row, _ := tester.Find("Sheet 1")
	y := column.center("Sheet 1")
	rest := tester.Image().RGBAAt(column.x, y)
	tester.Move(row.X+row.W/2, row.Y+row.H/2)
	settle(tester)
	hovered := tester.Image().RGBAAt(column.x, y)
	accent := state.Settings.Semantic().Accent.Foreground
	toward := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	if hovered == rest {
		t.Fatal("hovering a picked row did not change it")
	}
	if toward(hovered.R, accent.R)+toward(hovered.G, accent.G)+toward(hovered.B, accent.B) >=
		toward(rest.R, accent.R)+toward(rest.G, accent.G)+toward(rest.B, accent.B) {
		t.Fatalf("the hover went from %v to %v, not toward the accent %v", rest, hovered, accent)
	}
}
