package shell

import (
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// onDesk is the shell with its panes put away, the desk taking the
// window, its sheets opened and placed as the window does.
func onDesk(t *testing.T) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	library.desk = state
	state.ShowPanes(false, false)
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	return state, library, tester
}

// openOnDesk opens page as the window does: beside the sheet in front, or
// in its place, and in front.
func openOnDesk(state *State, page Page, inPlace bool) {
	at := slices.IndexFunc(state.Open, func(p Page) bool { return p.ID == page.ID })
	if at < 0 {
		front := slices.IndexFunc(state.Open, func(p Page) bool { return p.ID == state.ActivePage })
		state.Open = slices.Insert(state.Open, front+1, page)
		if inPlace && front >= 0 {
			state.Open = slices.Delete(state.Open, front, front+1)
		}
	}
	state.ActivePage = page.ID
}

// joining opens sheets one after another, each beside the one before.
func joining(state *State, tester *ui.Tester, pages ...Page) {
	for _, page := range pages {
		openOnDesk(state, page, false)
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

// gapAfter is where, across the window, the line after the strip's i-th
// column stands.
func gapAfter(state *State, i int) float32 {
	sp := state.desk.spans[i]
	return state.desk.viewport.X - state.desk.strip.X + sp.End() + columnGap/2
}

// closeColumn presses close in the column labelled column, its head shown
// with the pointer on it.
func closeColumn(tester *ui.Tester, column, close string) error {
	if col, ok := tester.Find(column); ok {
		tester.Move(col.X+col.W/2, col.Y+col.H/2)
		settle(tester)
	}
	return tester.Click(close)
}

// closeSheet closes the sheet titled title.
func closeSheet(tester *ui.Tester, title string) error {
	return closeColumn(tester, title, "Close "+title)
}

// columnsOf names the desk's columns: a sheet by its id, a picker as
// "picker".
func columnsOf(state *State) []string {
	var out []string
	for _, col := range state.deskColumns() {
		if col.picker != nil {
			out = append(out, "picker")
		} else {
			out = append(out, col.sheet)
		}
	}
	return out
}

// One sheet alone fills the desk, its text at reading measure, centred.
func TestOneSheetFillsTheDesk(t *testing.T) {
	state, _, tester := onDesk(t)
	var frame SheetFrame
	state.Sheet = func(c *ui.Context, _ string, f SheetFrame) { frame = f; ui.Text(c, "Body") }
	settle(tester)
	if spans := state.desk.spans; len(spans) != 1 || abs(spans[0].W-state.desk.viewport.W) > 1 {
		t.Fatalf("One sheet should fill the desk, %v wide, but the strip holds %+v.", state.desk.viewport.W, spans)
	}
	if want := frame.Prose.Inset(frame.LayoutWidth); frame.Inset != want || want <= 0 {
		t.Fatalf("Its text should be centred at reading measure, inset %v, but it is inset %v.", want, frame.Inset)
	}
}

// The desk stands beside the panes, taking the room they leave.
func TestTheDeskStandsBesideThePanes(t *testing.T) {
	state, _ := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if !state.SidebarVisible() || !state.SheetListVisible() {
		t.Fatal("The test needs the panes open.")
	}
	if x := state.desk.viewport.X; abs(x-state.lastSidebarWidth) > 1 {
		t.Fatalf("The desk should begin where the panes end, at %v, but it begins at %v.", state.lastSidebarWidth, x)
	}
	if spans := state.desk.spans; len(spans) != 1 || abs(spans[0].W-(1260-state.lastSidebarWidth)) > 1 {
		t.Fatalf("Its one sheet should take the room the panes leave, but the strip holds %+v.", spans)
	}
}

// Two sheets share the desk by halves; more go two or three to a window,
// three where a third still holds a page; a picker is the sheet list's
// width.
func TestSheetsShareTheDeskByHalvesOrThirds(t *testing.T) {
	for _, window := range []struct {
		width  float32
		sheets []string
		n      float32
	}{{1260, []string{"7"}, 2}, {2600, []string{"7"}, 2}, {2600, []string{"7", "8"}, 3}} {
		state, library := libraryState()
		library.desk = state
		state.ShowPanes(false, false)
		tester := ui.NewTester(state.View, int(window.width), 850)
		settle(tester)
		joining(state, tester, sheetPages(window.sheets...)...)
		state.NewPicker()
		settle(tester)
		want := (window.width - columnGap*(window.n-1)) / window.n
		spans := state.desk.spans
		if len(spans) != len(window.sheets)+2 {
			t.Fatalf("At %v across, the strip should hold the sheets and a picker, but it holds %+v.", window.width, spans)
		}
		for _, sp := range spans[:len(window.sheets)+1] {
			if abs(sp.W-want) > 1 {
				t.Fatalf("At %v across with %d sheets, each should be %v wide, but the strip holds %+v.",
					window.width, len(window.sheets)+1, want, spans)
			}
		}
		picker, ok := tester.Find("Sheet picker")
		if !ok || abs(picker.W-theme.DefaultLayout.SheetList) > 1 {
			t.Fatalf("A picker should be as wide as the sheet list, but it is %+v.", picker)
		}
	}
}

// The writing actions make a picker after the column in front.
func TestTheWritingActionsMakeAPicker(t *testing.T) {
	state, _, tester := onDesk(t)
	if err := tester.Click("New picker"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "picker"}) {
		t.Fatalf("New picker should make one after the sheet in front, but the desk holds %v.", cols)
	}
}

// Right-clicked on the line between columns, the desk makes a picker
// there.
func TestRightClickingTheDeskMakesAPickerThere(t *testing.T) {
	state, _, tester := onDesk(t)
	joining(state, tester, Page{ID: "7", Title: "Chapter"})
	if len(state.desk.spans) != 2 {
		t.Fatalf("Both sheets should show, but the desk shows %#v.", tester.Texts())
	}
	tester.RightClickAt(gapAfter(state, 0), 400)
	if err := tester.ChooseMenuItem("New Picker"); err != nil {
		t.Fatalf("The line between columns should offer a new picker: %v (menu %v)", err, tester.Menu())
	}
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "picker", "7"}) {
		t.Fatalf("The picker should stand where the desk was right-clicked, but the desk holds %v.", cols)
	}
}

