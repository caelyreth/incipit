package page

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// A force click on a word shows what it means where the page drew it, on
// its baseline in the page's type, and leaves the selection as it was; on
// no word, it looks nothing up.
func TestAForceClickLooksUpTheWordUnderIt(t *testing.T) {
	el := newElement(t)
	typeText(el, "Low tide turns")
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	tester.Frame()
	before := el.Editor.Selection()
	ref, _ := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	at, size, ok := el.lookUpPlace(profile.LookUpAsk{Slot: ref, From: 4, To: 8})
	if !ok || size <= 0 {
		t.Fatalf("tide stands nowhere: %v %v %v", at, size, ok)
	}
	// Bounds are the previous frame's: the page stands at the window's
	// origin in the Tester.
	tester.LookUp(at.X+size/2, at.Y-size/3)
	tester.Frame()
	got := tester.Definitions()
	if len(got) != 1 || got[0].Text != "tide" || got[0].At != at || got[0].Font.Size != size || got[0].Font.Family != el.Prose.Family {
		t.Fatalf("looked up %+v; want tide at %v in %v %s", got, at, size, el.Prose.Family)
	}
	if sel := el.Editor.Selection(); sel != before {
		t.Fatalf("the selection moved to %+v from %+v", sel, before)
	}
	tester.LookUp(at.X-2, at.Y-size/3) // the space before it
	tester.Frame()
	if len(tester.Definitions()) != 1 {
		t.Fatalf("a force click on a space looked up %+v", tester.Definitions()[1:])
	}
}

// A secondary click on text readies Look Up for the word clicked, as a
// text view's: outside the selection, the caret goes there; inside it,
// the selection stays.
func TestASecondaryClickOnTextOffersLookUp(t *testing.T) {
	el := newElement(t)
	typeText(el, "Low tide turns")
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	tester.Frame()
	ref, _ := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	at, size, _ := el.lookUpPlace(profile.LookUpAsk{Slot: ref, From: 4, To: 8})
	x, y := el.toPage(at.X+size/2, at.Y-size/3)
	el.readyMenu(x, y, time.Unix(500, 0))
	if got := el.menuGroups(); len(got) == 0 || got[0][0] != profile.LookUpCommand {
		t.Fatalf("a secondary click on a word offers %v, want Look Up first", got)
	}
	if head := el.Editor.Selection().Head.Pos.Offset; head < 4 || head > 8 {
		t.Fatalf("the caret stands at %d, want in tide", head)
	}
	el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 0)), Clicks: 1}, time.Unix(501, 0))
	el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 14)), Clicks: 1, Extend: true}, time.Unix(502, 0))
	selected := el.Editor.Selection()
	el.readyMenu(x, y, time.Unix(503, 0))
	if sel := el.Editor.Selection(); sel != selected {
		t.Fatalf("a secondary click in the selection moved it to %+v", sel)
	}
}
