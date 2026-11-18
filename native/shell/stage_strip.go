// stage is the shell's layout system: where things stand worked
// out as plain data, apart from drawing them (references/field-and-desk.md).
// It holds no UI and no state; the shell gives it what there is and draws
// what it works out.
//
// The strip is the desk's arrangement: its columns side by side in one
// row that scrolls sideways, as many to a window as the window holds for
// how many there are.
package shell

import "math"

// stageSpan is a column's place along a strip, from the strip's start.
type stageSpan struct{ X, W float32 }

// End is where the span ends along the strip.
func (s stageSpan) End() float32 { return s.X + s.W }

// stagePerWindow is how many sheets' columns a window shows of count, its room
// inner wide: one alone fills it, and more go two to a window, or three
// where a third still holds a page page wide, gap between each.
func stagePerWindow(count int, inner, page, gap float32) int {
	switch {
	case count <= 1:
		return 1
	case count >= 3 && (inner-2*gap)/3 >= page:
		return 3
	default:
		return 2
	}
}

// stageColumnWidth is a column's width where n share room inner wide, gap
// between each; n need not be whole, as it is on its way from one count to
// another.
func stageColumnWidth(inner, n, gap float32) float32 {
	return max(0, (inner-(n-1)*gap)/max(n, 1))
}

// stageRests are where a strip that goes as far as maxX rests at each of its
// columns, the column's start at the window's.
func stageRests(spans []stageSpan, maxX float32) []float32 {
	out := make([]float32, 0, len(spans))
	for _, sp := range spans {
		out = append(out, min(sp.X, maxX))
	}
	return out
}

// stageNearRest is the rest nearest x, the strip's own ends among them, if it is
// within reach.
func stageNearRest(spans []stageSpan, maxX, x, reach float32) (float32, bool) {
	best, found := float32(0), false
	for _, at := range append(stageRests(spans, maxX), 0, maxX) {
		if d := math.Abs(float64(at - x)); d <= float64(reach) && (!found || d < math.Abs(float64(best-x))) {
			best, found = at, true
		}
	}
	return best, found
}

// stageAt is the column whose span holds x, the last that starts at or before
// it, or the first.
func stageAt(spans []stageSpan, x float32) int {
	for i := len(spans) - 1; i > 0; i-- {
		if x >= spans[i].X {
			return i
		}
	}
	return 0
}