// A sheet chosen in a picker takes the picker's place; one opened
// alongside comes in after it, and the picker stays.
func TestASheetChosenInAPickerStandsWhereThePickerDoes(t *testing.T) {
	for _, tc := range []struct {
		alongside bool
		want      []string
	}{{false, []string{"1", "8"}}, {true, []string{"1", "picker", "8"}}} {
		state, library, tester := onDesk(t)
		state.NewPicker()
		settle(tester)
		click := tester.Click
		if tc.alongside {
			click = func(s string) error { return tester.ClickWith(ui.Alt, s) }
		}
		if err := click("Draft"); err != nil {
			t.Fatal(err)
		}
		settle(tester)
		if !slices.Contains(library.calls, "place 8 after 1") {
			t.Fatalf("The picker should place its sheet where it stands, but the calls are %v.", library.calls)
		}
		if cols := columnsOf(state); !slices.Equal(cols, tc.want) {
			t.Fatalf("Alongside %v, the desk should hold %v, but it holds %v.", tc.alongside, tc.want, cols)
		}
	}
}

// A picker whose sheet closes stays where it stood.
func TestAPickerStaysWhereItStoodAsItsSheetCloses(t *testing.T) {
	state, _, tester := onDesk(t)
	joining(state, tester, sheetPages("7", "8")...)
	state.ActivePage = "7"
	tester.Frame()
	state.NewPicker()
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "7", "picker", "8"}) {
		t.Fatalf("The test needs the picker after 7, but the desk holds %v.", cols)
	}
	state.Open = slices.DeleteFunc(state.Open, func(p Page) bool { return p.ID == "7" })
	state.ActivePage = "8"
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1", "picker", "8"}) {
		t.Fatalf("The picker should stay after the sheet before it, but the desk holds %v.", cols)
	}
}

// A picker closes.
func TestAPickerCloses(t *testing.T) {
	state, _, tester := onDesk(t)
	state.NewPicker()
	settle(tester)
	if err := closeColumn(tester, "Sheet picker", "Close picker"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"1"}) {
		t.Fatalf("Closing a picker should take it off the desk, but it holds %v.", cols)
	}
}

// A sheet opened in place takes the place of the one in front.
func TestASheetOpenedInPlaceTakesTheFrontsPlace(t *testing.T) {
	state, library, tester := onDesk(t)
	joining(state, tester, sheetPages("7")...)
	state.ActivePage = "1"
	tester.Frame()
	library.OpenSheet("8", false)
	settle(tester)
	if cols := columnsOf(state); !slices.Equal(cols, []string{"8", "7"}) {
		t.Fatalf("The sheet should take the place of the one in front, but the desk holds %v.", cols)
	}
}

