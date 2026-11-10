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

// Accessibility names each unit by its semantic text and maps every stop
// to a UTF-16 place in that text. Projected notation adds none.
func TestAccessibilityTextAndStops(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := vocab.EmptyPage(1, 4, doc.NewInterner())
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	now := time.Unix(1, 0)
	for _, s := range []string{"#", "#", " ", "T", "i", "t", "l", "e", "\n", "A", "😀", " ", "$", "x", "+", "1", "$"} {
		now = now.Add(time.Second)
		in := vocab.Intent(vocab.Type{Text: s})
		if s == "\n" {
			in = vocab.Break{}
		}
		if _, err := e.Do(1, vocab.Input{Intent: in, At: now}); err != nil {
			t.Fatal(err)
		}
	}
	units := e.Units(1)
	if len(units) != 2 {
		t.Fatalf("units: %d, want a heading and a paragraph", len(units))
	}
	ids := []doc.ID{units[0].ID, units[1].ID}
	pres := e.Present(1, ids)
	heading := pres[0]
	if heading.Role != "heading" || heading.Label != "Title" || heading.Level != 2 {
		t.Fatalf("heading accessibility = %q %q level %d", heading.Role, heading.Label, heading.Level)
	}
	para := pres[1]
	if para.Role != "paragraph" || para.Label != "A😀 x+1" || para.Value != "A😀 x+1" {
		t.Fatalf("paragraph accessibility = %q %q %q", para.Role, para.Label, para.Value)
	}
	if len(para.AccessStops) != len(para.Stops) {
		t.Fatalf("%d access stops for %d presentation stops", len(para.AccessStops), len(para.Stops))
	}
	last := -1
	for i, st := range para.AccessStops {
		if st.UTF16 < last {
			t.Fatalf("access stop %d goes back from %d to %d", i, last, st.UTF16)
		}
		last = st.UTF16
	}
	for i, st := range para.Stops {
		if st.Surface != 0 {
			continue
		}
		switch st.Offset {
		case 0:
			if para.AccessStops[i].UTF16 != 0 {
				t.Fatalf("start maps to %d", para.AccessStops[i].UTF16)
			}
		case 5:
			if para.AccessStops[i].UTF16 != 3 {
				t.Fatalf("after the emoji maps to %d, want 3", para.AccessStops[i].UTF16)
			}
		}
	}
}
