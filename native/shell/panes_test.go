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

// The place's band heads the list at every scale, and keeps clear of
// the window's controls where the list is leftmost.
func TestThePlaceBandHeadsTheListAtEveryScale(t *testing.T) {
	for _, scale := range []float32{12, 16, 24} {
		state, _ := libraryState()
		state.Settings.UIFontSize = scale
		state.Go(Place{Kind: PlaceGroup, Group: "2"})
		state.ShowPanes(false, true)
		tester := ui.NewTester(state.View, 1260, 500)
		settle(tester)
		if got, want := placeBandHeight(scale), float32(56)*scale/theme.DefaultUIFontSize; abs(got-want) > 0.001 {
			t.Fatalf("scale %v band height = %v, want %v", scale, got, want)
		}
		if !tester.HasText("Novel") {
			t.Fatalf("scale %v: the band lacks the place: %#v", scale, tester.Texts())
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

func abs(x float32) float32 { return max(x, -x) }
