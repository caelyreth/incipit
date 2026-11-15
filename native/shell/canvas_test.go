package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func onCanvas(t *testing.T) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	state.ToggleCanvas()
	tester.Frame()
	settle(tester)
	return state, library, tester
}

// The canvas shows every group as a region of its sheets' cards, takes the
// window from the panes, and gives it back as it was.
func TestTheCanvasShowsTheNotebookWhole(t *testing.T) {
	state, _, tester := onCanvas(t)
	for _, text := range []string{"Inbox", "Novel", "Chapter", "Draft", "1 sheet · 1,204 words · 1 this week"} {
		if !tester.HasText(text) {
			t.Fatalf("The canvas should show %q, but it shows %#v.", text, tester.Texts())
		}
	}
	if _, ok := tester.Find("Groups"); ok {
		t.Fatal("The library should give way to the canvas, but it stays beside it.")
	}
	if err := tester.Click("Toggle canvas"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.CanvasOpen() || !state.SidebarVisible() {
		t.Fatal("Leaving the canvas should bring the columns back as they were.")
	}
}

// A pinch zooms the canvas about the pointer; the head's buttons zoom by
// steps, set it at its actual size, and fit it in view.
func TestTheCanvasZoomsByPinchAndButtons(t *testing.T) {
	state, _, tester := onCanvas(t)
	region, _ := tester.Find("Novel")
	tester.Move(region.X+40, region.Y+40)
	tester.Pinch(region.X+40, region.Y+40, 1.5)
	tester.Frame()
	settle(tester)
	if z := state.canvas.scale(); z != 1.5 {
		t.Fatalf("A pinch of 1.5 should bring the canvas to 150%%, but it is at %v.", z)
	}
	if !tester.HasText("150%") {
		t.Fatalf("The head should say how near the canvas is, but it shows %#v.", tester.Texts())
	}
	if err := tester.Click("Zoom out"); err != nil {
		t.Fatal(err)
	}
	if z := state.canvas.scale(); z != 1.5/zoomStep {
		t.Fatalf("Zoom out should go a step farther, but the canvas is at %v.", z)
	}
	if err := tester.Click("120%"); err != nil {
		t.Fatal(err)
	}
	if z := state.canvas.scale(); z != 1 {
		t.Fatalf("The percentage should set the canvas at its actual size, but it is at %v.", z)
	}
	for range 12 {
		if err := tester.Click("Zoom in"); err != nil {
			t.Fatal(err)
		}
	}
	if z := state.canvas.scale(); z != maxZoom {
		t.Fatalf("Zooming in should stop at %v, but the canvas is at %v.", maxZoom, z)
	}
	if err := tester.Click("Fit"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if z := state.canvas.scale(); z >= maxZoom || state.canvas.strip.X != 0 {
		t.Fatalf("Fit should bring the whole canvas into view from its start, but it is at %v, panned %v.",
			z, state.canvas.strip.X)
	}
}

// Clicking a card opens its sheet in a column of its own, after those
// open, and the canvas stays.
func TestACardOpensInAColumnOfItsOwn(t *testing.T) {
	state, library, tester := onCanvas(t)
	if err := tester.Click("Chapter"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !slices.Contains(library.calls, "open alongside 7") || !state.CanvasOpen() {
		t.Fatalf("The card should open its sheet alongside the others and keep the canvas, but the calls are %v.",
			library.calls)
	}
}

// The strip holds every open sheet at once, each written in where it
// stands; its head brings it in front, takes it back to the columns, or
// closes it.
func TestTheStripHoldsEveryOpenSheet(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.Open = []Page{{ID: "7", Title: "Chapter"}, {ID: "8", Title: "Draft"}}
	state.ActivePage = "7"
	focused := map[string]bool{}
	state.Sheet = func(c *ui.Context, id string, frame SheetFrame) {
		focused[id] = frame.Focus
		ui.Text(c, "Body of "+id)
	}
	var selected, closed []string
	state.SelectPage = func(id string) { selected = append(selected, id); state.ActivePage = id }
	state.ClosePageID = func(id string) { closed = append(closed, id) }
	// Brought in front, a sheet is brought into view.
	state.ActivePage = "8"
	tester.Frame()
	settle(tester)
	state.ActivePage = "7"
	tester.Frame()
	settle(tester)
	if !tester.HasText("Body of 7") || !tester.HasText("Body of 8") {
		t.Fatalf("Both open sheets should stand on the strip, but it shows %#v.", tester.Texts())
	}
	if !focused["7"] || focused["8"] {
		t.Fatalf("Only the sheet in front should take the focus, but the frames ask %v.", focused)
	}
	if err := tester.Click("Bring Draft in front"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Equal(selected, []string{"8"}) {
		t.Fatalf("A column's head should bring its sheet in front, but the sheets selected are %v.", selected)
	}
	if err := tester.Click("Close Draft"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(closed, []string{"8"}) {
		t.Fatalf("A column's close should close its sheet, but the sheets closed are %v.", closed)
	}
	if err := tester.Click("Write Draft in columns"); err != nil {
		t.Fatal(err)
	}
	if state.CanvasOpen() || len(selected) != 2 {
		t.Fatalf("Writing in columns should leave the canvas at that sheet, but the canvas is open %v, selected %v.",
			state.CanvasOpen(), selected)
	}
}

// A search lights what it finds in place and leaves the rest where they
// stand, dimmed.
func TestTheCanvasSearchLightsInPlace(t *testing.T) {
	state, library, tester := onCanvas(t)
	state.Search()
	tester.Frame()
	tester.Type("chap")
	tester.Frame()
	settle(tester)
	if state.canvas.query != "chap" || !slices.Contains(library.calls, "search chap") {
		t.Fatalf("The search should run on the canvas, but its query is %q and the calls are %v.",
			state.canvas.query, library.calls)
	}
	if !tester.HasText("2 sheets found") || !tester.HasText("Inbox") {
		t.Fatalf("The head should say what the search found, among the regions, but it shows %#v.", tester.Texts())
	}
	if state.list.searching {
		t.Fatal("The columns' search should stay closed under the canvas.")
	}
}

// A card dragged onto another region is filed there.
func TestACardDraggedOntoARegionIsFiledThere(t *testing.T) {
	_, library, tester := onCanvas(t)
	dragOnto(t, tester, "Chapter", "Inbox")
	if !slices.Contains(library.calls, "move 7 to 1") {
		t.Fatalf("The drop should file the sheet in the Inbox, but the calls are %v.", library.calls)
	}
}

// The strip goes to the sheet that comes in front, and a drag on it moves
// it along, coming to rest at a column.
func TestTheStripFollowsTheFrontAndPansByDrag(t *testing.T) {
	state, library, tester := onCanvas(t)
	for _, id := range []string{"7", "8", "9", "10"} {
		state.Open = append(state.Open, Page{ID: id, Title: "Sheet " + id})
	}
	library.calls = nil
	state.ActivePage = "7"
	state.Sheet = func(c *ui.Context, id string, _ SheetFrame) { ui.Text(c, "Body of "+id) }
	state.SelectPage = func(id string) { state.ActivePage = id }
	tester.Frame()
	settle(tester)
	before := state.canvas.strip.X
	state.ActivePage = "10"
	tester.Frame()
	settle(tester)
	last := state.canvas.strip.X
	if last <= before {
		t.Fatalf("The strip should go to the sheet in front, but it stayed at %v.", last)
	}
	head, ok := tester.Find("Bring Sheet 9 in front")
	if !ok {
		t.Fatalf("The ninth sheet's head should show, but the strip shows %#v.", tester.Texts())
	}
	x, y := head.X+head.W/2, head.Y+head.H/2
	tester.Press(x, y)
	for i := 1; i <= 10; i++ {
		tester.Move(x+float32(i)*30, y)
		tester.Frame()
	}
	if moved := last - state.canvas.strip.X; moved < 200 {
		t.Fatalf("A drag on a head should move the strip with it, but it moved %v.", moved)
	}
	tester.Release(x+300, y)
	tester.Frame()
	settle(tester)
	rest := state.canvas.strip.X
	if !slices.ContainsFunc(state.canvas.rests, func(x float32) bool { return x-rest < 1 && rest-x < 1 }) &&
		rest != 0 {
		t.Fatalf("The strip should rest at a column, but it rests at %v among %v.", rest, state.canvas.rests)
	}
	if state.ActivePage != "10" {
		t.Fatalf("A drag on a head should bring nothing in front, but %s came.", state.ActivePage)
	}
}
