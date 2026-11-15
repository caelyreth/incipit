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

// A page widened past its measure lays out again only what shows into its
// margins: its text keeps its shaped lines, and a table, which may reach
// into the margins, is laid out again.
func TestWideningPastTheMeasureLaysOutOnlyTheMargins(t *testing.T) {
	el, table := tablePage(t, 2, 2, "a", "b")
	units := func() (text, grid *unitLayout) {
		for _, u := range el.relayout().units {
			if u.table == nil {
				text = u
			}
		}
		return text, table()
	}
	text, grid := units()
	measure := el.Editor.metrics.Width
	el.viewport.W += 200
	text2, grid2 := units()
	if el.Editor.metrics.Width != measure {
		t.Fatalf("the measure changed from %v to %v", measure, el.Editor.metrics.Width)
	}
	if text2 != text {
		t.Fatal("the text was laid out again though its measure stayed")
	}
	if grid2 == grid {
		t.Fatal("the table kept a layout from narrower margins")
	}
}
