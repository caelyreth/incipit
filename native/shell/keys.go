package shell

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The keys go along the field two ways (field-and-desk.md §4.7.7): the
// arrows by place, to the row nearest in their direction, and Tab and
// Shift-Tab in the field's order, which is its rank. Up and Down put the
// keys on the field; once they are on a row, Left and Right go by place
// too, and Space selects it, where writing would have moved the caret or
// written a space (spaceKey). Writing anything changes the query, which
// lets the row go (picks.go). Return opens the row; Cmd-Return opens it
// alongside.

// cardPlace is where the card being drawn stands in the field, its scale,
// and its box as laid out, before it is moved there.
type cardPlace struct {
	rect  stageRect
	scale float32
	box   ui.Rect
}

// rowPlace keeps where the row id, drawn as el in the card being drawn,
// stands in the field, as it was last laid out.
func (s *State) rowPlace(el ui.Element, id string) {
	b, card := el.LayoutBounds(), s.field.drawing
	if b.W <= 0 || card.box.W <= 0 {
		return
	}
	if s.field.rowRects == nil {
		s.field.rowRects = map[string]stageRect{}
	}
	s.field.rowRects[id] = stageRect{
		X: card.rect.X + (b.X-card.box.X)*card.scale, Y: card.rect.Y + (b.Y-card.box.Y)*card.scale,
		W: b.W * card.scale, H: b.H * card.scale,
	}
}

// keepRowPlaces lets go of where rows stood that the field no longer
// shows.
func (s *State) keepRowPlaces() {
	shown := make(map[string]bool, len(s.field.order))
	for _, id := range s.field.order {
		shown[id] = true
	}
	for id := range s.field.rowRects {
		if !shown[id] {
			delete(s.field.rowRects, id)
		}
	}
}

// unbuilt stands the rows of cd, a card not built, where the card does.
func (s *State) unbuilt(cd card, r stageRect) {
	if s.field.rowRects == nil {
		s.field.rowRects = map[string]stageRect{}
	}
	for _, id := range cd.rows {
		s.field.rowRects[id] = r
	}
}

// chooseToward moves the keys to the row nearest the one they are on in
// the direction dx, dy, by place; on none yet, to the first. Along the
// way, a row across from it counts for more than one off to the side.
func (s *State) chooseToward(dx, dy float32) {
	from, ok := s.field.rowRects[s.field.chosen]
	if !ok {
		s.choose(0)
		return
	}
	best, bestCost := "", float32(0)
	for _, id := range s.field.order {
		r, ok := s.field.rowRects[id]
		if !ok || id == s.field.chosen {
			continue
		}
		// How far it is along the way, from edge to edge, and how far off
		// to the side, 0 where they overlap.
		var along, aside float32
		switch {
		case dy > 0:
			along, aside = r.Y-(from.Y+from.H), gapBetween(from.X, from.W, r.X, r.W)
		case dy < 0:
			along, aside = from.Y-(r.Y+r.H), gapBetween(from.X, from.W, r.X, r.W)
		case dx > 0:
			along, aside = r.X-(from.X+from.W), gapBetween(from.Y, from.H, r.Y, r.H)
		default:
			along, aside = from.X-(r.X+r.W), gapBetween(from.Y, from.H, r.Y, r.H)
		}
		if along < -1 {
			continue
		}
		if cost := max(along, 0) + 3*aside; best == "" || cost < bestCost {
			best, bestCost = id, cost
		}
	}
	if best != "" {
		s.field.chosen = best
	}
}

// gapBetween is the room between the spans a and b, from at, as long, 0
// where they overlap.
func gapBetween(a, aLen, b, bLen float32) float32 {
	return max(0, max(b-(a+aLen), a-(b+bLen)))
}

// spaceKey takes Space back from the pill's words, where the input wrote
// it, while the keys are on a group or a sheet, and selects it or lets it
// go, where it can be: the input shows no key that writes text before it
// does.
func (s *State) spaceKey() {
	filed := strings.HasPrefix(s.field.chosen, "group:") || strings.HasPrefix(s.field.chosen, "sheet:")
	if s.field.words == s.field.wordsBefore+" " && filed {
		s.pickToggle(s.field.chosen)
		s.setWords(s.field.wordsBefore)
	}
	s.field.wordsBefore = s.field.words
}

// fieldKey is what a key does to the field from the pill, before the pill
// takes it as writing, reporting whether it took it.
func (s *State) fieldKey(key ui.TextKey) bool {
	onRow := s.field.chosen != ""
	// ⌥ and the arrows move the row along the list's order, as filed.
	if key.Mods == ui.Alt {
		switch key.Key {
		case ui.KeyLeft, ui.KeyUp:
			return s.nudge(-1)
		case ui.KeyRight, ui.KeyDown:
			return s.nudge(1)
		}
	}
	switch key.Key {
	case ui.KeyTab:
		switch key.Mods {
		case 0:
			s.choose(1)
		case ui.Shift:
			s.choose(-1)
		default:
			return false
		}
	case ui.KeyUp, ui.KeyDown:
		if key.Mods != 0 {
			return false
		}
		s.chooseToward(0, map[ui.Key]float32{ui.KeyUp: -1, ui.KeyDown: 1}[key.Key])
	case ui.KeyLeft, ui.KeyRight:
		if key.Mods != 0 || !onRow {
			return false
		}
		s.chooseToward(map[ui.Key]float32{ui.KeyLeft: -1, ui.KeyRight: 1}[key.Key], 0)
	default:
		return false
	}
	return true
}
