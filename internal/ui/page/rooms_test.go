package page

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// A page at rest steps no room as it is painted, and a room that opens
// after a long rest still springs open from where it stood rather than
// jumping there.
func TestARoomOpeningAfterRestSprings(t *testing.T) {
	el := newElement(t)
	typeText(el, "Let us say $x+1$ and more")
	tt := ui.NewTester(el.View, 900, 600)
	tt.Advance(5 * time.Second)
	if !el.roomsSettled {
		t.Fatal("the rooms of a page at rest are not settled")
	}
	unit := func() *unitLayout { return el.Editor.layout.units[0] }
	closed := unit().shown
	// Into the formula, which opens its card under it.
	for range len(" and more") + 1 {
		el.Editor.Do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}, time.Now())
	}
	tt.Frame()
	var between bool
	for range 60 {
		tt.Advance(16 * time.Millisecond)
		u := unit()
		if u.shown > closed && u.shown < u.h {
			between = true
		}
	}
	if u := unit(); !between || u.h <= closed || u.shown != u.h {
		t.Fatalf("the card's room went %v → %v (shown %v) without springing", closed, u.h, u.shown)
	}
}
