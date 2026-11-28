package shell

import (
	"fmt"
	"slices"
	"strings"
)

// The field's columns are a function of what it shows: chosen for the
// query, the cards it finds in their order, the columns and the bands
// (field-and-desk.md §5.2), so the same field stands the same however it
// came to be, and undoing a change, as a sheet made and deleted, puts
// it back. Chosen, they are held while what the cards hold grows or
// shrinks: a block keeps its column and its side of the pill, so a
// change within one block moves only what is stacked beyond it in its
// own half of its column, and moves it back as it undoes. Over the
// frames after they are chosen, while what was chosen measures itself
// (the settings' sections), they are chosen again from what it measures,
// until it stands a frame. The window's height alone chooses nothing
// anew, which would move blocks from one side of the pill to the other
// at every step of a resize.

// slotKey is what the field's columns are chosen for.
type slotKey struct {
	query string
	cols  int
	bands string
	ids   string
}

// placing is the field's columns as chosen: slots by block id, at the
// heights the blocks last stood; open whether they are still being
// chosen as the blocks measure themselves.
type placing struct {
	key     slotKey
	heights []float32
	slots   []stageSlot
	open    bool
}

// slotted stands f's cards of heights about the horizon z, in the columns
// chosen for them as placing says.
func (s *State) slotted(z stageHorizon, f found, heights []float32, bands []int) ([]stageRect, []stageMark) {
	p := &s.field.placing
	ids := make([]string, len(f.cards))
	for i, cd := range f.cards {
		ids[i] = cd.id
	}
	key := slotKey{query: f.query, cols: z.Cols, bands: fmt.Sprint(bands), ids: strings.Join(ids, "\x00")}
	switch {
	case key != p.key:
		p.key, p.open = key, true
	case p.open && !slices.Equal(heights, p.heights):
		// Still measuring itself: chosen again from what it measures.
	default:
		p.open = false
	}
	var kept []stageSlot
	if !p.open {
		kept = p.slots
	}
	rects, marks, slots := z.Place(heights, bands, kept)
	p.heights, p.slots = slices.Clone(heights), slots
	return rects, marks
}
