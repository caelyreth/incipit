package shell

import (
	"cmp"
	"slices"
)

// The field stands on either side of its horizon, the band across its
// middle that holds the pill and that nothing is drawn in. A block takes
// a column on one side of it (a stageSlot), chosen in rank order as the
// query lands: the first row under the pill goes across from the middle
// outward (stageSlots), so that what little the field holds stands side
// by side, as wide as the window lets it; after that each block goes
// under the pill, in the column it stands nearest the pill in, while it
// fits in the window under it (RoomBelow), so a short block's column takes
// the next rather than the far side; then over the pill while it fits
// there (RoomAbove); then under it again, past what the window holds, so
// nothing stands over the pill but what the window holds there. The columns
// chosen are held while the query stands (field_place.go): a block that
// grows moves only those stacked beyond it in its own half of its column,
// and only outward.
//
// Bands (bands.go) stand as rings: a band entering a half that a nearer
// band holds starts past it, the half levelled across the columns and a
// ring's room left, so the band reads as one step further out. Where the
// bands are named, the ring's room holds the name, in each half the band
// takes.

// stageHorizon is the field's columns, the band they stand on either side of,
// and the room between what they hold. The band reaches Above over the
// horizon's middle and Below under it: the pill, and the ribbons on either
// side of it.
type stageHorizon struct {
	Cols         int
	Col, Gap     float32
	Above, Below float32
	// Ring is the room a band starts past, besides the gap; Named is
	// whether its names stand there, each band's first row in each half
	// then having one, the nearest's too.
	Ring  float32
	Named bool
	// RoomBelow and RoomAbove are how far blocks may reach past the band
	// in the window, under it and over it, the pill at rest: what a block
	// is put where it fits in, whatever ribbons stand about the pill as it
	// is, so the pill opening or closing chooses nothing anew; 0 fits
	// nothing.
	RoomBelow, RoomAbove float32
}

// stageMark is where a band's name stands: the ring's room it starts past,
// across the columns, in a half it takes, under the pill or over it.
type stageMark struct {
	Band  int
	Below bool
	Rect  stageRect
}

// stageColumns fits a horizon's columns into width w: as many as hold at least
// least wide, each no wider than most.
func stageColumns(w, least, most, gap float32) (cols int, col float32) {
	cols = max(1, int((w+gap)/(least+gap)))
	col = min(most, (w-float32(cols-1)*gap)/float32(cols))
	return cols, max(col, 1)
}

// stageSlot is a place on one side of the horizon in a column: under it, or
// over it; col below 0 for none yet.
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

// unslotted is a block not yet given its slot.
var unslotted = stageSlot{col: -1}

// Place stands blocks of heights, the horizon's middle at the origin:
// those below hang from the band, those above stand on it, each band of
// bands, nil for none, a ring further out than the last. Each block takes
// the slot kept gives it, kept nil or its slot unslotted for one chosen
// here; slots are those it took. Every block is one column wide.
func (z stageHorizon) Place(heights []float32, bands []int, kept []stageSlot) (rects []stageRect, marks []stageMark, slots []stageSlot) {
	// across is the columns from the middle outward.
	var across []int
	for _, s := range stageSlots(z.Cols)[:z.Cols] {
		across = append(across, s.col)
	}
	width := float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap
	left := -width / 2
	// fill is how far each column reaches from the middle in each half,
	// past the band.
	fill := map[bool][]float32{true: make([]float32, z.Cols), false: make([]float32, z.Cols)}
	for i := range z.Cols {
		fill[true][i], fill[false][i] = z.Below, z.Above
	}
	// room is how far each half reaches from the middle, past the band as
	// it stands now.
	room := map[bool]float32{true: z.Below + z.RoomBelow, false: z.Above + z.RoomAbove}
	// The band last stood in each half, under the horizon and over it;
	// -1 for none yet.
	held := map[bool]int{true: -1, false: -1}
	// entering is whether band, coming into half, starts past a ring
	// there: a nearer band holds it, or the band is named and its name
	// wants the room.
	entering := func(half bool, band int) bool {
		return bands != nil && held[half] != band && (held[half] >= 0 || z.Named)
	}
	// nearest is the column in half a block stands nearest the pill in,
	// the first across among the nearest, and how far out it starts.
	nearest := func(half bool, band int) (int, float32) {
		if entering(half, band) {
			return across[0], slices.Max(fill[half]) + z.Ring
		}
		col := across[0]
		for _, c := range across[1:] {
			if fill[half][c] < fill[half][col] {
				col = c
			}
		}
		return col, fill[half][col]
	}
	rects, slots = make([]stageRect, len(heights)), make([]stageSlot, len(heights))
	for i, h := range heights {
		band := -1
		if bands != nil {
			band = bands[i]
		}
		s := unslotted
		if i < len(kept) && kept[i].col >= 0 && kept[i].col < z.Cols {
			s = kept[i]
		}
		if s.col < 0 {
			bc, bs := nearest(true, band)
			ac, as := nearest(false, band)
			switch {
			case i < z.Cols && bands == nil:
				// The first row goes across under the pill.
				s = stageSlot{across[i], true}
			case bs+h <= room[true]:
				s = stageSlot{bc, true}
			case as+h <= room[false]:
				s = stageSlot{ac, false}
			default:
				// Past what the window holds, the field goes on under the
				// pill, the way it is scrolled: over it stands no more than
				// the window holds there.
				s = stageSlot{bc, true}
			}
		}
		slots[i] = s
		half := s.below
		if entering(half, band) {
			line := slices.Max(fill[half])
			for j := range fill[half] {
				fill[half][j] = line + z.Ring
			}
			if z.Named {
				room := stageRect{X: left, Y: line, W: width, H: z.Ring}
				if !half {
					room.Y = -line - z.Ring
				}
				marks = append(marks, stageMark{Band: band, Below: half, Rect: room})
			}
		}
		if bands != nil {
			held[half] = band
		}
		x := left + float32(s.col)*(z.Col+z.Gap)
		if half {
			rects[i] = stageRect{X: x, Y: fill[half][s.col], W: z.Col, H: h}
		} else {
			rects[i] = stageRect{X: x, Y: -fill[half][s.col] - h, W: z.Col, H: h}
		}
		fill[half][s.col] += h + z.Gap
	}
	return rects, marks, slots
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
// §4.7.6): from its top down, left to right in rows of the columns, each
// row as tall as its tallest, the first starting at 0. A block wide says
// stands across the columns, alone in its row, as a divider does, the
// cards it heads close under it. A list does not stand on the pill: it
// reads from the window's top, and what grows or shrinks at its foot, as
// the ribbons, moves nothing in it.
func (z stageHorizon) Rows(heights []float32, wide []bool) []stageRect {
	width := float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap
	left := -width / 2
	out := make([]stageRect, len(heights))
	y, rowH, col := float32(0), float32(0), 0
	for i, h := range heights {
		if i > 0 && (col == z.Cols || wide[i] || wide[i-1]) {
			gap := z.Gap
			if wide[i-1] {
				gap /= 2
			}
			y, rowH, col = y+rowH+gap, 0, 0
		}
		out[i] = stageRect{X: left + float32(col)*(z.Col+z.Gap), Y: y, W: z.Col, H: h}
		if wide[i] {
			out[i].W = width
		}
		rowH = max(rowH, h)
		col++
	}
	return out
}
