package window

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/egoist/mygo/ui"
)

func catalogCommand(t *testing.T, commands []shell.Command, id keymap.CommandID) shell.Command {
	t.Helper()
	for _, command := range commands {
		if command.ID == string(id) {
			return command
		}
	}
	t.Fatalf("command %q missing from %#v", id, commands)
	return shell.Command{}
}

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

func TestCommandsUseLiveBindings(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	moved := keymap.NewChord("g", keymap.ModifiersPrimary|keymap.ModifiersAlt|keymap.ModifiersShift)
	app.Settings.Apply(appsettings.Remap(keymap.Settings, moved))

	command := catalogCommand(t, app.commandCatalog(), keymap.Settings)
	if want := keymap.Spell(moved, keymap.PlatformCurrent); command.Binding != want {
		t.Fatalf("binding = %q, want %q", command.Binding, want)
	}
	if want := keymap.Keycaps(moved, keymap.PlatformCurrent); !slices.Equal(command.Keys, want) {
		t.Fatalf("keys = %q, want %q", command.Keys, want)
	}

	app.Settings.Apply(appsettings.Unbind(keymap.Settings))
	command = catalogCommand(t, app.commandCatalog(), keymap.Settings)
	if command.Binding != "" || len(command.Keys) != 0 {
		t.Fatalf("unbound command still shows %q %q", command.Binding, command.Keys)
	}
}

func TestCommandsStayWithoutAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	before := len(app.commandCatalog())
	app.closeActive()
	commands := app.commandCatalog()
	catalogCommand(t, commands, keymap.NewPage)
	catalogCommand(t, commands, keymap.Settings)
	if len(commands) != before {
		t.Fatalf("page-less catalog has %d commands, want %d", len(commands), before)
	}
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

func TestSearchCommandsWritesThePill(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()

	app.Shell.SearchCommands()
	run(tester, time.Second)
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("the pill did not take the keyboard")
	}
	tester.Type(" zen")
	tester.Frame()
	if got := app.Shell.FieldQuery(); got != "> zen" {
		t.Fatalf("pill = %q", got)
	}
}

func TestNewPageButtonOpensAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	run(tester, time.Second)
	before := len(app.Workspace.Opened())
	if err := tester.Click("New sheet"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if got := len(app.Workspace.Opened()); got != before+1 {
		t.Fatalf("open pages after New sheet = %d, want %d", got, before+1)
	}
}

// The Settings command brings the field up on the settings categories,
// opening a section shows its category's live cards working in place, and
// the command again takes them away.
func TestTheSettingsCommandShowsTheSettingsInTheField(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	if err := app.routeCommand(keymap.Settings); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(2 * time.Second)
	if !app.Shell.InSettings() || !tester.HasText("Appearance") || !tester.HasText("Editor") || !tester.HasText("Keymap") || !tester.HasText("Theme") {
		t.Fatalf("settings categories not in the field: %#v", tester.Texts())
	}
	if err := tester.Click("Writing Environment"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(2 * time.Second)
	if !tester.HasText("Typewriter mode") {
		t.Fatalf("section card not opened: %#v", tester.Texts())
	}
	typewriter := app.Product.Presentation.Typewriter
	toggle, ok := tester.AccessByRole(ui.RoleSwitch, "Typewriter mode")
	if !ok {
		t.Fatal("no Typewriter mode switch")
	}
	tester.ClickAt(toggle.Bounds.X+toggle.Bounds.W/2, toggle.Bounds.Y+toggle.Bounds.H/2)
	tester.Frame()
	if app.Product.Presentation.Typewriter == typewriter {
		t.Fatal("the switch in the field did not change the setting")
	}
	if err := app.routeCommand(keymap.Settings); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(2 * time.Second)
	if app.Shell.InSettings() || tester.HasText("Typewriter mode") {
		t.Fatalf("settings stayed: %#v", tester.Texts())
	}
}
