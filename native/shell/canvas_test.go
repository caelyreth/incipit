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

// The canvas comes with a picker beside the sheet in front, showing what
// the sheet list shows, takes the window from the panes, and gives it back
// as it was.
func TestTheCanvasComesWithAPickerBesideTheSheetInFront(t *testing.T) {
	state, _, tester := onCanvas(t)
	if n := len(state.canvas.columns); n != 2 || state.canvas.columns[0].picker == nil ||
		state.canvas.columns[1].sheet != "1" {
		t.Fatalf("The strip should hold a picker, then the sheet in front, but it holds %+v.", state.canvas.columns)
	}
	picker, ok := tester.Find("Sheet picker")
	sheet, ok2 := tester.Find("Bring Untitled in front")
	if !ok || !ok2 || picker.X > sheet.X || sheet.X+sheet.W > 1260 {
		t.Fatalf("The picker and the sheet should both show, side by side, but they are at %+v and %+v.", picker, sheet)
	}
	if !tester.HasText("Draft") {
		t.Fatalf("The picker should list the place's sheets, but the canvas shows %#v.", tester.Texts())
	}
	if _, ok := tester.Find("Groups"); ok {
		t.Fatal("The library should give way to the canvas, but it stays beside it.")
	}
	if err := tester.Click("Page mode"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.CanvasOpen() || !state.SidebarVisible() {
		t.Fatal("Leaving the canvas should bring the panes back as they were.")
	}
}

// The mode is the window's top choice: on the canvas the window's controls
// add the canvas's own, the panes' title line goes with the panes, and
// there is no zoom and no line of its own over the strip.
func TestTheCanvasIsAModeOfTheWindow(t *testing.T) {
	_, _, tester := onCanvas(t)
	if _, ok := tester.Find("New sheet"); !ok {
		t.Fatal("The window's controls should offer a new sheet on the canvas.")
	}
	for _, label := range []string{"Toggle library", "Toggle sheet list", "End search", "Zoom in", "Light sheets"} {
		if _, ok := tester.Find(label); ok {
			t.Fatalf("The canvas should not show %q.", label)
		}
	}
}

// The window's controls switch the mode, and offer the canvas's own only
// while it shows.
func TestTheModeSwitchesFromTheWindowsControls(t *testing.T) {
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if _, ok := tester.Find("Canvas actions"); ok {
		t.Fatal("The canvas's controls should not show on the page.")
	}
	if err := tester.Click("Canvas mode"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !state.CanvasOpen() {
		t.Fatal("Choosing the canvas should lay the notebook out on it.")
	}
	if err := tester.Click("New sheet"); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(library.calls, func(call string) bool { return strings.HasPrefix(call, "new sheet ") }) {
		t.Fatalf("New sheet should make one, but the calls are %v.", library.calls)
	}
}

// A sheet clicked in a picker takes the picker's place, the picker
// turning into its page, and the canvas stays.
func TestASheetChosenInAPickerTakesItsPlace(t *testing.T) {
	state, library, tester := onCanvas(t)
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
	if cols := state.canvas.columns; len(cols) != 2 || cols[0].sheet != "8" || cols[1].sheet != "1" {
		t.Fatalf("The sheet should stand where the picker stood, but the strip holds %+v.", cols)
	}
}

// A sheet opened alongside in a picker comes in after it, and the picker
// stays for the next.
func TestASheetOpenedAlongsideComesInBesideThePicker(t *testing.T) {
	state, library, tester := onCanvas(t)
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
	cols := state.canvas.columns
	if len(cols) != 3 || cols[0].picker == nil || cols[1].sheet != "8" || cols[2].sheet != "1" {
		t.Fatalf("The sheet should come in after the picker, which stays, but the strip holds %+v.", cols)
	}
}

// New picker makes another after the column in front; a picker closes.
func TestPickersAreMadeAndClosed(t *testing.T) {
	state, _, tester := onCanvas(t)
	if err := tester.Click("New picker"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	cols := state.canvas.columns
	if len(cols) != 3 || cols[2].picker == nil {
		t.Fatalf("A new picker should stand after the sheet in front, but the strip holds %+v.", cols)
	}
	state.closePicker(cols[0].picker.id)
	tester.Frame()
	if cols := state.canvas.columns; len(cols) != 2 || cols[0].sheet != "1" {
		t.Fatalf("Closing a picker should take it off the strip, but the strip holds %+v.", cols)
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
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if moved := last - state.canvas.strip.X; moved < 200 {
		t.Fatalf("A drag on a head should move the strip with it, but it moved %v.", moved)
	}
	tester.Release(x+300, y)
	tester.Frame()
	settle(tester)
	rest := state.canvas.strip.X
	settle(tester)
	if state.canvas.strip.X != rest {
		t.Fatalf("The strip should come to rest, but it went on from %v to %v.", rest, state.canvas.strip.X)
	}
	if state.ActivePage != "10" {
		t.Fatalf("A drag on a head should bring nothing in front, but %s came.", state.ActivePage)
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
		state.Open = []Page{{ID: "7", Title: "Chapter"}, {ID: "8", Title: "Draft"}}
		state.ActivePage = "7"
		tester := ui.NewTester(state.View, int(window.width), 850)
		tester.Frame()
		state.ToggleCanvas()
		tester.Frame()
		settle(tester)
		inner := window.width - 2*theme.DefaultLayout.ContentGutter
		want := (inner - columnGap*(window.n-1)) / window.n
		// The picker, then the two sheets.
		if spans := state.canvas.spans; len(spans) != 3 || abs(spans[1].w-want) > 1 || abs(spans[2].w-want) > 1 {
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
	for _, id := range []string{"7", "8", "9", "10"} {
		state.Open = append(state.Open, Page{ID: id, Title: "Sheet " + id})
	}
	tester.Frame()
	settle(tester)
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
	for _, id := range []string{"7", "8", "9", "10"} {
		state.Open = append(state.Open, Page{ID: id, Title: "Sheet " + id})
	}
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

// The page and the canvas share what they both show: going to the canvas,
// the sheet in front moves from the page into its column, and the sheet
// list out into the picker beside it; going back, both move back. The
// page's layout stays laid out beneath the canvas as it comes, and the
// layout gone to is built a build before what moves to it.
func TestThePageAndTheCanvasShareTheirSheetAndList(t *testing.T) {
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
	moving := func(key string) bool { return state.morph.Moving(key, tester.Now()) }
	state.ToggleCanvas()
	step(3)
	if !moving("sheet:1") || !moving("list") {
		t.Fatal("Going to the canvas, the sheet in front and the list should move into their columns.")
	}
	if state.lastSidebarWidth != listWidth {
		t.Fatalf("The panes should stay as they were under the canvas, but they went from %v to %v.",
			listWidth, state.lastSidebarWidth)
	}
	if n := strings.Count(strings.Join(tester.Texts(), "\n"), "Body of 1"); n != 1 {
		t.Fatalf("The sheet in front should be drawn once, in one place, but it is drawn %d times.", n)
	}
	settle(tester)
	if moving("sheet:1") || moving("list") {
		t.Fatal("The sheet and the picker should rest in their columns once there.")
	}
	if _, ok := tester.Find("Groups"); ok {
		t.Fatal("Under the whole canvas, the page mode's layout should not be built.")
	}
	state.SetMode(ModePage)
	step(3)
	if !moving("sheet:1") || !moving("list") {
		t.Fatal("Going back, the sheet and the list should move back to the page and the panes.")
	}
	settle(tester)
	if moving("sheet:1") || moving("list") || !tester.HasText("Body of 1") {
		t.Fatal("The page and the list should rest in their places once there.")
	}
}
