package window

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/app/keymap"
	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	"github.com/caelyreth/incipit/internal/ui/shell"
	"github.com/egoist/mygo"
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
	app.closeActive()
	commands := app.commandCatalog()
	catalogCommand(t, commands, keymap.NewPage)
	catalogCommand(t, commands, keymap.Settings)
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

// Every command but the page's has a home in the menu bar, where its key
// is shown and Help finds it.
func TestEveryCommandIsInTheMenuBar(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.MainMenu()
	for _, command := range app.Product.Keymap.Commands() {
		if command.Scope == keymap.ScopeEditing {
			continue
		}
		switch command.ID {
		case keymap.CheckSpelling, keymap.CheckGrammar, keymap.CorrectSpelling, keymap.PredictSpelling:
			if !mygo.Spelling.Available() {
				continue
			}
		}
		if app.menuItems[command.ID] == nil {
			t.Errorf("%s has no menu item", command.ID)
		}
	}
}

// A key acts on what is in front: on the field, Close Sheet closes
// nothing on the desk behind it.
func TestKeysActOnTheSceneInFront(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	app.Shell.ShowField()
	tester.Frame()
	if app.commandState(keymap.ClosePage, "").Enabled {
		t.Fatal("Close Sheet is enabled on the field")
	}
	app.MainMenu()
	if item := app.menuItems[keymap.Trash].item; item.Accelerator == "" {
		t.Fatal("Move to Trash has no key on the field")
	}
	app.Shell.ShowDesk()
	tester.Frame()
	if item := app.menuItems[keymap.Trash].item; item.Accelerator != "" {
		t.Fatalf("Move to Trash keeps %q on the desk, where the page hears it", item.Accelerator)
	}
	if len(app.offerItems) == 0 {
		t.Fatal("The Format menu holds nothing the rules offer.")
	}
	for _, f := range app.offerItems {
		if f.id == "figures.insert" || f.id == "formats.strong" || f.id == "blocks.heading1" {
			if f.item.Disabled {
				t.Fatalf("Format item %q is disabled at a caret on the desk", f.item.Label)
			}
		}
	}
	allDisabled := func(where string) {
		t.Helper()
		for _, f := range app.offerItems {
			if !f.item.Disabled {
				t.Fatalf("Format item %q is enabled %s", f.item.Label, where)
			}
		}
	}

	app.Shell.ShowField()
	tester.Frame()
	allDisabled("on the field")

	app.Shell.ShowDesk()
	app.closeActive()
	tester.Frame()
	allDisabled("with no active sheet")
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
	tester.Type(" typewriter")
	tester.Frame()
	if got := app.Shell.FieldQuery(); got != "> typewriter" {
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
	if !app.Shell.InSettings() || !tester.HasText("Appearance") || !tester.HasText("Editor") || !tester.HasText("Keymap") || !tester.HasText("Interface") {
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

// A sheet made and closed with nothing in it goes, rather than piling up
// as Untitled.
func TestASheetClosedEmptyIsDiscarded(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	before := len(app.sessions)
	hostHandlers[keymap.NewPage](app)
	tester.Frame()
	id, ok := app.Workspace.Active()
	if !ok || len(app.sessions) != before+1 {
		t.Fatal("New Sheet should make and open a sheet")
	}
	app.closeColumn(sheetColumn(t, app, sheetName(id)))
	tester.Frame()
	if _, kept := app.sessions[id]; kept || len(app.sessions) != before {
		t.Fatal("An empty new sheet closed should be discarded")
	}
}
