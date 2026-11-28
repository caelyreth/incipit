package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func foundOf(query string, ids ...string) found {
	f := found{query: query}
	for _, id := range ids {
		f.cards = append(f.cards, card{id: id})
	}
	return f
}

// The columns are chosen as the query lands, again while what it brings
// measures itself, and then held: what grows moves only what is beyond it
// in its column; the cards changing choose anew, so one come and gone
// leaves the field as it stood.
func TestTheFieldHoldsTheColumnsItChoseForItsQuery(t *testing.T) {
	s := &State{}
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20, RoomBelow: 280, RoomAbove: 280}
	short := []float32{200, 40, 60, 80}
	// Landing, the fourth goes under the shortest.
	s.slotted(z, foundOf("q", "a", "b", "c", "d"), []float32{200, 200, 200, 80}, nil)
	measured, _ := s.slotted(z, foundOf("q", "a", "b", "c", "d"), short, nil)
	if measured[3].X != measured[1].X || measured[3].Y < 0 {
		t.Fatalf("Measuring itself as it lands, the field should choose from what it measures: %+v.", measured)
	}
	s.slotted(z, foundOf("q", "a", "b", "c", "d"), short, nil)
	grown, _ := s.slotted(z, foundOf("q", "a", "b", "c", "d"), []float32{200, 290, 60, 80}, nil)
	if grown[3].X != measured[3].X || grown[3].Y != 20+290+10 {
		t.Fatalf("Grown once it stands, a block should push what is beyond it in its column, not move it: %+v.", grown[3])
	}
	// A block come and gone again, the field stands as it did.
	s.slotted(z, foundOf("q", "n", "a", "b", "c", "d"), []float32{40, 200, 40, 60, 80}, nil)
	s.slotted(z, foundOf("q", "n", "a", "b", "c", "d"), []float32{40, 200, 40, 60, 80}, nil)
	back, _ := s.slotted(z, foundOf("q", "a", "b", "c", "d"), short, nil)
	if !slices.Equal(back, measured) {
		t.Fatalf("A block come and gone should leave the field as it stood: %+v, not %+v.", back, measured)
	}
	anew, _ := s.slotted(z, foundOf("r", "d", "c", "a"), []float32{80, 60, 200}, nil)
	if anew[0].X != 0-50 || anew[0].Y != 20 {
		t.Fatalf("A new query should choose anew, what ranks first in the middle: %+v.", anew)
	}
}

// Short blocks under the pill take the next block under them, where it
// fits, rather than the field standing one alone over the pill.
func TestTheFieldFillsTheRoomUnderThePillFirst(t *testing.T) {
	state, library := libraryState()
	library.desk = state
	library.groupTree = map[string][]GroupItem{"": {
		{ID: "1", Name: "Inbox", Count: 1, Inbox: true}, {ID: "2", Name: "Novel", Count: 1},
		{ID: "3", Name: "Notes"}, {ID: "4", Name: "Poems"}, {ID: "6", Name: "Plays"},
	}}
	state.ShowField()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	cols := map[float32]bool{}
	for _, fr := range state.field.rest {
		cols[fr.Rect.X] = true
		if fr.Rect.Y < 0 {
			t.Fatalf("Short blocks should all stand under the pill while they fit: %v.", state.field.rest)
		}
	}
	if len(state.field.rest) != 5 || len(cols) >= 5 {
		t.Fatalf("Five short blocks should stack under the pill: %v.", state.field.rest)
	}
}
