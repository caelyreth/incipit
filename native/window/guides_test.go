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
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
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
