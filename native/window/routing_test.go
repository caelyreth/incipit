package window

import (
	"errors"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/egoist/mygo/ui"
)

func runListedCommand(t *testing.T, app *AppState, id keymap.CommandID) {
	t.Helper()
	for _, command := range app.Shell.Commands() {
		if command.ID == string(id) {
			app.Shell.RunCommand(command.ID)
			return
		}
	}
	t.Fatalf("%s missing from the commands", id)
}

func TestEveryCommandHasAHandler(t *testing.T) {
	for _, command := range appsettings.DefaultState().Keymap.Commands() {
		// The page answers the editing commands.
		if command.Scope != keymap.ScopeEditing && hostHandlers[command.ID] == nil {
			t.Errorf("%s has no handler", command.ID)
		}
	}
}

func TestPageLessNewPageRunsFromThePill(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	app.closeActive()
	tester.Frame()
	if _, active := app.Workspace.Active(); active {
		t.Fatal("the fixture still has an active page")
	}
	runListedCommand(t, app, keymap.NewPage)
	tester.Frame()
	if _, active := app.Workspace.Active(); !active {
		t.Fatal("page-less New Page did not open a page")
	}
}

// A key with no menu item behind it is heard by the window itself.
func TestUnmenuedKeysRunTheirCommand(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if menuCommands[keymap.Typewriter] {
		t.Fatal("typewriter is carried by a menu item")
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	modifiers, key, ok := keymap.Native(chordFor(t, app, keymap.Typewriter), keymap.PlatformCurrent)
	if !ok {
		t.Fatal("typewriter key has no native form")
	}
	tester.Key(modifiers, key)
	tester.Frame()
	if !app.Product.Presentation.Typewriter || !app.Shell.Toolbar.Typewriter {
		t.Fatal("typewriter key did not turn typewriter mode on")
	}
}

func TestRoutingRejectsUnknownCommands(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.routeCommand("missing.command"); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("unknown command = %v", err)
	}
}
