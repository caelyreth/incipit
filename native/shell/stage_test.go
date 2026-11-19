package shell

import (
	"slices"
	"testing"
)

func TestBlocksTakeTheirSlotsOutwardFromThePill(t *testing.T) {
	want5 := []stageSlot{{2, true}, {3, true}, {1, true}, {4, true}, {0, true}, {2, false}, {3, false}, {1, false}, {4, false}, {0, false}}
	if got := stageSlots(5); !slices.Equal(got, want5) {
		t.Fatalf("Five columns should take the row under the pill from the middle outward, then the row over: %v", got)
	}
	want4 := []stageSlot{{2, true}, {1, true}, {3, true}, {0, true}, {2, false}, {1, false}, {3, false}, {0, false}}
	if got := stageSlots(4); !slices.Equal(got, want4) {
		t.Fatalf("Four columns should start at the two middle ones: %v", got)
	}
	// A row's stageSlots taken as many as make the middle even stand even
	// about the pill.
	for _, cols := range []int{1, 2, 3, 4, 5} {
		z := stageHorizon{Cols: cols, Col: 100, Gap: 10}
		for n := 2 - cols%2; n <= cols; n += 2 {
			lo, hi := float32(1e9), float32(-1e9)
			rects, _ := z.Place(make([]float32, n), nil)
			for _, r := range rects {
				lo, hi = min(lo, r.X), max(hi, r.X+r.W)
			}
			if lo != -hi {
				t.Fatalf("%d of %d columns' stageSlots should stand even across: %v to %v", n, cols, lo, hi)
			}
		}
	}
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20}
	rects, _ := z.Place([]float32{50, 60, 70, 80}, nil)
	if rects[0].Y != 20 || rects[1].Y != 20 || rects[2].Y != 20 || rects[3].Y != -100 {
		t.Fatalf("The first three should stand side by side under the pill, the fourth over it: %+v", rects)
	}
	// A block growing moves none but what stands beyond it in its half.
	grown, _ := z.Place([]float32{500, 60, 70, 80}, nil)
	for i := 1; i < 4; i++ {
		if grown[i] != rects[i] {
			t.Fatalf("Block %d moved as the first grew: %+v to %+v", i, rects[i], grown[i])
		}
	}
}

// A list is read: rows of the columns from its top, left to right and
// top down, each as tall as its tallest, a divider across the columns
// alone in its row.
func TestAListFillsItsRowsInReadingOrder(t *testing.T) {
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20}
	rects := z.Rows([]float32{30, 50, 80, 50, 50}, []bool{true, false, false, false, false})
	if rects[0].W != 320 || rects[1].Y != rects[0].Y+30+5 || rects[2].X != rects[1].X+110 {
		t.Fatalf("The divider should stand across, its sheets under it left to right: %+v.", rects)
	}
	if rects[0].Y != 0 || rects[4].Y != rects[1].Y+80+10 || rects[4].X != rects[1].X {
		t.Fatalf("The list should start at its top, the next row under the tallest: %+v.", rects)
	}
}

// Each band stands as a ring past the last: a band starts at the next row
// of slots, past a ring's room across the half that a nearer band holds,
// its name there where named.
func TestBandsStandAsRings(t *testing.T) {
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20, Ring: 30}
	// Two in the nearest band, under the pill; the next starts over it,
	// then comes back under, past the ring.
	rects, marks := z.Place([]float32{50, 80, 50, 50, 50, 50, 50}, []int{0, 0, 1, 1, 1, 1, 1})
	if rects[2].Y >= 0 || rects[2].Y != -70 {
		t.Fatalf("The next band should start over the pill: %+v.", rects[2])
	}
	if rects[5].Y != 20+80+10+30 || rects[6].Y != rects[5].Y {
		t.Fatalf("Coming under, the band should stand past the ring, levelled: %+v.", rects)
	}
	if len(marks) != 0 {
		t.Fatalf("Bands not named have no names: %+v.", marks)
	}
	z.Named = true
	rects, marks = z.Place([]float32{50, 50}, []int{0, 1})
	if len(marks) != 2 || marks[0].Rect.Y != 20 || rects[0].Y != 50 || marks[1].Rect.Y != -50 || rects[1].Y != -100 {
		t.Fatalf("Each band's first row should have its name's room: %+v %+v.", rects, marks)
	}
}

// The screen's order reads the rows over the horizon from the top, then
// those under it, each left to right, however the slots were filled.
func TestTheScreenOrderReadsAsTheFieldStands(t *testing.T) {
	z := stageHorizon{Cols: 4, Col: 100, Gap: 10, Above: 20, Below: 20}
	heights := []float32{50, 50, 50, 50, 50, 50, 50, 50, 50}
	rects, _ := z.Place(heights, nil)
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
