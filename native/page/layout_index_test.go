package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// A caret's place is found through its unit, as an index of every stop of
// the page in page order would find it: text, cards, a heading's marker,
// two figures side by side and the gap they share. A unit a keystroke did
// not change keeps its index.
func TestEveryStopIsFoundThroughItsUnit(t *testing.T) {
	el := newElement(t)
	typeText(el, "## Head $x+1$ and **b**")
	el.Editor.Do(vocab.Break{}, time.Unix(100, 0))
	pic := profile.Picture{Media: "m", Width: 400, Height: 300}
	el.Editor.Do(profile.PlaceFigures{Pictures: []profile.Picture{pic, pic}}, time.Unix(200, 0))
	typeText(el, "after $y$")
	pl := el.Editor.Relayout()

	want := map[doc.Caret]place{}
	var first, last *stopAt
	for _, u := range pl.units {
		for _, s := range orderedStops(u) {
			if _, dup := want[s.stop.caret]; !dup {
				want[s.stop.caret] = place{unit: u, surface: s.surface, stop: s.stop}
			}
			if first == nil {
				first = s.stop
			}
			last = s.stop
		}
	}
	gaps := 0
	for c, w := range want {
		if got, ok := pl.lookup(c); !ok || got != w {
			t.Fatalf("%v is at %+v, %v, want %+v", c, got, ok, w)
		}
		if w.surface == nil {
			gaps++
		}
	}
	if gaps < 2 || pl.first != first || pl.last != last {
		t.Fatalf("%d gaps, first %v last %v, want at least 2 gaps, first %v last %v", gaps, pl.first, pl.last, first, last)
	}

	head := pl.units[0]
	typeText(el, "z")
	pl = el.Editor.Relayout()
	if pl.units[0] != head || head.at == nil {
		t.Fatal("the heading was laid out or indexed again")
	}
}
