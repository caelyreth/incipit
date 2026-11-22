package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/egoist/mygo/ui"
)

// Go to Sheet on the desk grows the overview open into the switcher and
// stays on the desk: Return goes to the column the keys are on, first the
// one beside the sheet in front; ⌘Return opens it beside, split again;
// Escape puts it away.
func TestGoToSheetSwitchesOnTheDesk(t *testing.T) {
	app, tester := deskApp(t)
	app.Shell.ShowDesk()
	run(tester, time.Second)
	front := app.Shell.ActiveColumn
	other := ""
	for _, page := range app.Shell.Open {
		if page.Column != front {
			other = page.Column
		}
	}
	open := func() {
		t.Helper()
		if err := app.routeCommand(keymap.FindPage); err != nil {
			t.Fatal(err)
		}
		run(tester, time.Second)
		if !app.Shell.OnDesk() {
			t.Fatal("Go to Sheet left the desk.")
		}
		if _, ok := tester.Find("Go to sheet"); !ok {
			t.Fatalf("Go to Sheet on the desk did not open the switcher: %v", tester.Texts())
		}
	}

	open()
	tester.Key(0, ui.KeyEnter)
	run(tester, time.Second)
	if app.Shell.ActiveColumn != other {
		t.Fatalf("Return went to %s, want the column beside the one in front, %s.", app.Shell.ActiveColumn, other)
	}
	if _, ok := tester.Find("Go to sheet"); ok {
		t.Fatal("The switcher stayed after Return.")
	}

	open()
	tester.Key(ui.Cmd, ui.KeyEnter)
	run(tester, time.Second)
	if len(app.Shell.Open) != 3 {
		t.Fatalf("⌘Return on an open column left %d columns, want it split beside.", len(app.Shell.Open))
	}

	open()
	tester.Key(0, ui.KeyEscape)
	run(tester, time.Second)
	if _, ok := tester.Find("Go to sheet"); ok {
		t.Fatal("Escape left the switcher open.")
	}
}
