package window

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/app/keymap"
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

// While open the switcher holds the keyboard: the pointer passing over
// the columns brings none in front, and a press on the desk around it
// puts it away and does nothing else.
func TestTheSwitcherHoldsTheKeyboard(t *testing.T) {
	app, tester := deskApp(t)
	app.Shell.ShowDesk()
	run(tester, time.Second)
	front := app.Shell.ActiveColumn
	if err := app.routeCommand(keymap.FindPage); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	for _, x := range []float32{300, 1100} {
		tester.Move(x, 300)
		run(tester, 300*time.Millisecond)
	}
	if app.Shell.ActiveColumn != front {
		t.Fatal("The pointer passing over a column brought it in front under the switcher.")
	}
	if !tester.Focused("Go to sheet") {
		t.Fatal("The switcher's field lost the keyboard to the pointer.")
	}
	x := float32(1100)
	if app.Shell.Open[len(app.Shell.Open)-1].Column == front {
		x = 300
	}
	tester.ClickAt(x, 300)
	run(tester, time.Second)
	if _, ok := tester.Find("Go to sheet"); ok {
		t.Fatal("A press on the desk left the switcher open.")
	}
	if app.Shell.ActiveColumn != front {
		t.Fatal("The press that put the switcher away went on to the column under it.")
	}
}

// A sheet closed from the desk is listed as closed lately.
func TestTheSwitcherListsSheetsClosedLately(t *testing.T) {
	app, tester := deskApp(t)
	app.Shell.ShowDesk()
	run(tester, time.Second)
	if _, ok := tester.Find("Closed lately"); ok {
		t.Fatal("Nothing was closed, but the switcher lists something closed.")
	}
	app.Shell.CloseColumnID(app.Shell.Open[0].Column)
	run(tester, time.Second)
	if err := app.routeCommand(keymap.FindPage); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	if _, ok := tester.Find("Closed lately"); !ok {
		t.Fatalf("The sheet closed is not listed as closed lately: %v", tester.Texts())
	}
}

// With a sheet alone on the desk, which shows no overview, Go to Sheet
// grows the switcher from closed all the same.
func TestTheSwitcherGrowsWithASheetAlone(t *testing.T) {
	app, tester := deskApp(t)
	app.Shell.CloseColumnID(app.Shell.Open[0].Column)
	app.Shell.ShowDesk()
	run(tester, 2*time.Second)
	if err := app.routeCommand(keymap.FindPage); err != nil {
		t.Fatal(err)
	}
	width := func() float32 {
		t.Helper()
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
		got, ok := tester.AccessByRole(ui.RoleGroup, "Go to sheet")
		if !ok {
			t.Fatal("Go to Sheet did not open the switcher.")
		}
		return got.Bounds.W
	}
	first := width()
	run(tester, time.Second)
	if open := width(); first >= open/2 {
		t.Fatalf("The switcher opened %v wide in its first frame, %v at rest: it did not grow.", first, open)
	}
}
