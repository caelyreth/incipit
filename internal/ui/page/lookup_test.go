package page

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
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
