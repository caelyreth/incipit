package shell

import (
	"slices"
	"strings"
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

// The canvas shows every group as a region with what it holds, takes the
// window from the panes, and gives it back as it was.
func TestTheCanvasShowsTheNotebookWhole(t *testing.T) {
	state, _, tester := onCanvas(t)
	for _, text := range []string{"Inbox", "Novel", "1 sheet · 1,204 words · 1 this week"} {
		if !tester.HasText(text) {
			t.Fatalf("the canvas lacks %q: %#v", text, tester.Texts())
		}
	}
	if _, ok := tester.Find("Groups"); ok {
		t.Fatal("the library stays beside the canvas")
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	settle(tester)
	if state.CanvasOpen() || !state.SidebarVisible() {
		t.Fatalf("Escape leaves the canvas for the columns as they were: canvas %v", state.CanvasOpen())
	}
}

// Nearer, the canvas shows the sheets' titles; a pinch goes there a step
// at a time.
func TestTheCanvasZoomsByStepsAndPinches(t *testing.T) {
	state, _, tester := onCanvas(t)
	if hasPrefix(tester, "1,204 words · ") {
		t.Fatal("far off, the sheets are marks, not lines")
	}
	region, _ := tester.Find("Novel")
	tester.Move(region.X+40, region.Y+40)
	tester.Pinch(region.X+40, region.Y+40, 1.5)
	tester.Frame()
	settle(tester)
	if state.canvas.zoom != zoomMiddle || !hasPrefix(tester, "1,204 words · ") {
		t.Fatalf("a pinch goes one step nearer: zoom %d", state.canvas.zoom)
	}
	if err := tester.Click("Far"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.canvas.zoom != zoomFar {
		t.Fatalf("Far goes back: zoom %d", state.canvas.zoom)
	}
}

// hasPrefix reports a text shown that starts with prefix.
func hasPrefix(tester *ui.Tester, prefix string) bool {
	return slices.ContainsFunc(tester.Texts(), func(text string) bool { return strings.HasPrefix(text, prefix) })
}

// Opening a card goes back to the columns, at the sheet's group.
func TestOpeningACardReturnsToTheColumns(t *testing.T) {
	state, library, tester := onCanvas(t)
	if err := tester.Click("Near"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("Chapter"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	settle(tester)
	if !slices.Contains(library.calls, "open 7") || state.CanvasOpen() {
		t.Fatalf("the card opens its sheet and leaves the canvas: %v, canvas %v", library.calls, state.CanvasOpen())
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
		t.Fatalf("the search runs on the canvas: %q, %v", state.canvas.query, library.calls)
	}
	if !tester.HasText("2 sheets found") || !tester.HasText("Inbox") {
		t.Fatalf("it says what it found, among the regions: %#v", tester.Texts())
	}
	if state.nav.searching {
		t.Fatal("the columns' search opened under the canvas")
	}
}

// A card dragged onto another region is filed there.
func TestACardDraggedOntoARegionIsFiledThere(t *testing.T) {
	_, library, tester := onCanvas(t)
	if err := tester.Click("Near"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	card, _ := tester.Find("Chapter")
	inbox, _ := tester.Find("Inbox")
	x, y := card.X+card.W/2, card.Y+card.H/2
	tester.Press(x, y)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		tester.Move(x+(inbox.X+10-x)*f, y+(inbox.Y+inbox.H/2-y)*f)
		tester.Frame()
	}
	tester.Release(inbox.X+10, inbox.Y+inbox.H/2)
	tester.Frame()
	if !slices.Contains(library.calls, "move 7 to 1") {
		t.Fatalf("the drop files the sheet: %v", library.calls)
	}
}
