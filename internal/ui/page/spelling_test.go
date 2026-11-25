package page

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/policies/spelling"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// misspells is a checker that knows every word but tdie.
type misspells struct{}

func (misspells) Check(text string, _ bool) []spelling.Issue {
	if i := strings.Index(text, "tdie"); i >= 0 {
		return []spelling.Issue{{From: i, To: i + 4}}
	}
	return nil
}

// A secondary click on a misspelled word offers its corrections first,
// then learning and ignoring it; a correction writes itself over it.
func TestTheMenuOverAMisspelledWordOffersItsCorrections(t *testing.T) {
	spelling.Use(misspells{})
	t.Cleanup(func() { spelling.Use(nil) })
	el := newElement(t)
	el.Editor.Guesses = func(word string) []string {
		if word != "tdie" {
			t.Errorf("guesses asked for %q", word)
		}
		return []string{"tide", "tie"}
	}
	typeText(el, "Low tdie turns")
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	tester.Frame()
	ref, _ := vocab.FirstTextSlot(el.Editor.Engine.Snapshot().Root())
	at, size, _ := el.lookUpPlace(profile.LookUpAsk{Slot: ref, From: 4, To: 8})
	tester.RightClickAt(at.X+size/2, at.Y-size/3)
	tester.Advance(100 * time.Millisecond)
	items := tester.Menu()
	learn := slices.Index(items, "Learn Spelling")
	if len(items) < 2 || items[0] != "tide" || items[1] != "tie" || learn < 0 || !slices.Contains(items, "Ignore Spelling") {
		t.Fatalf("the menu offers %q", items)
	}
	if err := tester.ChooseMenuItem("tide"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(100 * time.Millisecond)
	n, _ := el.Editor.Engine.Snapshot().Node(ref.Node)
	if got := n.Slot(ref.Slot).Content().Flat(); got != "Low tide turns" {
		t.Fatalf("corrected to %q", got)
	}
	if head := el.Editor.Selection().Head.Pos.Offset; head != 8 {
		t.Fatalf("the caret stands at %d, want after tide", head)
	}
}
