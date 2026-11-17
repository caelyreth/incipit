package shell

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// Resized a little at a time, the field stands each frame as its new
// measures lay it out: nothing springs from where it stood, so no card is
// drawn scaled or over another, and the cards are laid out again only a
// step at a time.
func TestResizingLaysTheFieldOutAtOnce(t *testing.T) {
	state, library, tester := onField(t, 1400, 900)
	groups := []GroupItem{{ID: "1", Name: "Inbox", Inbox: true}}
	for i := range 12 {
		groups = append(groups, GroupItem{ID: fmt.Sprint(100 + i), Name: fmt.Sprintf("Shelf %d", i+1)})
	}
	library.groupTree = map[string][]GroupItem{"": groups}
	library.extraSheets = 6
	library.rev++
	tester.ClickAt(12, 12)
	settle(tester)
	layouts := 0
	for w := 1400; w >= 1000; w -= 2 {
		before := state.field.shapesKey
		tester.SetSize(w, 900)
		tester.Frame()
		if state.field.shapesKey != before {
			layouts++
		}
		if !slices.Equal(state.field.drawn, state.field.rest) {
			t.Fatalf("%d wide, the field should stand as laid out, not on its way there.", w)
		}
		for _, fr := range state.field.drawn {
			if fr.Rect.W != fr.Size.W || fr.Rect.H != fr.Size.H {
				t.Fatalf("%d wide, %s should be drawn at its own size: %v in %v.", w, fr.ID, fr.Size, fr.Rect)
			}
		}
		tester.Advance(16 * time.Millisecond)
	}
	if layouts > 60 {
		t.Fatalf("200 steps of 2 pt should lay the cards out again a step at a time, not %d times.", layouts)
	}
}
