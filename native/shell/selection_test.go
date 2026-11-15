package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

type selectionLibrary struct {
	*fakeLibrary
	state *State
}

func (l selectionLibrary) OpenSheet(id string, alongside bool) {
	l.fakeLibrary.OpenSheet(id, alongside)
	l.state.ActivePage = id
}

func TestLibrarySelectionGlidesBetweenGroups(t *testing.T) {
	state, library := libraryState()
	library.extraGroups = 2
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	from := selectionRow(t, tester, "Group 1")
	to := selectionRow(t, tester, "Group 2")
	state.Go(Place{Kind: PlaceGroup, Group: "extra-0"})
	tester.Frame()
	settle(tester)
	state.Go(Place{Kind: PlaceGroup, Group: "extra-1"})
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	assertSelectionBetween(t, tester, state, from, to)
}

func TestLibrarySelectionGlidesAfterClick(t *testing.T) {
	state, library := libraryState()
	library.extraGroups = 2
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	from := selectionRow(t, tester, "Group 1")
	to := selectionRow(t, tester, "Group 2")
	state.Go(Place{Kind: PlaceGroup, Group: "extra-0"})
	tester.Frame()
	settle(tester)
	tester.ClickAt(to.X+to.W/2, to.Y+to.H/2)
	if state.Place() != (Place{Kind: PlaceGroup, Group: "extra-1"}) {
		t.Fatalf("click did not choose group: %v", state.Place())
	}
	tester.Advance(48 * time.Millisecond)
	assertSelectionBetween(t, tester, state, from, to)
}

func TestSheetSelectionGlidesBetweenRows(t *testing.T) {
	state, library := libraryState()
	library.extraSheets = 2
	state.ActivePage = "7"
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	from := selectionRow(t, tester, "Chapter")
	to := selectionRow(t, tester, "Sheet 2")
	state.ActivePage = "extra-1"
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	assertSelectionBetween(t, tester, state, from, to)
}

func TestSheetSelectionGlidesAfterClick(t *testing.T) {
	state, library := libraryState()
	library.extraSheets = 2
	state.Library = selectionLibrary{fakeLibrary: library, state: state}
	state.ActivePage = "7"
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	from := selectionRow(t, tester, "Chapter")
	to := selectionRow(t, tester, "Sheet 2")
	tester.Press(to.X+to.W/2, to.Y+to.H/2)
	tester.Advance(80 * time.Millisecond)
	tester.Release(to.X+to.W/2, to.Y+to.H/2)
	if state.ActivePage != "extra-1" {
		t.Fatalf("click did not open sheet: %q", state.ActivePage)
	}
	tester.Advance(48 * time.Millisecond)
	assertSelectionBetween(t, tester, state, from, to)
}

func TestSheetSelectionGlidesBetweenDifferentHeights(t *testing.T) {
	state, library := libraryState()
	library.entries = []GroupEntry{
		{Kind: EntrySheet, ID: "short", Sheet: SheetItem{ID: "short", Title: "Short"}},
		{Kind: EntrySheet, ID: "middle", Sheet: SheetItem{ID: "middle", Title: "Middle"}},
		{Kind: EntrySheet, ID: "long", Sheet: SheetItem{ID: "long", Title: "Long", Excerpt: "A longer sheet with an excerpt."}},
	}
	state.Library = selectionLibrary{fakeLibrary: library, state: state}
	state.ActivePage = "short"
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	from := selectionRow(t, tester, "Short")
	to := selectionRow(t, tester, "Long")
	if from.H == to.H {
		t.Fatal("fixture rows have the same height")
	}
	tester.Press(to.X+to.W/2, to.Y+to.H/2)
	tester.Advance(80 * time.Millisecond)
	tester.Release(to.X+to.W/2, to.Y+to.H/2)
	tester.Advance(48 * time.Millisecond)
	assertSelectionBetween(t, tester, state, from, to)
}

func TestSheetSelectionHonorsReducedMotion(t *testing.T) {
	state, library := libraryState()
	library.extraSheets = 2
	state.Library = selectionLibrary{fakeLibrary: library, state: state}
	state.ActivePage = "7"
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	settle(tester)
	to := selectionRow(t, tester, "Sheet 2")
	tester.ClickAt(to.X+to.W/2, to.Y+to.H/2)
	if state.ActivePage != "extra-1" {
		t.Fatalf("click did not open sheet: %q", state.ActivePage)
	}
	fill := state.Settings.Semantic().Surface.Chosen
	pixel := tester.Image().RGBAAt(int(to.X+4), int(to.Y+to.H/2))
	if pixel.R != fill.R || pixel.G != fill.G || pixel.B != fill.B {
		t.Fatalf("highlight did not arrive immediately: %v, want %v", pixel, fill)
	}
}

func selectionRow(t *testing.T, tester *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tester.Find(label)
	if !ok {
		t.Fatalf("%s not found", label)
	}
	return r
}

func assertSelectionBetween(t *testing.T, tester *ui.Tester, state *State, from, to ui.Rect) {
	t.Helper()
	fill := state.Settings.Semantic().Surface.Chosen
	x := int(from.X + 4)
	top := int(from.Y + from.H)
	bottom := int(to.Y)
	for y := top; y < bottom; y++ {
		pixel := tester.Image().RGBAAt(x, y)
		if pixel.R == fill.R && pixel.G == fill.G && pixel.B == fill.B {
			return
		}
	}
	t.Fatalf("selection did not glide between rows: from %v to %v", from, to)
}
