package shell

import (
	"slices"
	"testing"
)

func TestBlocksTakeTheirSlotsOutwardFromThePill(t *testing.T) {
	want5 := []stageSlot{{2, true}, {3, true}, {1, true}, {4, true}, {0, true}, {2, false}, {3, false}, {1, false}, {4, false}, {0, false}}
	if got := stageSlots(5, false); !slices.Equal(got, want5) {
		t.Fatalf("Five columns should take the row under the pill from the middle outward, then the row over: %v", got)
	}
	want4 := []stageSlot{{2, true}, {1, true}, {3, true}, {0, true}, {2, false}, {1, false}, {3, false}, {0, false}}
	if got := stageSlots(4, false); !slices.Equal(got, want4) {
		t.Fatalf("Four columns should start at the two middle ones: %v", got)
	}
	// A row's stageSlots taken as many as make the middle even stand even
	// about the pill.
	for _, cols := range []int{1, 2, 3, 4, 5} {
		z := stageHorizon{Cols: cols, Col: 100, Gap: 10}
		for n := 2 - cols%2; n <= cols; n += 2 {
			lo, hi := float32(1e9), float32(-1e9)
			for _, r := range z.Place(make([]float32, n)) {
				lo, hi = min(lo, r.X), max(hi, r.X+r.W)
			}
			if lo != -hi {
				t.Fatalf("%d of %d columns' stageSlots should stand even across: %v to %v", n, cols, lo, hi)
			}
		}
	}
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20}
	rects := z.Place([]float32{50, 60, 70, 80})
	if rects[0].Y != 20 || rects[1].Y != 20 || rects[2].Y != 20 || rects[3].Y != -100 {
		t.Fatalf("The first three should stand side by side under the pill, the fourth over it: %+v", rects)
	}
	// A block growing moves none but what stands beyond it in its half.
	grown := z.Place([]float32{500, 60, 70, 80})
	for i := 1; i < 4; i++ {
		if grown[i] != rects[i] {
			t.Fatalf("Block %d moved as the first grew: %+v to %+v", i, rects[i], grown[i])
		}
	}
}

// A sorted field is read: each row from the left, the row under the pill
// first, then the row over it.
func TestASortedFieldFillsItsRowsInReadingOrder(t *testing.T) {
	want := []stageSlot{{0, true}, {1, true}, {2, true}, {0, false}, {1, false}, {2, false}}
	if got := stageSlots(3, true); !slices.Equal(got, want) {
		t.Fatalf("Three columns read should go left to right under the pill, then over it: %v", got)
	}
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Reading: true}
	rects := z.Place([]float32{50, 50, 50, 50})
	for i := 1; i < 3; i++ {
		if rects[i].X <= rects[i-1].X {
			t.Fatalf("The row under the pill should read left to right: %v.", rects)
		}
	}
	if rects[3].Y >= 0 || rects[3].X != rects[0].X {
		t.Fatalf("The fourth should begin the row over the pill at the left: %v.", rects[3])
	}
}

// The screen's order reads the rows over the horizon from the top, then
// those under it, each left to right, however the slots were filled.
func TestTheScreenOrderReadsAsTheFieldStands(t *testing.T) {
	z := stageHorizon{Cols: 4, Col: 100, Gap: 10, Above: 20, Below: 20}
	heights := []float32{50, 50, 50, 50, 50, 50, 50, 50, 50}
	rects := z.Place(heights)
	var got []int
	for _, i := range stageScreenOrder(rects) {
		got = append(got, i)
	}
	// Ranks under the pill, middle out: 2 1 3 0 across as 3 1 0 2; over
	// it the same as 7 5 4 6; the ninth stacks under rank 0's column.
	want := []int{7, 5, 4, 6, 3, 1, 0, 2, 8}
	if !slices.Equal(got, want) {
		t.Fatalf("The field should be read across as it stands: %v, not %v.", want, got)
	}
}
