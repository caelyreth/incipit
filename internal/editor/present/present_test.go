package present_test

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/app/manifest"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// The stop map is total and monotonic, and a presentation is reused while
// its unit is unchanged.
func TestStopsAreOrderedAndUnitsMemoised(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := profile.EmptyPage(1, 3, doc.NewInterner())
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
	snap, _ := profile.EmptyPage(1, 4, doc.NewInterner())
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
	if heading.Role != "heading" || heading.Label() != "Title" || heading.Level != 2 {
		t.Fatalf("heading accessibility = %q %q level %d", heading.Role, heading.Label(), heading.Level)
	}
	para := pres[1]
	if para.Role != "paragraph" || para.Label() != "A😀 x+1" || para.Value() != "A😀 x+1" {
		t.Fatalf("paragraph accessibility = %q %q %q", para.Role, para.Label(), para.Value())
	}
	access := para.AccessOffsets()
	if len(access) != len(para.Stops) {
		t.Fatalf("%d access offsets for %d presentation stops", len(access), len(para.Stops))
	}
	last := int32(-1)
	for i, at := range access {
		if at < last {
			t.Fatalf("access offset %d goes back from %d to %d", i, last, at)
		}
		last = at
	}
	for i, st := range para.Stops {
		if st.Surface != 0 {
			continue
		}
		switch st.Offset() {
		case 0:
			if access[i] != 0 {
				t.Fatalf("start maps to %d", access[i])
			}
		case 5:
			if access[i] != 3 {
				t.Fatalf("after the emoji maps to %d, want 3", access[i])
			}
		}
	}
}

// An object's accessibility name comes from its role, not the renderer's
// empty-box placeholder.
func TestObjectAccessibilityUsesItsRole(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := profile.EmptyPage(1, 6, doc.NewInterner())
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	now := time.Unix(1, 0)
	for _, s := range []string{"-", "-", "-"} {
		now = now.Add(time.Second)
		if _, err := e.Do(1, vocab.Input{Intent: vocab.Type{Text: s}, At: now}); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(time.Second)
	if _, err := e.Do(1, vocab.Input{Intent: vocab.Break{}, At: now}); err != nil {
		t.Fatal(err)
	}
	units := e.Units(1)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	for _, p := range e.Present(1, ids) {
		if p.Kind == profile.Divider {
			if p.Role != "separator" || p.Label() != "" || p.Value() != "" {
				t.Fatalf("divider accessibility = %q %q %q", p.Role, p.Label(), p.Value())
			}
			return
		}
	}
	t.Fatal("the page has no divider")
}

// Every stop's accessible offset is the same found by the index as by
// scanning every part: through marks shown as ghosts, inline formulas and
// their open cards, code, CJK and text past the BMP.
func TestAccessibleOffsetsIndexMatchesScan(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := profile.EmptyPage(1, 3, doc.NewInterner())
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	now := time.Unix(1, 0)
	do := func(in vocab.Intent) {
		now = now.Add(time.Second)
		if _, err := e.Do(1, vocab.Input{Intent: in, At: now}); err != nil {
			t.Fatal(err)
		}
	}
	do(vocab.Paste{Text: "# Tides 🌊 and **moons**\n\nSome **strong** and *em* text (pairs [nested]) `code` 中文English, 😀 done... #tag\n\n- item **one**\n- item `two` #moon\n\n> quoted *words*\n\n```\nfn main() {}\n```\n"})
	for _, s := range []string{"a", " ", "$", "x", "^", "2", "$", " ", "b"} {
		do(vocab.Type{Text: s})
	}
	check := func(when string) {
		units := e.Units(1)
		ids := make([]doc.ID, len(units))
		for i, u := range units {
			ids[i] = u.ID
		}
		for _, p := range e.Present(1, ids) {
			if err := present.AccessMismatch(p); err != nil {
				t.Fatalf("%s, unit %s: %v", when, p.Unit, err)
			}
		}
	}
	check("caret after the formula")
	// Into the formula, which opens its card.
	for range 3 {
		do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	}
	check("caret in the formula")
}

// The list of units depends only on the page's blocks: typing keeps it,
// and a new block lists the units again.
func TestTypingKeepsTheUnits(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := profile.EmptyPage(1, 3, doc.NewInterner())
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	now := time.Unix(1, 0)
	do := func(in vocab.Intent) {
		t.Helper()
		now = now.Add(time.Second)
		if _, err := e.Do(1, vocab.Input{Intent: in, At: now}); err != nil {
			t.Fatal(err)
		}
	}
	do(vocab.Type{Text: "a"})
	do(vocab.Break{})
	before := e.Units(1)
	for _, s := range []string{"b", "c", "**", "d"} {
		do(vocab.Type{Text: s})
	}
	do(vocab.Delete{Dir: vocab.Backward})
	if after := e.Units(1); &after[0] != &before[0] {
		t.Fatal("typing listed the units again")
	}
	do(vocab.Break{})
	if after := e.Units(1); len(after) != len(before)+1 {
		t.Fatalf("after a new block: %d units, want %d", len(after), len(before)+1)
	}
}

// A unit sketched far from a renderer's view stands in with its look and
// no text, the same pointer while it is unchanged; a presentation no
// renderer asks for any more is let go.
func TestASketchStandsInWithoutTextAndPresentationsNotAskedForGo(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := profile.EmptyPage(1, 3, doc.NewInterner())
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(1)
	if _, err := e.Do(1, vocab.Input{Intent: vocab.Paste{Text: "one\n\ntwo\n"}, At: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	units := e.Units(1)
	ids := []doc.ID{units[0].ID, units[len(units)-1].ID}
	sk := e.Sketch(1)
	if len(sk) != len(units) || sk[0] == nil || !sk[0].Sketch || len(sk[0].Surfaces) != 0 || len(sk[0].Stops) != 0 || sk[0].Unit != ids[0] {
		t.Fatalf("sketch %+v", sk[0])
	}
	if again := e.Sketch(1); again[0] != sk[0] || again[len(again)-1] != sk[len(sk)-1] {
		t.Fatal("an unchanged unit's sketch is a new pointer")
	}
	first := e.Present(1, ids[:1])[0]
	if again := e.Present(1, ids[:1])[0]; again != first {
		t.Fatal("an unchanged unit was presented again")
	}
	for range 100 {
		e.Present(1, ids[1:])
	}
	if again := e.Present(1, ids[:1])[0]; again == first {
		t.Fatal("a presentation not asked for in 100 calls was kept")
	}
}
