package shell

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// As the list, the field stands in one column beside the pill, read top
// down in its rank; the keys go along it so; and it goes back to the
// horizon as it came.
func TestTheListStandsTheFieldInOneColumnBesideThePill(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 3
	state.SetFieldQuery("$in:Novel")
	settle(tester)
	horizon := slices.Clone(state.field.drawn)
	if err := tester.Click("Show as List"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	settle(tester)
	pill, _ := tester.Find("Search")
	drawn := state.field.drawn
	for i, fr := range drawn {
		if fr.Rect.X != drawn[0].Rect.X {
			t.Fatalf("Every card should stand in the one column: %v.", drawn)
		}
		if i > 0 && fr.Rect.Y <= drawn[i-1].Rect.Y {
			t.Fatalf("The cards should stand top down in their rank: %v.", drawn)
		}
	}
	col := drawn[0].Rect.X + state.field.room.w/2
	if pill.X+pill.W >= col {
		t.Fatalf("The pill (%v) should stand before the column at %v.", pill, col)
	}
	// The name over the pill, centred on it; the stats under it.
	mark, _ := tester.Find("Neverland")
	if mark.Y+mark.H > pill.Y || abs(mark.X+mark.W/2-(pill.X+pill.W/2)) > 1 {
		t.Fatalf("As the list, the name should stand over the pill, centred: %v, %v.", mark, pill)
	}
	if !slices.Equal(state.field.order, state.field.ranked) {
		t.Fatalf("Read top down, the list's order should be its rank: %v, %v.", state.field.order, state.field.ranked)
	}
	if err := tester.Click("Show as Field"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	settle(tester)
	// Across, as they stood: how far over and under the pill depends on
	// whether it is open, which the press on the lane let go of.
	if !slices.EqualFunc(state.field.drawn, horizon, func(x, y stageFrame) bool { return x.ID == y.ID && x.Rect.X == y.Rect.X }) {
		t.Fatalf("Back on the horizon, the cards should stand as they did: %v, not %v.", horizon, state.field.drawn)
	}
	mark, _ = tester.Find("Neverland")
	pill, _ = tester.Find("Search")
	if mark.X+mark.W > pill.X || abs(mark.Y-pill.Y) > 1 {
		t.Fatalf("Back on the horizon, the name should stand before the pill: %v, %v.", mark, pill)
	}
}

// Going between the postures turns from wherever it is: pressed again on
// the way, nothing leaps, the pill, the name and the cards alike.
func TestGoingBetweenThePosturesTurnsFromWhereItIs(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.extraSheets = 3
	state.SetFieldQuery("$in:Novel")
	settle(tester)
	where := func() (pill, mark ui.Rect, cards []stageFrame) {
		pill, _ = tester.Find("Search")
		mark, _ = tester.Find("Neverland")
		return pill, mark, slices.Clone(state.field.drawn)
	}
	lastPill, lastMark, lastCards := where()
	step := func(i int) {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
		pill, mark, cards := where()
		if d := max(abs(pill.X-lastPill.X), abs(mark.X-lastMark.X), abs(mark.Y-lastMark.Y)); d > 60 {
			t.Fatalf("Frame %d: the pill or the name leapt %.0f (%v to %v, %v to %v).", i, d, lastPill, pill, lastMark, mark)
		}
		for j := range cards {
			if j < len(lastCards) && cards[j].ID == lastCards[j].ID {
				if d := max(abs(cards[j].Rect.X-lastCards[j].Rect.X), abs(cards[j].Rect.Y-lastCards[j].Rect.Y)); d > 80 {
					t.Fatalf("Frame %d: %s leapt %.0f.", i, cards[j].ID, d)
				}
			}
		}
		lastPill, lastMark, lastCards = pill, mark, cards
	}
	state.toggleList()
	for i := range 8 {
		step(i)
	}
	state.toggleList()
	for i := 8; i < 50; i++ {
		step(i)
	}
	if state.field.listed != 0 {
		t.Fatalf("Turned back, the field should come to rest on the horizon: %v.", state.field.listed)
	}
}
