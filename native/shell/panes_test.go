package shell

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// paneMoves are the moves between ⌘1, ⌘2 and ⌘3 and back.
var paneMoves = []struct {
	name                string
	library, sheets     bool
	toLibrary, toSheets bool
}{
	{"library closes", true, true, false, true},
	{"library opens", false, true, true, true},
	{"page only", true, true, false, false},
	{"sheets from page", false, false, false, true},
	{"page from sheets", false, true, false, false},
	{"all from page", false, false, true, true},
}

func TestPlaceTitleKeepsItsSharedFlightAtDifferentScales(t *testing.T) {
	for _, scale := range []float32{12, 16, 24} {
		state, _ := libraryState()
		state.Settings.UIFontSize = scale
		state.Go(Place{Kind: PlaceGroup, Group: "2"})
		state.ShowPanes(true, true)
		tester := ui.NewTester(state.View, 1260, 500)
		settle(tester)
		if got, want := placeBandHeight(scale), float32(56)*scale/theme.DefaultUIFontSize; abs(got-want) > 0.001 {
			t.Fatalf("scale %v band height = %v, want %v", scale, got, want)
		}
		state.ShowPanes(false, true)
		tester.Frame()
		tester.Advance(60 * time.Millisecond)
		if !state.flying() || state.listDock <= 0 || state.listDock >= 1 {
			t.Fatalf("scale %v did not fly: dock=%v", scale, state.listDock)
		}
		state.ShowPanes(true, true)
		tester.Frame()
		settle(tester)
		if state.flying() || state.listDock > 0.001 || !tester.HasText("Novel") {
			t.Fatalf("scale %v did not settle after reversal: dock=%v", scale, state.listDock)
		}
	}
}

// TestTheSelectedSheetStaysHighlightedAsPanesMove watches the open sheet's
// row while the panes around it open and close: its highlight moves with
// the row and never goes out, as it would if the list were made anew.
func TestTheSelectedSheetStaysHighlightedAsPanesMove(t *testing.T) {
	for _, move := range paneMoves {
		if !move.sheets || !move.toSheets {
			continue
		}
		state, _ := libraryState()
		state.ActivePage = "8"
		state.ShowPanes(move.library, move.sheets)
		tester := ui.NewTester(state.View, 1260, 400)
		settle(tester)
		lit := highlight(tester)
		state.ShowPanes(move.toLibrary, move.toSheets)
		for frame := range 20 {
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
			if got := highlight(tester); got < lit-2 {
				t.Fatalf("%s: frame %d the highlight is at %d, settled at %d", move.name, frame, got, lit)
			}
		}
	}
}

// highlight is the brightness of the selected row's fill, just inside its
// edge.
func highlight(tester *ui.Tester) int {
	at, ok := tester.Find("Draft")
	if !ok {
		return 0
	}
	c := tester.Image().RGBAAt(int(at.X+3), int(at.Y+at.H/2))
	return int(c.R)
}

// TestTheTabsStayAsPanesMove watches the first tab and the library's
// toggle while the panes move: each slides one way to where it rests, and
// the tab never folds away and comes back, as it would if the tab line
// were made anew. A title bar line that met its least width, or a toggle
// that followed both a pane's edge and the moving title, would turn back.
func TestTheTabsStayAsPanesMove(t *testing.T) {
	for _, move := range paneMoves {
		state, _ := libraryState()
		state.ShowPanes(move.library, move.sheets)
		tester := ui.NewTester(state.View, 1260, 400)
		settle(tester)
		from, _ := tester.Find("Untitled")
		toggleFrom, _ := tester.Find("Toggle library")
		state.ShowPanes(move.toLibrary, move.toSheets)
		settle(tester)
		to, _ := tester.Find("Untitled")
		toggleTo, _ := tester.Find("Toggle library")
		toggleLast := toggleFrom
		state.ShowPanes(move.library, move.sheets)
		settle(tester)
		state.ShowPanes(move.toLibrary, move.toSheets)
		last := from
		for frame := range 30 {
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
			at, ok := tester.Find("Untitled")
			if !ok || at.W < from.W-1 {
				t.Fatalf("%s: frame %d the tab folded: %+v, was %+v", move.name, frame, at, from)
			}
			if (to.X-from.X)*(at.X-last.X) < -0.5 {
				t.Fatalf("%s: frame %d the tab turned back: %v after %v, going to %v", move.name, frame, at.X, last.X, to.X)
			}
			last = at
			toggle, _ := tester.Find("Toggle library")
			if (toggleTo.X-toggleFrom.X)*(toggle.X-toggleLast.X) < -0.5 || (toggleTo.X == toggleFrom.X && abs(toggle.X-toggleFrom.X) > 0.5) {
				t.Fatalf("%s: frame %d the library's toggle turned back: %v after %v, going to %v", move.name, frame, toggle.X, toggleLast.X, toggleTo.X)
			}
			toggleLast = toggle
		}
	}
}

// TestTheSheetListsToggleStaysInSight follows the sheet list's toggle as
// the library closes and opens: the place's title goes down to the list,
// and the toggle stays where it is, in sight all the way.
func TestTheSheetListsToggleStaysInSight(t *testing.T) {
	state, _ := libraryState()
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 400)
	settle(tester)
	for _, library := range []bool{false, true} {
		state.ShowPanes(library, true)
		for frame := range 20 {
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
			if at, ok := tester.Find("Hide sheet list"); !ok || at.W < 31 {
				t.Fatalf("library %v, frame %d: the toggle is at %+v", library, frame, at)
			}
		}
	}
}

func abs(x float32) float32 { return max(x, -x) }
