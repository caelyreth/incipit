package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Inside a pair Tab leaves it and its key shows where it would land; out
// of it Shift-Tab goes back. With no scope, Tab nests a list item.
func TestTabLeavesAPairWithItsKeyShown(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "The tide turned (slowly" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Advance(time.Second)
	shot(t, tester, "guide-leave")
	tester.Key(0, ui.KeyTab)
	tester.Type(" and on.")
	tester.Frame()
	tester.Advance(time.Second)
	shot(t, tester, "guide-left")
	if got := pageText(app); got != "paragraph:The tide turned (slowly) and on." {
		t.Fatalf("page %q", got)
	}
}

// Backspace into a pair arms both its sides, drawn as what the next press
// takes; a mark's signs arm the same way.
func TestArmedSidesShow(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "Some (words) and *marks*" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Key(0, ui.KeyLeft)
	tester.Frame()
	tester.Advance(time.Second)
	shot(t, tester, "mark-leave-guide")
	for range 5 {
		tester.Key(0, ui.KeyLeft)
	}
	tester.Key(0, ui.KeyBackspace)
	tester.Frame()
	tester.Advance(time.Second)
	shot(t, tester, "mark-armed")
	if got := pageText(app); got != "paragraph:Some (words) and marks" && got != "paragraph:Some (words) and *marks*" {
		t.Logf("page %q", got)
	}
}
