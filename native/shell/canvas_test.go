package shell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
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

// joining opens sheets on the canvas one after another, as the window
// opens each in front, so each joins the workspace after the one before.
func joining(state *State, tester *ui.Tester, pages ...Page) {
	for _, page := range pages {
		state.Open = append(state.Open, page)
		state.ActivePage = page.ID
		tester.Frame()
	}
	settle(tester)
}

func sheetPages(ids ...string) []Page {
	var pages []Page
	for _, id := range ids {
		pages = append(pages, Page{ID: id, Title: "Sheet " + id})
	}
	return pages
}

// gapAfter is where, across the window, the room after the strip's i-th
// column stands.
func gapAfter(state *State, i int) float32 {
	sp := state.canvas.spans[i]
	return canvasEdge - state.canvas.strip.X + sp.x + sp.w + columnGap/2
}

// closeColumn presses close in the column labelled column on the canvas,
// its head shown with the pointer on it.
func closeColumn(tester *ui.Tester, column, close string) error {
	if col, ok := tester.Find(column); ok {
		tester.Move(col.X+col.W/2, col.Y+col.H/2)
		settle(tester)
	}
	return tester.Click(close)
}

// closeSheet closes the sheet titled title on the canvas.
func closeSheet(tester *ui.Tester, title string) error {
	return closeColumn(tester, title, "Close "+title)
}

// columnsOf names the workspace's columns: a sheet by its id, a picker
// as "picker".
func columnsOf(state *State) []string {
	var out []string
	for _, col := range state.workspace().columns {
		if col.picker != nil {
			out = append(out, "picker")
		} else {
			out = append(out, col.sheet)
		}
	}
	return out
}

// The canvas comes with the sheet in front and nothing else, takes the
// window from the panes, and gives it back as it was.
func TestTheCanvasComesWithTheSheetInFrontAlone(t *testing.T) {
	state, _, tester := onCanvas(t)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1"}) {
		t.Fatalf("The canvas should hold the sheet in front alone, but it holds %v.", cols)
	}
	if _, ok := tester.Find("Sheet picker"); ok {
		t.Fatal("Coming to the canvas should make no picker.")
	}
	if _, ok := tester.Find("Untitled"); !ok {
		t.Fatal("The sheet in front should stand on the canvas.")
	}
	if _, ok := tester.Find("Groups"); ok {
		t.Fatal("The library should give way to the canvas, but it stays beside it.")
	}
	if err := tester.Click("Page mode"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.CanvasOpen() || !state.SidebarVisible() || state.ActivePage != "1" {
		t.Fatal("Leaving the canvas should bring the panes back as they were, at the sheet in front.")
	}
}

// The canvas keeps its workspace from one visit to the next: a sheet in
// front that stands in it shows it as it was, and one that does not joins
// it at its end.
func TestTheCanvasKeepsItsWorkspaceAcrossModes(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.ToggleCanvas()
	settle(tester)
	// In the page mode, another sheet comes in front, and joins.
	state.Open = append(state.Open, Page{ID: "7", Title: "Chapter"})
	state.ActivePage = "7"
	state.ToggleCanvas()
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "7"}) {
		t.Fatalf("A sheet not on the canvas should join it at its end, but it holds %v.", cols)
	}
	// Back with a sheet that stands there: nothing changes.
	state.ToggleCanvas()
	state.ActivePage = "1"
	state.ToggleCanvas()
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "7"}) {
		t.Fatalf("A sheet standing on the canvas should show it as it was, but it holds %v.", cols)
	}
}

// With nothing in front, the canvas comes back as it was and nothing
// joins; with nothing in front on the canvas, the page has nothing in
// front either.
func TestNothingInFrontStaysNothing(t *testing.T) {
	state, _, tester := onCanvas(t)
	cleared := 0
	state.ClearPage = func() { cleared++; state.ActivePage = "" }
	state.ToggleCanvas()
	state.ActivePage = ""
	state.ToggleCanvas()
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1"}) || state.ActivePage != "" {
		t.Fatalf("With nothing in front, the canvas should come back as it was, but it holds %v, %q in front.",
			cols, state.ActivePage)
	}
	if cleared != 0 {
		t.Fatal("Coming to the canvas should leave the page's sheets as they are.")
	}
	// A sheet in front that is not on the canvas, as one opened from the
	// palette and closed off it, is nothing in front there.
	state.ToggleCanvas()
	state.ActivePage = "1"
	state.ToggleCanvas()
	state.workspace().columns = nil
	state.ToggleCanvas()
	if cleared != 1 || state.ActivePage != "" {
		t.Fatalf("Leaving the canvas with nothing in front should leave nothing in front, but %q is (cleared %d).",
			state.ActivePage, cleared)
	}
}

