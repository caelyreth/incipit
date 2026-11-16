package stage

import "testing"

func TestBlocksTakeTheirSlotsOutwardFromThePill(t *testing.T) {
	if got := order(5); got[0] != 2 || got[1] != 3 || got[2] != 1 || got[3] != 4 || got[4] != 0 {
		t.Fatalf("Five columns should go outward from the middle: %v", got)
	}
	if got := order(4); got[0] != 2 || got[1] != 1 || len(got) != 4 {
		t.Fatalf("Four columns should start at the two middle ones: %v", got)
	}
	z := Horizon{Cols: 3, Col: 100, Gap: 10, Band: 40}
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
