package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// mathPage opens a math block holding src after a paragraph and leaves
// the caret in its source.
func mathPage(t *testing.T, src string) (*Element, func() *unitLayout) {
	t.Helper()
	el := newElement(t)
	typeText(el, "above")
	el.Editor.Do(vocab.Break{}, time.Unix(200, 0))
	typeText(el, "$$")
	typeText(el, src)
	block := func() *unitLayout {
		for _, u := range el.relayout().units {
			if u.pres.Kind == profile.Math {
				return u
			}
		}
		t.Fatal("no math block")
		return nil
	}
	return el, block
}

// A math block shows its formula set for display; its source panel opens
// under it while the caret is in it, and folds away when the caret leaves.
func TestAMathBlockFoldsItsSourceAway(t *testing.T) {
	el, block := mathPage(t, `\frac{a}{b}`)
	u := block()
	if u.math == nil || u.math.box == nil {
		t.Fatal("the block's formula is not set")
	}
	if !u.math.held || u.h != u.math.full || u.math.full <= u.math.fold {
		t.Fatalf("held: height %v, fold %v, full %v", u.h, u.math.fold, u.math.full)
	}
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(300, 0))
	if u = block(); u.math.held || u.h != u.math.fold {
		t.Fatalf("left: held %v, height %v, fold %v", u.math.held, u.h, u.math.fold)
	}
}

// A math block is math from its first keystroke: empty, or with a source
// half typed, it is set as it stands, its panel open while it is held.
func TestAMathBlockIsMathFromItsFirstKeystroke(t *testing.T) {
	for _, src := range []string{"", `\`, `\frac{`} {
		_, block := mathPage(t, src)
		if u := block(); u.math == nil || u.math.box == nil || !u.math.held || u.h != u.math.full {
			t.Fatalf("%q: not set as math with its panel open", src)
		}
	}
}

// An inline formula sits on its line as an object as wide as its ink.
func TestInlineMathIsAnObjectInItsLine(t *testing.T) {
	el := newElement(t)
	typeText(el, "f $x^2$ g")
	s := el.Editor.layout.units[0].surfaces[0]
	for i, sg := range s.segs {
		if sg.kind != segBox {
			continue
		}
		r, ok := s.objectRect(i)
		if sg.math.box == nil || !ok {
			t.Fatal("the formula is not set as an object")
		}
		if w := sg.math.to - sg.math.from + 2*mathPad; r.W != w || r.H <= 0 {
			t.Fatalf("object %v, want width %v", r, w)
		}
		return
	}
	t.Fatal("no formula")
}
