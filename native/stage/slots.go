package stage

// The field stands on either side of its horizon, the band across its
// middle that holds the pill and that nothing is drawn in. What it shows
// goes in slots numbered outward from the pill, in rings of columns, each
// ring's slots alternately under and over it (slots). Block n takes slot n; past the last, blocks
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

// slot is a place on one side of the horizon in a column: under it, or
// over it.
type slot struct {
	col   int
	below bool
}

// slots are the horizon's slots outward from the pill, in rings: the
// middle column under and then over it, where the count is odd; then each
// two columns either side of what is taken, outward, crosswise: the right
// under, the left over, the left under, the right over. So the slots taken
// stand even about the pill, across and up and down, whenever an even
// count of a ring's are.
func slots(cols int) []slot {
	out := make([]slot, 0, 2*cols)
	right, left := cols/2, cols/2-1
	if cols%2 == 1 {
		out = append(out, slot{right, true}, slot{right, false})
		right++
	}
	for ; left >= 0; right, left = right+1, left-1 {
		out = append(out, slot{right, true}, slot{left, false}, slot{left, true}, slot{right, false})
	}
	return out
}

// Place stands blocks of heights in their slots, the horizon's middle at
// the origin: those below hang from the band, those above stand on it.
// Every block is one column wide.
func (z Horizon) Place(heights []float32) []Rect {
	order := slots(z.Cols)
	left := -(float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap) / 2
	below := make([]float32, z.Cols)
	above := make([]float32, z.Cols)
	for i := range below {
		below[i], above[i] = z.Below, -z.Above
	}
	out := make([]Rect, len(heights))
	for i, h := range heights {
		s := order[i%len(order)]
		c := s.col
		x := left + float32(c)*(z.Col+z.Gap)
		if s.below {
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

// Extent is how far across what rects hold reaches, from the horizon's
// middle: left of it, lo below 0, and right, hi; none for none.
func Extent(rects []Rect) (lo, hi float32) {
	if len(rects) == 0 {
		return 0, 0
	}
	lo, hi = rects[0].X, rects[0].X+rects[0].W
	for _, r := range rects[1:] {
		lo, hi = min(lo, r.X), max(hi, r.X+r.W)
	}
	return lo, hi
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
