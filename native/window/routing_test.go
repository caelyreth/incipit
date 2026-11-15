package window

import (
	"errors"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func submitPaletteCommand(t *testing.T, app *AppState, id keymap.CommandID) {
	t.Helper()
	app.Shell.OpenCommands()
	for index, command := range app.Palette.Filtered() {
		if command.ID != id {
			continue
		}
		app.Palette.Selected = index
		chosen, ok := app.Palette.Submit()
		if !ok || chosen != id {
			t.Fatalf("palette submit = %q,%v, want %q", chosen, ok, id)
		}
		return
	}
	t.Fatalf("%s missing from palette %#v", id, app.Palette.Commands)
}

func TestEveryCommandHasAHandler(t *testing.T) {
	for _, command := range appsettings.DefaultState().Keymap.Commands() {
		// The page answers the editing commands.
		if command.Scope != keymap.ScopeEditing && hostHandlers[command.ID] == nil {
			t.Errorf("%s has no handler", command.ID)
		}
	}
}

func TestPageLessNewPageRunsFromThePalette(t *testing.T) {
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
	submitPaletteCommand(t, app, keymap.NewPage)
	tester.Frame()
	if _, active := app.Workspace.Active(); !active {
		t.Fatal("page-less palette New Page did not open a page")
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

func TestRoutingRejectsUnknownAndAuxiliaryCommands(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.routeWindowCommand(nil, "missing.command"); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("unknown command = %v", err)
	}

	mainWindow := &mygo.Window{}
	settingsWindow := &mygo.Window{}
	app.mainWindow = mainWindow
	app.settingsWindow = settingsWindow
	var closed *mygo.Window
	app.closeWindow = func(window *mygo.Window) { closed = window }
	tabs := len(app.Workspace.Opened())
	if err := app.routeWindowCommand(settingsWindow, keymap.NewPage); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("auxiliary New Page = %v", err)
	}
	if got := len(app.Workspace.Opened()); got != tabs {
		t.Fatalf("auxiliary command changed tabs: %d -> %d", tabs, got)
	}
	if err := app.routeWindowCommand(settingsWindow, keymap.ClosePage); err != nil || closed != settingsWindow {
		t.Fatalf("auxiliary Close Sheet = %v, closed %v", err, closed)
	}
	if got := len(app.Workspace.Opened()); got != tabs {
		t.Fatalf("auxiliary Close Sheet closed a page: %d -> %d", tabs, got)
	}
}
