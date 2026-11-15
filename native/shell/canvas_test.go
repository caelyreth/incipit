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
	tester.Frame()
	if z := state.canvas.scale(); z >= maxZoom || state.canvas.scroll.X != 0 {
		t.Fatalf("Fit should bring the whole canvas into view from its start, but it is at %v, panned %v.",
			z, state.canvas.scroll.X)
	}
}

// Clicking a card opens its sheet on the desk, beside those open, and the
// canvas stays.
func TestACardOpensOnTheDesk(t *testing.T) {
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

// The desk holds every open sheet at once, each written in where it lies;
// its head brings it in front, takes it back to the columns, or closes it.
func TestTheDeskHoldsEveryOpenSheet(t *testing.T) {
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
	// Opened on the canvas, a sheet is brought into view.
	state.canvas.reveal = "8"
	tester.Frame()
	settle(tester)
	if !tester.HasText("Body of 7") || !tester.HasText("Body of 8") {
		t.Fatalf("Both open sheets should lie on the desk, but it shows %#v.", tester.Texts())
	}
	if !focused["7"] || focused["8"] {
		t.Fatalf("Only the sheet in front should take the focus, but the frames ask %v.", focused)
	}
	if err := tester.Click("Bring Draft in front"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected, []string{"8"}) {
		t.Fatalf("A panel's head should bring its sheet in front, but the sheets selected are %v.", selected)
	}
	if err := tester.Click("Close Draft"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(closed, []string{"8"}) {
		t.Fatalf("A panel's close should close its sheet, but the sheets closed are %v.", closed)
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
	if state.nav.searching {
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
