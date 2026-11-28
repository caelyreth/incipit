package shell

import (
	"fmt"
	"slices"
)

// The field's columns are chosen as the query lands and held while it
// stands (field-and-desk.md §5.2): each block keeps its column and its
// side of the pill while what it holds grows or shrinks, so a change to
// one block moves only what is stacked beyond it in its own half of its
// column. They are chosen again, all of them, as the query, the columns
// or the bands change, or as what the field holds is ranked anew; not as
// the window's height alone changes, which would move blocks from one
// side of the pill to the other at every step of a resize; while what was chosen measures itself, over the frames
// after it lands, they are chosen again from what it measures, until it
// stands a frame. What leaves the field without the rest being ranked
// anew, as a sheet trashed or set aside in the tray, leaves its column,
// the rest holding theirs, and so does what comes in ranked past all
// that stays, taking a column of its own; what comes in nearer the pill
// than what stays would push it out, and the columns are chosen anew.

// slotKey is what the field's columns are chosen anew for.
type slotKey struct {
	query string
	cols  int
	bands string
}

// placing is the field's columns as chosen: slots by block id, for the
// blocks ids at heights as last stood; base those held through a choice
// still under way, nil for a choice made anew; open whether one is.
type placing struct {
	key     slotKey
	ids     []string
	heights []float32
	slots   map[string]stageSlot
	base    map[string]stageSlot
	open    bool
}

// slotted stands f's cards of heights about the horizon z, in the columns
// held for them, chosen anew as placing says.
func (s *State) slotted(z stageHorizon, f found, heights []float32, bands []int) ([]stageRect, []stageMark) {
	p := &s.field.placing
	ids := make([]string, len(f.cards))
	for i, cd := range f.cards {
		ids[i] = cd.id
	}
	key := slotKey{query: f.query, cols: z.Cols, bands: fmt.Sprint(bands)}
	switch {
	case key != p.key || p.slots == nil:
		p.key, p.base, p.open = key, nil, true
	case !slices.Equal(ids, p.ids):
		p.base, p.open = nil, true
		if staysAhead(p.ids, ids) {
			p.base = p.slots
		}
	case p.open && !slices.Equal(heights, p.heights):
		// Still measuring itself: chosen again from what it measures.
	default:
		p.open = false
		p.base = p.slots
	}
	kept := make([]stageSlot, len(ids))
	for i, id := range ids {
		kept[i] = unslotted
		if at, ok := p.base[id]; ok {
			kept[i] = at
		}
	}
	rects, marks, slots := z.Place(heights, bands, kept)
	p.ids, p.heights = ids, slices.Clone(heights)
	p.slots = make(map[string]stageSlot, len(ids))
	for i, id := range ids {
		p.slots[id] = slots[i]
	}
	return rects, marks
}

// staysAhead is whether b is a with some ids gone and some come in after
// all that stays: nothing ranked anew, nothing come in nearer.
func staysAhead(a, b []string) bool {
	in := func(ids []string) map[string]bool {
		m := make(map[string]bool, len(ids))
		for _, id := range ids {
			m[id] = true
		}
		return m
	}
	inA, inB := in(a), in(b)
	var x, y []string
	for _, id := range a {
		if inB[id] {
			x = append(x, id)
		}
	}
	come := false
	for _, id := range b {
		switch {
		case !inA[id]:
			come = true
		case come:
			return false
		default:
			y = append(y, id)
		}
	}
	return slices.Equal(x, y)
}
