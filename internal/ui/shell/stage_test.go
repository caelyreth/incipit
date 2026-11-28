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
			rects, _, _ := z.Place(make([]float32, n), nil, nil)
			for _, r := range rects {
				lo, hi = min(lo, r.X), max(hi, r.X+r.W)
			}
			if lo != -hi {
				t.Fatalf("%d of %d columns' stageSlots should stand even across: %v to %v", n, cols, lo, hi)
			}
		}
	}
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20, RoomAbove: 100}
	rects, _, slots := z.Place([]float32{50, 60, 70, 80, 80, 80}, nil, nil)
	if rects[0].Y != 20 || rects[1].Y != 20 || rects[2].Y != 20 || rects[3].Y != -100 || rects[5].Y != -100 {
		t.Fatalf("With no room under the pill, the first three should stand side by side under it, the next three over it: %+v", rects)
	}
	// Fewer than a row left, none starts a row over the pill alone.
	lone, _, _ := z.Place([]float32{50, 60, 70, 80}, nil, nil)
	if lone[3].Y < 0 || lone[3].X != rects[0].X {
		t.Fatalf("One left should go on under the pill, not alone over it: %+v.", lone[3])
	}
	// Past the room over the pill too, the field goes on under it.
	more, _, _ := z.Place([]float32{50, 60, 70, 80, 80, 80, 150}, nil, nil)
	if more[6].Y < 0 || more[6].X != rects[0].X {
		t.Fatalf("What fits nowhere should go on under the pill, nearest it: %+v.", more[6])
	}
	// A block growing, its columns held, moves none but what stands
	// beyond it in its half.
	grown, _, _ := z.Place([]float32{500, 60, 70, 80, 80, 80}, nil, slots)
	for i := 1; i < 6; i++ {
		if grown[i] != rects[i] {
			t.Fatalf("Block %d moved as the first grew: %+v to %+v", i, rects[i], grown[i])
		}
	}
}

// Past the first row, a block goes under the pill while it fits there,
// in the column it stands nearest the pill in, then over it while it
// fits, then wherever it stands nearest.
func TestBlocksFillTheRoomUnderThePillFirst(t *testing.T) {
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20, RoomBelow: 280, RoomAbove: 280}
	rects, _, _ := z.Place([]float32{200, 40, 60, 80, 50, 200, 100, 200}, nil, nil)
	// Across: 200 in the middle, 40 right, 60 left.
	if rects[3].X != rects[1].X || rects[3].Y != 20+40+10 {
		t.Fatalf("The fourth should stand under the shortest, not over the pill: %+v.", rects[3])
	}
	if rects[4].X != rects[2].X || rects[4].Y != 20+60+10 {
		t.Fatalf("The fifth should stand under the next shortest: %+v.", rects[4])
	}
	if rects[5].Y >= 0 {
		t.Fatalf("What fits nowhere under the pill should stand over it: %+v.", rects[5])
	}
	if rects[6].Y != 20+60+10+50+10 || rects[6].X != rects[2].X {
		t.Fatalf("What fits under the pill still goes there: %+v.", rects[6])
	}
	if rects[7].Y >= 0 || rects[7].X == rects[5].X {
		t.Fatalf("Over the pill, the next should take the column nearest it: %+v after %+v.", rects[7], rects[5])
	}
	if nowhere, _, _ := z.Place([]float32{200, 40, 60, 80, 50, 200, 100, 200, 290}, nil, nil); nowhere[8].Y < 0 {
		t.Fatalf("What fits nowhere should go on under the pill, not over it: %+v.", nowhere[8])
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

// Each band stands as a ring past the last: a band coming into a half a
// nearer band holds starts past it, the half levelled across and a
// ring's room left, its name there where named, in each half it takes.
func TestBandsStandAsRings(t *testing.T) {
	z := stageHorizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20, Ring: 30, RoomBelow: 100, RoomAbove: 200}
	// Two in the nearest band, under the pill; with no room under it, the
	// next starts over it.
	rects, marks, _ := z.Place([]float32{50, 80, 50, 50, 50}, []int{0, 0, 1, 1, 1}, nil)
	if rects[2].Y != -70 {
		t.Fatalf("The next band should start over the pill: %+v.", rects[2])
	}
	if len(marks) != 0 {
		t.Fatalf("Bands not named have no names: %+v.", marks)
	}
	// With room under the pill, it comes under, past the ring, levelled.
	z.RoomBelow = 380
	rects, _, _ = z.Place([]float32{50, 80, 50, 50}, []int{0, 0, 1, 1}, nil)
	if rects[2].Y != 20+80+10+30 || rects[3].Y != rects[2].Y {
		t.Fatalf("Coming under, the band should stand past the ring, levelled: %+v.", rects)
	}
	z.RoomBelow, z.Named, z.Cols = 100, true, 1
	rects, marks, _ = z.Place([]float32{50, 50}, []int{0, 1}, nil)
	if len(marks) != 2 || marks[0].Rect.Y != 20 || rects[0].Y != 50 || marks[1].Rect.Y != -50 || rects[1].Y != -100 {
		t.Fatalf("Each band's first row should have its name's room: %+v %+v.", rects, marks)
	}
	// A band taking both halves is named in each.
	_, marks, _ = z.Place([]float32{50, 50, 50, 60}, []int{0, 1, 1, 1}, nil)
	if len(marks) != 3 || marks[1].Band != 1 || marks[2].Band != 1 || marks[1].Below == marks[2].Below {
		t.Fatalf("The second band should be named over the pill and under it: %+v.", marks)
	}
}

// The screen's order reads the rows over the horizon from the top, then
// those under it, each left to right, however the slots were filled.
func TestTheScreenOrderReadsAsTheFieldStands(t *testing.T) {
	z := stageHorizon{Cols: 4, Col: 100, Gap: 10, Above: 20, Below: 20, RoomAbove: 60}
	heights := []float32{50, 50, 50, 50, 50, 50, 50, 50, 50}
	rects, _, _ := z.Place(heights, nil, nil)
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