// Leaving the canvas, the sheet list goes to where the sheet in front
// stands where the place it shows does not list it.
func TestLeavingTheCanvasListsTheSheetInFront(t *testing.T) {
	state, _, tester := onCanvas(t)
	joining(state, tester, Page{ID: "7", Title: "Chapter"})
	if state.Place() == (Place{Kind: PlaceGroup, Group: "2"}) {
		t.Fatal("The test needs the list elsewhere first.")
	}
	state.ToggleCanvas()
	settle(tester)
	if got := state.Place(); got != (Place{Kind: PlaceGroup, Group: "2"}) {
		t.Fatalf("The list should go to the group of the sheet in front, but it shows %+v.", got)
	}
}

// The writing actions offer a new picker on the canvas alone.
func TestTheCanvasIsAModeOfTheWindow(t *testing.T) {
	_, _, tester := onCanvas(t)
	if _, ok := tester.Find("New picker"); !ok {
		t.Fatal("The writing actions should offer a new picker on the canvas.")
	}
	for _, label := range []string{"New sheet", "Toggle library", "Toggle sheet list", "End search", "Zoom in", "Light sheets"} {
		if _, ok := tester.Find(label); ok {
			t.Fatalf("The canvas should not show %q.", label)
		}
	}
}

// The window's controls switch the mode, and the canvas's own actions
// show only while it does.
func TestTheModeSwitchesFromTheWindowsControls(t *testing.T) {
	state, _ := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if _, ok := tester.Find("Canvas actions"); ok {
		t.Fatal("The canvas's actions should not show on the page.")
	}
	if err := tester.Click("Canvas mode"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !state.CanvasOpen() {
		t.Fatal("Choosing the canvas should lay the notebook out on it.")
	}
	if err := tester.Click("New picker"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "picker"}) {
		t.Fatalf("New picker should make one after the sheet in front, but the canvas holds %v.", cols)
	}
}

// Right-clicked in its room between columns, the canvas makes a picker
// there.
func TestRightClickingTheCanvasMakesAPickerThere(t *testing.T) {
	state, _, tester := onCanvas(t)
	joining(state, tester, Page{ID: "7", Title: "Chapter"})
	if len(state.canvas.spans) != 2 {
		t.Fatalf("Both sheets should show, but the canvas shows %#v.", tester.Texts())
	}
	// The gap between their columns.
	tester.RightClickAt(gapAfter(state, 0), 400)
	if err := tester.ChooseMenuItem("New Picker"); err != nil {
		t.Fatalf("The canvas's room should offer a new picker: %v (menu %v)", err, tester.Menu())
	}
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "picker", "7"}) {
		t.Fatalf("The picker should stand where the canvas was right-clicked, but the canvas holds %v.", cols)
	}
}

// A sheet clicked in a picker takes the picker's place, the picker
// turning into its page, and the canvas stays.
func TestASheetChosenInAPickerTakesItsPlace(t *testing.T) {
	state, library, tester := onCanvas(t)
	state.NewPicker()
	settle(tester)
	if err := tester.Click("Draft"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !slices.Contains(library.calls, "open alongside 8") || !state.CanvasOpen() {
		t.Fatalf("The picker should open its sheet with the others and keep the canvas, but the calls are %v.",
			library.calls)
	}
	// The window opens it.
	state.Open = append(state.Open, Page{ID: "8", Title: "Draft"})
	state.ActivePage = "8"
	tester.Frame()
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "8"}) {
		t.Fatalf("The sheet should stand where the picker stood, but the canvas holds %v.", cols)
	}
}

// A sheet opened alongside in a picker comes in after it, and the picker
// stays for the next.
func TestASheetOpenedAlongsideComesInBesideThePicker(t *testing.T) {
	state, library, tester := onCanvas(t)
	state.NewPicker()
	settle(tester)
	if err := tester.ClickWith(ui.Alt, "Draft"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !slices.Contains(library.calls, "open alongside 8") {
		t.Fatalf("Option should open the sheet alongside, but the calls are %v.", library.calls)
	}
	state.Open = append(state.Open, Page{ID: "8", Title: "Draft"})
	state.ActivePage = "8"
	tester.Frame()
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "picker", "8"}) {
		t.Fatalf("The sheet should come in after the picker, which stays, but the canvas holds %v.", cols)
	}
}

