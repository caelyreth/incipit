package manifest

import (
	"reflect"
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// Decorations over the presentations a renderer has just pulled are the
// decorations over every unit: a selection, pair and mark matches, pulses
// and guides alike.
func TestDecorationsOfPulledPresentationsAreTheViewsDecorations(t *testing.T) {
	p := open(t)
	p.do(vocab.Paste{Text: "# Head\n\nSome **strong** text (pairs [nested]) and a #tag...\n\n- item\n- two\n"})
	same := func(when string) {
		units := p.e.Units(1)
		ids := make([]doc.ID, len(units))
		for i, u := range units {
			ids[i] = u.ID
		}
		got := p.e.DecorationsOf(1, p.e.Present(1, ids))
		if want := p.e.Decorations(1); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s:\n got %+v\nwant %+v", when, got, want)
		}
	}
	same("after the paste")
	for range 12 {
		p.do(left)
	}
	same("in the marks")
	p.do(extend(left))
	p.do(extend(left))
	same("with a range")
	p.do(vocab.Type{Text: "-"})
	p.do(vocab.Type{Text: "-"})
	same("after a rewrite")
}
