package stage

import "testing"

func TestBlocksTakeTheirSlotsOutwardFromThePill(t *testing.T) {
	if got := slots(5); len(got) != 10 || got[0] != (slot{2, true}) || got[1] != (slot{2, false}) ||
		got[2] != (slot{3, true}) || got[3] != (slot{1, false}) || got[6] != (slot{4, true}) || got[7] != (slot{0, false}) {
		t.Fatalf("Five columns should go outward from the middle one, crosswise: %v", got)
	}
	if got := slots(4); len(got) != 8 || got[0] != (slot{2, true}) || got[1] != (slot{1, false}) ||
		got[2] != (slot{1, true}) || got[3] != (slot{2, false}) {
		t.Fatalf("Four columns should start at the two middle ones, crosswise: %v", got)
	}
	// Any even count of a ring's slots taken stands even about the pill.
	for _, cols := range []int{2, 3, 4, 5} {
		z := Horizon{Cols: cols, Col: 100, Gap: 10}
		for n := 2; n <= 2*cols; n += 2 {
			lo, hi := float32(1e9), float32(-1e9)
			for _, r := range z.Place(make([]float32, n)) {
				lo, hi = min(lo, r.X), max(hi, r.X+r.W)
			}
			if lo != -hi {
				t.Fatalf("%d of %d columns' slots should stand even across: %v to %v", n, cols, lo, hi)
			}
		}
	}
	z := Horizon{Cols: 3, Col: 100, Gap: 10, Above: 20, Below: 20}
	rects := z.Place([]float32{50, 60, 70, 80})
	if rects[0].Y != 20 || rects[1].Y != -80 || rects[0].X != rects[1].X {
		t.Fatalf("The first two should stand below and above the middle: %+v", rects)
	}
	// A block growing moves none but what stands beyond it in its half.
	grown := z.Place([]float32{500, 60, 70, 80})
	for i := 1; i < 4; i++ {
		if grown[i] != rects[i] {
			t.Fatalf("Block %d moved as the first grew: %+v to %+v", i, rects[i], grown[i])
		}
	}
}
