package shell

import (
	"testing"
	"time"

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
	{"editor only", true, true, false, false},
	{"sheets from editor", false, false, false, true},
	{"editor from sheets", false, true, false, false},
	{"all from editor", false, false, true, true},
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

// TestTheTabsStayAsPanesMove watches the first tab while the panes move:
// it slides with the panes' edge, one way, and never folds away and comes
// back, as it would if the tab line were made anew.
func TestTheTabsStayAsPanesMove(t *testing.T) {
	for _, move := range paneMoves {
		state, _ := libraryState()
		state.ShowPanes(move.library, move.sheets)
		tester := ui.NewTester(state.View, 1260, 400)
		settle(tester)
		from, _ := tester.Find("Untitled")
		state.ShowPanes(move.toLibrary, move.toSheets)
		settle(tester)
		to, _ := tester.Find("Untitled")
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
		}
	}
}
