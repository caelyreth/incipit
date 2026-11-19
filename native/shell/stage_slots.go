package shell

import (
	"cmp"
	"slices"
)

// The field stands on either side of its horizon, the band across its
// middle that holds the pill and that nothing is drawn in. What it shows
// goes in stageSlots numbered outward from the pill, a row at a time: across
// the row under it, from the middle outward, then across the row over it
// (stageSlots), so that what little the field holds stands side by side, as
// wide as the window lets it, before it takes the other side. Block n
// takes stageSlot n; past the last, blocks stack outward in the same order, a
// row under and then a row over. A block's stageSlot so depends only on its
// rank and how many columns the window holds, never on any block's size:
// one that grows moves only those stacked beyond it in its own half of its
// column, and only outward.

// stageHorizon is the field's columns, the band they stand on either side of,
// and the room between what they hold. The band reaches Above over the
// horizon's middle and Below under it: the pill, and the ribbons on either
// side of it.
type stageHorizon struct {
	Cols         int
	Col, Gap     float32
	Above, Below float32
}

// stageColumns fits a horizon's columns into width w: as many as hold at least
// least wide, each no wider than most.
func stageColumns(w, least, most, gap float32) (cols int, col float32) {
	cols = max(1, int((w+gap)/(least+gap)))
	col = min(most, (w-float32(cols-1)*gap)/float32(cols))
	return cols, max(col, 1)
}

// stageSlot is a place on one side of the horizon in a column: under it, or
// over it.
type stageSlot struct {
	col   int
	below bool
}

// stageSlots are the horizon's stageSlots outward from the pill: the row under it,
// then the row over it, each from the middle column, or the right of the
// middle two, outward, right and left in turn. So a row's stageSlots taken
// stand even about the pill whenever as many of them are taken as make
// the middle even: an odd count where the columns are odd, an even where
// they are even.
func stageSlots(cols int) []stageSlot {
	var across []int
	right, left := cols/2, cols/2-1
	if cols%2 == 1 {
		across = append(across, right)
		right++
	}
	for ; left >= 0; right, left = right+1, left-1 {
		across = append(across, right, left)
	}
	out := make([]stageSlot, 0, 2*cols)
	for _, below := range []bool{true, false} {
		for _, col := range across {
			out = append(out, stageSlot{col, below})
		}
	}
	return out
}

// Place stands blocks of heights in their stageSlots, the horizon's middle at
// the origin: those below hang from the band, those above stand on it.
// Every block is one column wide.
func (z stageHorizon) Place(heights []float32) []stageRect {
	order := stageSlots(z.Cols)
	left := -(float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap) / 2
	below := make([]float32, z.Cols)
	above := make([]float32, z.Cols)
	for i := range below {
		below[i], above[i] = z.Below, -z.Above
	}
	out := make([]stageRect, len(heights))
	for i, h := range heights {
		s := order[i%len(order)]
		c := s.col
		x := left + float32(c)*(z.Col+z.Gap)
		if s.below {
			out[i] = stageRect{X: x, Y: below[c], W: z.Col, H: h}
			below[c] += h + z.Gap
		} else {
			above[c] -= h
			out[i] = stageRect{X: x, Y: above[c], W: z.Col, H: h}
			above[c] -= z.Gap
		}
	}
	return out
}

// stageScreenOrder is the order rects are read in as they stand on the
// screen, as indexes into rects: the rows over the horizon from the
// furthest to the one on it, then those under it from the one on it
// outward, each left to right. A row is how far a block is stacked out
// from the horizon in its column, not where it happens to stand, so
// blocks of uneven heights read across as their slots do.
func stageScreenOrder(rects []stageRect) []int {
	level := make([]int, len(rects))
	for i, r := range rects {
		for j, o := range rects {
			// Nearer the horizon on the same side of it, in the same column.
			if j != i && o.X == r.X && (o.Y < 0) == (r.Y < 0) && (r.Y < 0 && o.Y > r.Y || r.Y >= 0 && o.Y < r.Y) {
				level[i]++
			}
		}
	}
	out := make([]int, len(rects))
	for i := range out {
		out[i] = i
	}
	slices.SortStableFunc(out, func(a, b int) int {
		ra, rb := rects[a], rects[b]
		aboveA, aboveB := ra.Y < 0, rb.Y < 0
		switch {
		case aboveA != aboveB:
			return map[bool]int{true: -1, false: 1}[aboveA]
		case level[a] != level[b] && aboveA:
			return level[b] - level[a]
		case level[a] != level[b]:
			return level[a] - level[b]
		}
		return cmp.Compare(ra.X, rb.X)
	})
	return out
}

// stageExtent is how far across what rects hold reaches, from the horizon's
// middle: left of it, lo below 0, and right, hi; none for none.
func stageExtent(rects []stageRect) (lo, hi float32) {
	if len(rects) == 0 {
		return 0, 0
	}
	lo, hi = rects[0].X, rects[0].X+rects[0].W
	for _, r := range rects[1:] {
		lo, hi = min(lo, r.X), max(hi, r.X+r.W)
	}
	return lo, hi
}

// stageReach is how far each half of the field holds, past the band: below,
// how far down its lowest block ends, and above how far up its highest
// starts, from the horizon's middle.
func stageReach(rects []stageRect) (below, above float32) {
	for _, r := range rects {
		if r.Y >= 0 {
			below = max(below, r.Y+r.H)
		} else {
			above = max(above, -r.Y)
		}
	}
	return below, above
}

// Rows stands blocks of heights as a list reads (field-and-desk.md
// §4.7.6): in rows of the columns under the band, left to right and top
// down, each row as tall as its tallest, a block starting a new row where
// newRow says. The rows stand together across the columns, a short one
// from the left.
func (z stageHorizon) Rows(heights []float32, newRow []bool) []stageRect {
	left := -(float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap) / 2
	out := make([]stageRect, len(heights))
	y, rowH, col := z.Below, float32(0), 0
	for i, h := range heights {
		if i > 0 && (col == z.Cols || newRow[i]) {
			y, rowH, col = y+rowH+z.Gap, 0, 0
		}
		out[i] = stageRect{X: left + float32(col)*(z.Col+z.Gap), Y: y, W: z.Col, H: h}
		rowH = max(rowH, h)
		col++
	}
	return out
}