// A picker closes.
func TestAPickerCloses(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.NewPicker()
	settle(tester)
	if err := closeColumn(tester, "Sheet picker", "Close picker"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1"}) {
		t.Fatalf("Closing a picker should take it off the canvas, but it holds %v.", cols)
	}
}

// The canvas holds its sheets at once, each written in where it stands;
// the pointer coming onto one brings it in front, and its head closes it,
// the one beside it coming in front where it was, or none.
func TestTheCanvasHoldsItsSheetsAtOnce(t *testing.T) {
	state, _, tester := onCanvas(t)
	focused := map[string]bool{}
	state.Sheet = func(c *ui.Context, id string, frame SheetFrame) {
		focused[id] = frame.Focus
		ui.Text(c, "Body of "+id)
	}
	var selected, closed []string
	cleared := 0
	state.SelectPage = func(id string) { selected = append(selected, id); state.ActivePage = id }
	state.ClosePageID = func(id string) {
		closed = append(closed, id)
		state.Open = slices.DeleteFunc(state.Open, func(p Page) bool { return p.ID == id })
	}
	state.ClearPage = func() { cleared++; state.ActivePage = "" }
	joining(state, tester, Page{ID: "7", Title: "Chapter"}, Page{ID: "8", Title: "Draft"})
	state.ActivePage = "7"
	tester.Frame()
	settle(tester)
	if !tester.HasText("Body of 1") || !tester.HasText("Body of 7") || !tester.HasText("Body of 8") {
		t.Fatalf("Every sheet on the canvas should stand there, but it shows %#v.", tester.Texts())
	}
	if !focused["7"] || focused["8"] {
		t.Fatalf("Only the sheet in front should take the focus, but the frames ask %v.", focused)
	}
	draft, ok := tester.Find("Body of 8")
	if !ok {
		t.Fatal("Draft should show.")
	}
	tester.Move(draft.X+4, draft.Y+draft.H/2)
	tester.Frame()
	settle(tester)
	if !slices.Equal(selected, []string{"8"}) {
		t.Fatalf("The pointer coming onto a sheet should bring it in front, but the sheets selected are %v.", selected)
	}
	// Draft, last, closes: the one before it comes in front.
	if err := closeSheet(tester, "Draft"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Equal(closed, []string{"8"}) || state.ActivePage != "7" {
		t.Fatalf("Closing the sheet in front should bring the one beside it in front, but %v closed and %q is in front.",
			closed, state.ActivePage)
	}
	for _, title := range []string{"Untitled", "Chapter"} {
		if err := closeSheet(tester, title); err != nil {
			t.Fatal(err)
		}
		settle(tester)
	}
	if cleared != 1 || state.ActivePage != "" {
		t.Fatalf("Closing the last sheet on the canvas should leave none in front, but %q is (cleared %d).",
			state.ActivePage, cleared)
	}
}

// Search goes from the canvas to the sheet list, where it runs.
func TestSearchGoesFromTheCanvasToTheSheetList(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.Search()
	tester.Frame()
	settle(tester)
	if state.CanvasOpen() || !state.list.searching {
		t.Fatal("Search should go back to the page and search in the sheet list.")
	}
}

// The strip goes to the sheet that comes in front, and a drag on its room
// between columns moves it along, coming to rest at a column.
func TestTheStripFollowsTheFrontAndPansByDrag(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.Sheet = func(c *ui.Context, id string, _ SheetFrame) { ui.Text(c, "Body of "+id) }
	state.SelectPage = func(id string) { state.ActivePage = id }
	joining(state, tester, sheetPages("7", "8", "9", "10")...)
	state.ActivePage = "7"
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
	// Between the ninth sheet and the tenth.
	x, y := gapAfter(state, 3), float32(400)
	tester.Press(x, y)
	for i := 1; i <= 10; i++ {
		tester.Move(x+float32(i)*30, y)
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if moved := last - state.canvas.strip.X; moved < 200 {
		t.Fatalf("A drag on the strip should move it along, but it moved %v.", moved)
	}
	tester.Release(x+300, y)
	tester.Frame()
	settle(tester)
	rest := state.canvas.strip.X
	settle(tester)
	if state.canvas.strip.X != rest {
		t.Fatalf("The strip should come to rest, but it went on from %v to %v.", rest, state.canvas.strip.X)
	}
}

// A sheet's column is half the window less the gap, so two read as a
// split, or a third where the window is wide enough for a third to hold a
// sheet's page; a picker's is the sheet list's width.
func TestTheColumnsShareTheWindowByHalvesOrThirds(t *testing.T) {
	for _, window := range []struct {
		width float32
		n     float32
	}{{1260, 2}, {2600, 3}} {
		state, _ := libraryState()
		tester := ui.NewTester(state.View, int(window.width), 850)
		tester.Frame()
		state.ToggleCanvas()
		tester.Frame()
		joining(state, tester, Page{ID: "7", Title: "Chapter"})
		state.NewPicker()
		settle(tester)
		inner := window.width - 2*canvasEdge
		want := (inner - columnGap*(window.n-1)) / window.n
		// The two sheets, then the picker.
		if spans := state.canvas.spans; len(spans) != 3 || abs(spans[0].w-want) > 1 || abs(spans[1].w-want) > 1 {
			t.Fatalf("At %v across, the sheets' columns should be %v wide, but the strip holds %+v.",
				window.width, want, spans)
		}
		picker, ok := tester.Find("Sheet picker")
		if !ok || abs(picker.W-theme.DefaultLayout.SheetList) > 1 {
			t.Fatalf("A picker should be as wide as the sheet list, but it is %+v.", picker)
		}
	}
}

// The strip goes where it is scrolled, and pins to a column's start only
// once it comes to rest near one, the fingers lifted; far from any, it
// stays where it stopped.
func TestTheStripPinsToAColumnItComesToRestNear(t *testing.T) {
	state, _, tester := onCanvas(t)
	joining(state, tester, sheetPages("7", "8", "9", "10")...)
	state.canvas.strip.ScrollTo(0, 0, ui.Animation{Duration: 1})
	settle(tester)
	rests := state.rests()
	scroll := func(dx float32) {
		tester.Scroll(600, 400, dx, 0)
		tester.Frame()
		tester.Scroll(600, 400, 0, 0) // the fingers lift
		settle(tester)
	}
	// Near the second column's start: pinned there.
	scroll(rests[1] - 30)
	if abs(state.canvas.strip.X-rests[1]) > 1 {
		t.Fatalf("Coming to rest 30 short of a column, the strip should pin to it at %v, but it is at %v.",
			rests[1], state.canvas.strip.X)
	}
	// Midway between two: it stays.
	mid := (rests[1] + rests[2]) / 2
	scroll(mid - state.canvas.strip.X)
	if abs(state.canvas.strip.X-mid) > 1 {
		t.Fatalf("Coming to rest far from any column, the strip should stay at %v, but it went to %v.",
			mid, state.canvas.strip.X)
	}
}

// The strip stretches past its start, as the lists do, and comes back.
func TestTheStripStretchesPastItsEnds(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.canvas.strip.ScrollTo(0, 0, ui.Animation{Duration: 1})
	settle(tester)
	tester.Scroll(600, 400, -80, 0)
	tester.Frame()
	if x, _ := state.canvas.over.Offset(); x == 0 {
		t.Fatal("Scrolled past its start, the strip should stretch.")
	}
	tester.Scroll(600, 400, 0, 0)
	settle(tester)
	if x, _ := state.canvas.over.Offset(); x != 0 || state.canvas.strip.X != 0 {
		t.Fatalf("Let go, the strip should come back to its start, but it is %v past it at %v.", x, state.canvas.strip.X)
	}
}

// The scrubber goes to a column pressed in it, and a drag on it moves the
// strip, coming to rest at a column.
func TestTheScrubberMovesTheStrip(t *testing.T) {
	state, _, tester := onCanvas(t)
	joining(state, tester, sheetPages("7", "8", "9", "10")...)
	state.ActivePage = "7"
	tester.Frame()
	settle(tester)
	// The scrubber shows while the strip moves, as it did coming to the
	// sheet in front.
	state.canvas.strip.X = 1
	tester.Frame()
	bar, ok := tester.Find("Canvas overview")
	if !ok {
		t.Fatal("The scrubber should show while the strip moves.")
	}
	spans := state.canvas.spans
	f := (bar.W - 8) / (spans[len(spans)-1].x + spans[len(spans)-1].w)
	tester.Press(bar.X+4+(spans[3].x+10)*f, bar.Y+bar.H/2)
	tester.Release(bar.X+4+(spans[3].x+10)*f, bar.Y+bar.H/2)
	tester.Frame()
	settle(tester)
	if want := state.rests()[3]; abs(state.canvas.strip.X-want) > 1 {
		t.Fatalf("A column pressed in the scrubber should come to the strip's start at %v, but the strip is at %v.",
			want, state.canvas.strip.X)
	}
	x, y := bar.X+bar.W-6, bar.Y+bar.H/2
	before := state.canvas.strip.X
	tester.Press(x, y)
	for i := 1; i <= 5; i++ {
		tester.Move(x-float32(i)*8, y)
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	tester.Release(x-40, y)
	tester.Frame()
	settle(tester)
	if rest := state.canvas.strip.X; rest >= before {
		t.Fatalf("A drag back on the scrubber should bring the strip back, but it went from %v to %v.", before, rest)
	}
}

// The page and the canvas are two scenes crossing, sharing the sheet in
// front alone: it stands in one at a time, drawn once, flying across;
// the page's layout stays as it was beneath the canvas coming, and is no
// longer built once covered.
func TestThePageAndTheCanvasShareTheSheetInFront(t *testing.T) {
	state, _ := libraryState()
	state.Appearance.Motion = true
	state.Sheet = func(c *ui.Context, id string, _ SheetFrame) { ui.Text(c, "Body of "+id) }
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	listWidth := state.lastSidebarWidth
	step := func(frames int) {
		for range frames {
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
		}
	}
	once := func(when string) {
		t.Helper()
		if n := strings.Count(strings.Join(tester.Texts(), "\n"), "Body of 1"); n != 1 {
			t.Fatalf("%s, the sheet in front should be drawn once, but it is drawn %d times.", when, n)
		}
	}
	// By the mode switch, which, built first, changes the mode as the
	// window is being built.
	if err := tester.Click("Canvas mode"); err != nil {
		t.Fatal(err)
	}
	step(3)
	once("Going to the canvas")
	if _, ok := tester.Find("Groups"); !ok {
		t.Fatal("The page's scene should stay beneath the canvas as it comes.")
	}
	if state.lastSidebarWidth != listWidth {
		t.Fatalf("The panes should stay as they were under the canvas, but they went from %v to %v.",
			listWidth, state.lastSidebarWidth)
	}
	settle(tester)
	once("On the canvas")
	if _, ok := tester.Find("Groups"); ok {
		t.Fatal("Under the whole canvas, the page mode's scene should not be built.")
	}
	if err := tester.Click("Page mode"); err != nil {
		t.Fatal(err)
	}
	step(3)
	once("Going back")
	// Turned back on the way, it goes back from where it is.
	state.ToggleCanvas()
	step(2)
	once("Turned back")
	state.ToggleCanvas()
	settle(tester)
	once("Back on the page")
	if state.CanvasOpen() || !tester.HasText("Groups") {
		t.Fatal("The page should be back with its panes.")
	}
}

// A column brought into view at the strip's end keeps the gap from the
// window's edge, as every column does, rather than meeting it.
func TestAColumnBroughtIntoViewKeepsItsGap(t *testing.T) {
	state, _, tester := onCanvas(t)
	joining(state, tester, sheetPages("7", "8", "9")...)
	state.NewPicker()
	settle(tester)
	picker, ok := tester.Find("Sheet picker")
	if !ok {
		t.Fatal("The new picker should show.")
	}
	if right := picker.X + picker.W; right > 1260-canvasEdge+1 {
		t.Fatalf("The picker should stand %v from the window's edge, but it ends at %v of 1260.", canvasEdge, right)
	}
}

// A column closing at the strip's end closes up where it is: the strip is
// not cut back under it at once, but eases back after it.
func TestClosingAColumnEasesTheStripBack(t *testing.T) {
	state, _, tester := onCanvas(t)
	state.Appearance.Motion = true
	joining(state, tester, sheetPages("7", "8", "9")...)
	state.NewPicker()
	settle(tester)
	before := state.canvas.strip.X
	if before <= 0 {
		t.Fatal("The test needs the strip scrolled along.")
	}
	if err := closeColumn(tester, "Sheet picker", "Close picker"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	if jump := before - state.canvas.strip.X; jump > 40 {
		t.Fatalf("Closing a column, the strip should ease back, but it jumped %v at once.", jump)
	}
	settle(tester)
	if state.canvas.strip.X >= before {
		t.Fatalf("Once it has closed, the strip should have come back, but it stands at %v from %v.",
			state.canvas.strip.X, before)
	}
}

// The column in front follows the pointer: moved onto a sheet's column,
// it brings that sheet in front and gives its page the keyboard, the strip
// staying where it is; moved onto a picker, it brings the picker in front,
// the sheet in front staying for the writing actions. The strip moving
// under a pointer at rest brings nothing.
func TestThePointerBringsTheColumnUnderItInFront(t *testing.T) {
	state, _, tester := onCanvas(t)
	var claimed []string
	state.Sheet = func(c *ui.Context, id string, frame SheetFrame) {
		if frame.Claim {
			claimed = append(claimed, id)
		}
		ui.Text(c, "Body of "+id)
	}
	state.SelectPage = func(id string) { state.ActivePage = id }
	joining(state, tester, sheetPages("7")...)
	state.NewPicker()
	settle(tester)
	state.ActivePage = "1"
	tester.Frame()
	settle(tester)
	strip := state.canvas.strip.X
	body, ok := tester.Find("Body of 7")
	if !ok {
		t.Fatalf("The second sheet should show, but the strip shows %#v.", tester.Texts())
	}
	tester.Move(body.X+body.W/2, body.Y+body.H/2)
	tester.Frame()
	settle(tester)
	if state.ActivePage != "7" || !slices.Contains(claimed, "7") {
		t.Fatalf("The pointer moved onto a sheet should bring it in front and give it the keyboard, but %q is in front (claimed %v).",
			state.ActivePage, claimed)
	}
	if state.canvas.strip.X != strip {
		t.Fatalf("A sheet the pointer brings in front is where it is: the strip should stay at %v, but went to %v.",
			strip, state.canvas.strip.X)
	}
	// The strip moved under the pointer at rest brings nothing in front.
	state.stripAt(state.canvas.strip.MaxX)
	tester.Frame()
	settle(tester)
	if state.ActivePage != "7" || state.canvas.at != "sheet-7" {
		t.Fatalf("The strip moving under the pointer should bring nothing in front, but %q came.", state.canvas.at)
	}
	search, ok := tester.Find("Search sheets")
	if !ok {
		t.Fatal("The picker's search should show.")
	}
	// Onto the picker, under its search.
	tester.Move(search.X+8, search.Y+search.H+40)
	tester.Frame()
	settle(tester)
	if state.canvas.at != "picker-1" || state.ActivePage != "7" {
		t.Fatalf("The pointer moved onto a picker should bring it in front, the sheet in front staying, but %q is in front.",
			state.canvas.at)
	}
}

// The title line is the chrome's: a column under the controls holds what
// it holds under the line, whole, and only its head on it, clear of them,
// centred on the traffic lights as the controls are.
func TestTheTitleLineHoldsOnlyTheHeadsClearOfTheControls(t *testing.T) {
	state, _, tester := onCanvas(t)
	joining(state, tester, sheetPages("7", "8")...)
	state.newPickerAt(len(state.workspace().columns))
	settle(tester)
	state.stripAt(state.canvas.strip.MaxX)
	tester.SetTitleBar(ui.TitleBar{Height: 40, Left: 76})
	settle(tester)
	picker, ok := tester.Find("Sheet picker")
	if !ok {
		t.Fatal("The picker should show.")
	}
	if right := picker.X + picker.W; right < 1260-state.controlsAt.reach {
		t.Fatalf("The test needs the picker under the controls, but it ends at %v.", right)
	}
	tester.Move(picker.X+picker.W/2, picker.Y+picker.H/2)
	settle(tester)
	band := float32(40)
	place, ok := tester.Find("All")
	if !ok || place.W == 0 {
		t.Fatalf("The picker's place should show whole, but the strip shows %#v.", tester.Texts())
	}
	if place.Y < band {
		t.Fatalf("The picker's place should stand under the title line, %v down, but it is at %v.", band, place)
	}
	close, ok := tester.Find("Close picker")
	if !ok {
		t.Fatal("The picker's head should show with the pointer on it.")
	}
	if mid := close.Y + close.H/2; abs(mid-band/2) > 1 {
		t.Fatalf("The picker's head should be centred on the traffic lights, %v down, but it is at %v.", band/2, close)
	}
	if controls, ok := tester.Find("Search commands"); !ok || abs(controls.Y+controls.H/2-band/2) > 1 {
		t.Fatalf("The window's controls should be centred on the traffic lights, %v down, but they are at %v.", band/2, controls)
	}
	if close.X+close.W > 1260-state.controlsAt.reach {
		t.Fatalf("The picker's head should keep clear of the controls, from %v, but it reaches %v.",
			1260-state.controlsAt.reach, close.X+close.W)
	}
}
