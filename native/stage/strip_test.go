package stage

import "testing"

func TestAWindowShowsOneSheetWholeAndMoreByHalvesOrThirds(t *testing.T) {
	for _, tc := range []struct {
		count       int
		inner, page float32
		want        int
	}{
		{0, 1200, 600, 1},
		{1, 2400, 600, 1},
		{2, 2400, 600, 2},
		{3, 1200, 600, 2},
		{3, 2400, 600, 3},
		{5, 1900, 600, 3},
	} {
		if got := PerWindow(tc.count, tc.inner, tc.page, 1); got != tc.want {
			t.Errorf("%d sheets in %v with %v pages: %d to a window, want %d", tc.count, tc.inner, tc.page, got, tc.want)
		}
	}
}

func TestColumnsShareTheRoomLessTheGaps(t *testing.T) {
	if w := ColumnWidth(1001, 2, 1); w != 500 {
		t.Fatalf("Two columns of 1001 with a gap of 1 should be 500 wide, not %v.", w)
	}
	if w := ColumnWidth(800, 1, 1); w != 800 {
		t.Fatalf("One column should take the room whole, not %v.", w)
	}
}

func TestTheStripRestsAtTheNearestColumnWithinReach(t *testing.T) {
	spans := []Span{{0, 500}, {501, 500}, {1002, 300}}
	maxX := float32(1302 - 1000)
	if rest, ok := NearRest(spans, maxX, 280, 72); !ok || rest != maxX {
		t.Fatalf("Near the strip's end, it should rest there, at %v, not %v (%v).", maxX, rest, ok)
	}
	if _, ok := NearRest(spans, maxX, 150, 72); ok {
		t.Fatal("Nowhere near a rest, the strip should stay.")
	}
	if at := At(spans, 600); at != 1 {
		t.Fatalf("600 along stands in the second column, not %d.", at)
	}
}

// A horizon holds an odd count of columns, one under the pill.
func TestTheHorizonHoldsAnOddCountOfColumns(t *testing.T) {
	for _, w := range []float32{200, 600, 900, 1200, 1500} {
		if cols, col := Columns(w, 260, 340, 14); cols%2 == 0 || col > 340 {
			t.Fatalf("%v wide should hold an odd count of columns at most 340 wide: %d of %v.", w, cols, col)
		}
	}
}