// The desk holds its sheets at once, each written in where it stands; the
// pointer coming onto one brings it in front, and its head closes it.
func TestTheDeskHoldsItsSheetsAtOnce(t *testing.T) {
	state, _, tester := onDesk(t)
	focused := map[string]bool{}
	state.Sheet = func(c *ui.Context, id string, frame SheetFrame) {
		focused[id] = frame.Focus
		ui.Text(c, "Body of "+id)
	}
	var selected, closed []string
	state.SelectPage = func(id string) { selected = append(selected, id); state.ActivePage = id }
	state.ClosePageID = func(id string) {
		closed = append(closed, id)
		at := slices.IndexFunc(state.Open, func(p Page) bool { return p.ID == id })
		state.Open = slices.Delete(state.Open, at, at+1)
		if state.ActivePage == id {
			state.ActivePage = ""
			if len(state.Open) > 0 {
				state.ActivePage = state.Open[min(at, len(state.Open)-1)].ID
			}
		}
	}
	joining(state, tester, Page{ID: "7", Title: "Chapter"}, Page{ID: "8", Title: "Draft"})
	state.ActivePage = "7"
	tester.Frame()
	settle(tester)
	if !tester.HasText("Body of 1") || !tester.HasText("Body of 7") || !tester.HasText("Body of 8") {
		t.Fatalf("Every sheet on the desk should stand there, but it shows %#v.", tester.Texts())
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
	if err := closeSheet(tester, "Draft"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Equal(closed, []string{"8"}) || state.ActivePage != "7" {
		t.Fatalf("Closing the sheet in front should bring the one beside it in front, but %v closed and %q is in front.",
			closed, state.ActivePage)
	}
}

// Search runs in the sheet list, which shows for it.
func TestSearchShowsTheSheetList(t *testing.T) {
	state, _, tester := onDesk(t)
	state.Search()
	tester.Frame()
	settle(tester)
	if !state.SheetListVisible() || !state.list.searching {
		t.Fatal("Search should show the sheet list and search in it.")
	}
}

// The strip goes to the sheet that comes in front, and a drag on the line
// between columns moves it along, coming to rest at a column.
func TestTheStripFollowsTheFrontAndPansByDrag(t *testing.T) {
	state, _, tester := onDesk(t)
	state.Sheet = func(c *ui.Context, id string, _ SheetFrame) { ui.Text(c, "Body of "+id) }
	state.SelectPage = func(id string) { state.ActivePage = id }
	joining(state, tester, sheetPages("7", "8", "9", "10")...)
	state.ActivePage = "7"
	tester.Frame()
	settle(tester)
	before := state.desk.strip.X
	state.ActivePage = "10"
	tester.Frame()
	settle(tester)
	last := state.desk.strip.X
	if last <= before {
		t.Fatalf("The strip should go to the sheet in front, but it stayed at %v.", last)
	}
	x, y := gapAfter(state, 3), float32(400)
	tester.Press(x, y)
	for i := 1; i <= 10; i++ {
		tester.Move(x+float32(i)*30, y)
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if moved := last - state.desk.strip.X; moved < 200 {
		t.Fatalf("A drag on the strip should move it along, but it moved %v.", moved)
	}
	tester.Release(x+300, y)
	tester.Frame()
	settle(tester)
	rest := state.desk.strip.X
	settle(tester)
	if state.desk.strip.X != rest {
		t.Fatalf("The strip should come to rest, but it went on from %v to %v.", rest, state.desk.strip.X)
	}
}

// The strip goes where it is scrolled, and pins to a column's start only
// once it comes to rest near one, the fingers lifted; far from any, it
// stays where it stopped.
func TestTheStripPinsToAColumnItComesToRestNear(t *testing.T) {
	state, _, tester := onDesk(t)
	joining(state, tester, sheetPages("7", "8", "9", "10")...)
	state.desk.strip.ScrollTo(0, 0, ui.Animation{Duration: 1})
	settle(tester)
	rests := stage.Rests(state.desk.spans, state.desk.strip.MaxX)
	scroll := func(dx float32) {
		tester.Scroll(600, 400, dx, 0)
		tester.Frame()
		tester.Scroll(600, 400, 0, 0) // the fingers lift
		settle(tester)
	}
	scroll(rests[1] - 30)
	if abs(state.desk.strip.X-rests[1]) > 1 {
		t.Fatalf("Coming to rest 30 short of a column, the strip should pin to it at %v, but it is at %v.",
			rests[1], state.desk.strip.X)
	}
	mid := (rests[1] + rests[2]) / 2
	scroll(mid - state.desk.strip.X)
	if abs(state.desk.strip.X-mid) > 1 {
		t.Fatalf("Coming to rest far from any column, the strip should stay at %v, but it went to %v.",
			mid, state.desk.strip.X)
	}
}

// The strip stretches past its start, as the lists do, and comes back.
func TestTheStripStretchesPastItsEnds(t *testing.T) {
	state, _, tester := onDesk(t)
	state.desk.strip.ScrollTo(0, 0, ui.Animation{Duration: 1})
	settle(tester)
	tester.Scroll(600, 400, -80, 0)
	tester.Frame()
	if x, _ := state.desk.over.Offset(); x == 0 {
		t.Fatal("Scrolled past its start, the strip should stretch.")
	}
	tester.Scroll(600, 400, 0, 0)
	settle(tester)
	if x, _ := state.desk.over.Offset(); x != 0 || state.desk.strip.X != 0 {
		t.Fatalf("Let go, the strip should come back to its start, but it is %v past it at %v.", x, state.desk.strip.X)
	}
}

// The scrubber goes to a column pressed in it, and a drag on it moves the
// strip, coming to rest at a column.
func TestTheScrubberMovesTheStrip(t *testing.T) {
	state, _, tester := onDesk(t)
	joining(state, tester, sheetPages("7", "8", "9", "10")...)
	state.ActivePage = "7"
	tester.Frame()
	settle(tester)
	// The scrubber shows while the strip moves, as it did coming to the
	// sheet in front.
	state.desk.strip.X = 1
	tester.Frame()
	bar, ok := tester.Find("Desk overview")
	if !ok {
		t.Fatal("The scrubber should show while the strip moves.")
	}
	spans := state.desk.spans
	f := (bar.W - 8) / spans[len(spans)-1].End()
	tester.Press(bar.X+4+(spans[3].X+10)*f, bar.Y+bar.H/2)
	tester.Release(bar.X+4+(spans[3].X+10)*f, bar.Y+bar.H/2)
	tester.Frame()
	settle(tester)
	if want := stage.Rests(state.desk.spans, state.desk.strip.MaxX)[3]; abs(state.desk.strip.X-want) > 1 {
		t.Fatalf("A column pressed in the scrubber should come to the strip's start at %v, but the strip is at %v.",
			want, state.desk.strip.X)
	}
	x, y := bar.X+bar.W-6, bar.Y+bar.H/2
	before := state.desk.strip.X
	tester.Press(x, y)
	for i := 1; i <= 5; i++ {
		tester.Move(x-float32(i)*8, y)
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	tester.Release(x-40, y)
	tester.Frame()
	settle(tester)
	if rest := state.desk.strip.X; rest >= before {
		t.Fatalf("A drag back on the scrubber should bring the strip back, but it went from %v to %v.", before, rest)
	}
}

// A column brought into view at the strip's end comes whole into the
// window.
func TestAColumnBroughtIntoViewComesWhole(t *testing.T) {
	state, _, tester := onDesk(t)
	joining(state, tester, sheetPages("7", "8", "9")...)
	state.NewPicker()
	settle(tester)
	picker, ok := tester.Find("Sheet picker")
	if !ok {
		t.Fatal("The new picker should show.")
	}
	if right := picker.X + picker.W; right > 1260+1 || picker.X < 0 {
		t.Fatalf("The picker should stand whole in the window, but it spans %v to %v of 1260.", picker.X, right)
	}
}

// A column closing at the strip's end closes up where it is: the strip is
// not cut back under it at once, but eases back after it.
func TestClosingAColumnEasesTheStripBack(t *testing.T) {
	state, _, tester := onDesk(t)
	state.Appearance.Motion = true
	joining(state, tester, sheetPages("7", "8", "9")...)
	state.NewPicker()
	settle(tester)
	before := state.desk.strip.X
	if before <= 0 {
		t.Fatal("The test needs the strip scrolled along.")
	}
	if err := closeColumn(tester, "Sheet picker", "Close picker"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	if jump := before - state.desk.strip.X; jump > 40 {
		t.Fatalf("Closing a column, the strip should ease back, but it jumped %v at once.", jump)
	}
	settle(tester)
	if state.desk.strip.X >= before {
		t.Fatalf("Once it has closed, the strip should have come back, but it stands at %v from %v.",
			state.desk.strip.X, before)
	}
}

// The column in front follows the pointer: moved onto a sheet's column,
// it brings that sheet in front and gives its page the keyboard, the strip
// staying where it is, unless a field of the shell is being written in;
// moved onto a picker, it brings the picker in front, the sheet in front
// staying for the writing actions. The strip moving under a pointer at
// rest brings nothing.
func TestThePointerBringsTheColumnUnderItInFront(t *testing.T) {
	state, _, tester := onDesk(t)
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
	strip := state.desk.strip.X
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
	if state.desk.strip.X != strip {
		t.Fatalf("A sheet the pointer brings in front is where it is: the strip should stay at %v, but went to %v.",
			strip, state.desk.strip.X)
	}
	// The strip moved under the pointer at rest brings nothing in front.
	state.stripAt(state.desk.strip.MaxX)
	tester.Frame()
	settle(tester)
	if state.ActivePage != "7" || state.desk.at != "sheet-7" {
		t.Fatalf("The strip moving under the pointer should bring nothing in front, but %q came.", state.desk.at)
	}
	search, ok := tester.Find("Search sheets")
	if !ok {
		t.Fatal("The picker's search should show.")
	}
	tester.Move(search.X+8, search.Y+search.H+40)
	tester.Frame()
	settle(tester)
	if state.desk.at != "picker-1" || state.ActivePage != "7" {
		t.Fatalf("The pointer moved onto a picker should bring it in front, the sheet in front staying, but %q is in front.",
			state.desk.at)
	}
	// Writing in a field of the shell, the keyboard stays there.
	state.list.focus = true
	claimed = nil
	state.stripAt(0)
	settle(tester)
	if body, ok = tester.Find("Body of 1"); !ok {
		t.Fatalf("The first sheet should show, but the strip shows %#v.", tester.Texts())
	}
	tester.Move(body.X+body.W/2, body.Y+body.H/2)
	tester.Frame()
	settle(tester)
	if state.ActivePage != "1" || len(claimed) != 0 {
		t.Fatalf("Writing in a field, the pointer should bring the sheet in front and leave the keyboard, but %q is in front (claimed %v).",
			state.ActivePage, claimed)
	}
}

// The title line is the chrome's: a column under the controls holds what
// it holds under the line, whole, and only its head on it, clear of them,
// centred on the traffic lights as the controls are.
func TestTheTitleLineHoldsOnlyTheHeadsClearOfTheControls(t *testing.T) {
	state, _, tester := onDesk(t)
	joining(state, tester, sheetPages("7", "8")...)
	state.newPickerAt(len(state.deskColumns()))
	settle(tester)
	state.stripAt(state.desk.strip.MaxX)
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

// The divider and the pill's end bound one square close button, with
// equal room on its two sides, at every UI size.
func TestTheSheetHeadCentersCloseBetweenTheDividerAndEdge(t *testing.T) {
	for _, size := range []float32{12, 16, 24} {
		state, _ := libraryState()
		state.Settings.UIFontSize = size
		semantic := state.Settings.Semantic()
		tester := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(state.Settings.MyGoTheme())
			ui.Column(c).Fill().Children(func() {
				state.sheetHead(c, semantic, Page{ID: "1", Title: "A place to begin"},
					size/theme.DefaultUIFontSize, 0, 0, 1, false)
			})
		}, 520, 80)
		tester.SetTitleBar(ui.TitleBar{Height: 64})
		tester.Frame()
		pill, hasPill := tester.Find("Column")
		info, hasInfo := tester.Find("Sheet")
		close, hasClose := tester.Find("Close A place to begin")
		if !hasPill || !hasInfo || !hasClose {
			t.Fatal("The sheet's title, pill and close button should show.")
		}
		// The divider is one DIP wide, directly after the title group.
		before := close.X - (info.X + info.W + 1)
		after := pill.X + pill.W - (close.X + close.W)
		if abs(before-after) > 0.5 || abs(close.W-close.H) > 0.5 {
			t.Fatalf("At UI size %v, close should be square with equal room beside the divider and the pill's end, but it is %v with gaps %v and %v.",
				size, close, before, after)
		}
	}
}
