package present_test

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/vocab"
)

// The stop map is total and monotonic, and a presentation is reused while
// its unit is unchanged.
func TestStopsAreOrderedAndUnitsMemoised(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := vocab.EmptyPage(1, 3, doc.NewInterner())
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	now := time.Unix(1, 0)
	for _, s := range []string{"a", "b", " ", "$", "x", "y"} {
		now = now.Add(time.Second)
		if _, err := e.Do(1, vocab.Input{Intent: vocab.Type{Text: s}, At: now}); err != nil {
			t.Fatal(err)
		}
	}
	units := e.Units(1)
	if len(units) != 1 {
		t.Fatalf("units: %d", len(units))
	}
	p := e.Present(1, []doc.ID{units[0].ID})[0]
	if len(p.Surfaces) != 2 || !p.Surfaces[1].Card {
		t.Fatalf("the formula's card is not open: %+v", p.Surfaces)
	}
	r := e.Snapshot()
	for i := 1; i < len(p.Stops); i++ {
		if doc.Compare(r, p.Stops[i-1].Caret, p.Stops[i].Caret) >= 0 {
			t.Fatalf("stops %d and %d out of order: %v %v", i-1, i, p.Stops[i-1].Caret, p.Stops[i].Caret)
		}
	}
	// "ab " then the box then its card's three stops, then after the box.
	if len(p.Stops) != 4+3+1 {
		t.Fatalf("stops: %d", len(p.Stops))
	}
	if again := e.Present(1, []doc.ID{units[0].ID})[0]; again != p {
		t.Fatal("an unchanged unit was presented again")
	}
}
