package stage

// The field stands on either side of its horizon, the band across its
// middle that holds the pill and that nothing is drawn in. What it shows
// goes in slots numbered outward from the pill: the middle column below
// the horizon, then above it, then the columns beside it, below and then
// above, and so on outward. Block n takes slot n; past the last, blocks
// stack outward in the same order. A block's slot so depends only on its
// rank and how many columns the window holds, never on any block's size:
// one that grows moves only those stacked beyond it in its own half of its
// column, and only outward.

// Horizon is the field's columns, the band they stand on either side of,
// and the room between what they hold. The band reaches Above over the
// horizon's middle and Below under it: the pill, and the ribbons on either
// side of it.
type Horizon struct {
	Cols         int
	Col, Gap     float32
	Above, Below float32
}

// Columns fits a horizon's columns into width w: as many as hold at least
// least wide, each no wider than most.
func Columns(w, least, most, gap float32) (cols int, col float32) {
	cols = max(1, int((w+gap)/(least+gap)))
	col = min(most, (w-float32(cols-1)*gap)/float32(cols))
	return cols, max(col, 1)
}

// order is the columns outward from the middle, right before left: for an
// even count, the right of the two middle ones first.
func order(cols int) []int {
	out := make([]int, 0, cols)
	right, left := cols/2, cols/2-1
	if cols%2 == 1 {
		out = append(out, right)
		right++
	}
	for len(out) < cols {
		if right < cols {
			out = append(out, right)
			right++
		}
		if left >= 0 && len(out) < cols {
			out = append(out, left)
			left--
		}
	}
	return out
}

// Place stands blocks of heights in their slots, the horizon's middle at
// the origin: those below hang from the band, those above stand on it.
// Every block is one column wide.
func (z Horizon) Place(heights []float32) []Rect {
	cols := order(z.Cols)
	left := -(float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap) / 2
	below := make([]float32, z.Cols)
	above := make([]float32, z.Cols)
	for i := range below {
		below[i], above[i] = z.Below, -z.Above
	}
	out := make([]Rect, len(heights))
	slots := 2 * z.Cols
	for i, h := range heights {
		s := i % slots
		c := cols[s/2]
		x := left + float32(c)*(z.Col+z.Gap)
		if s%2 == 0 {
			out[i] = Rect{X: x, Y: below[c], W: z.Col, H: h}
			below[c] += h + z.Gap
		} else {
			above[c] -= h
			out[i] = Rect{X: x, Y: above[c], W: z.Col, H: h}
			above[c] -= z.Gap
		}
	}
	return out
}

// Reach is how far each half of the field holds, past the band: below,
// how far down its lowest block ends, and above how far up its highest
// starts, from the horizon's middle.
func Reach(rects []Rect) (below, above float32) {
	for _, r := range rects {
		if r.Y >= 0 {
			below = max(below, r.Y+r.H)
		} else {
			above = max(above, -r.Y)
		}
	}
	return below, above
}
