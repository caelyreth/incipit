package stage

import (
	"slices"
	"testing"
)

func TestBlocksTakeTheirSlotsOutwardFromThePill(t *testing.T) {
	want5 := []slot{{2, true}, {3, true}, {1, true}, {4, true}, {0, true}, {2, false}, {3, false}, {1, false}, {4, false}, {0, false}}
	if got := slots(5); !slices.Equal(got, want5) {
		t.Fatalf("Five columns should take the row under the pill from the middle outward, then the row over: %v", got)
	}
	want4 := []slot{{2, true}, {1, true}, {3, true}, {0, true}, {2, false}, {1, false}, {3, false}, {0, false}}
	if got := slots(4); !slices.Equal(got, want4) {
		t.Fatalf("Four columns should start at the two middle ones: %v", got)
	}
	// A row's slots taken as many as make the middle even stand even
	// about the pill.
	for _, cols := range []int{1, 2, 3, 4, 5} {
		z := Horizon{Cols: cols, Col: 100, Gap: 10}
		for n := 2 - cols%2; n <= cols; n += 2 {
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
