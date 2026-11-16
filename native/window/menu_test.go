package window

import (
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo"
)

func TestMainMenuExposesAppAndWorkspaceCommands(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	menu := app.MainMenu()
	items := menu.Items()
	if len(items) < 4 {
		t.Fatalf("menu items = %d", len(items))
	}
	if items[0].ID != "" || items[0].Role == "" {
		t.Fatalf("app menu = %#v", items[0])
	}
	settings := menu.ItemByID("settings")
	if settings == nil || settings.Label != "Settings..." || settings.Click == nil {
		t.Fatalf("settings = %#v", settings)
	}
	for _, id := range []string{"new-page", "close-page", "view-field", "commands"} {
		if item := menu.ItemByID(id); item == nil || item.Click == nil {
			t.Fatalf("%s = %#v", id, item)
		}
	}
}

func TestPillListsEveryCommandButItsOwn(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	listed := map[keymap.CommandID]string{}
	for _, command := range app.Shell.Commands() {
		if command.Label == "" || command.Group == "" {
			t.Fatalf("incomplete command = %#v", command)
		}
		listed[keymap.CommandID(command.ID)] = command.Group
	}
	for _, command := range appsettings.DefaultState().Keymap.Commands() {
		group, ok := listed[command.ID]
		if command.ID == keymap.Commands || command.Scope == keymap.ScopeEditing {
			if ok {
				t.Fatalf("the pill lists %s", command.ID)
			}
			continue
		}
		if !ok || group != command.Scope.Title() {
			t.Fatalf("%s listed = %v under %q", command.ID, ok, group)
		}
	}
}

// The menu's keys are the keymap's: a remap moves the item's key, and
// unbinding leaves the item without one.
func TestMenuKeysFollowTheKeymap(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	menu := app.MainMenu()
	app.refreshSettingsData()
	item := menu.ItemByID("new-page")
	if want, _ := keymap.Accelerator(chordFor(t, app, keymap.NewPage), keymap.PlatformCurrent); item.Accelerator != want || want == "" {
		t.Fatalf("new page key %q, want %q", item.Accelerator, want)
	}
	moved := keymap.NewChord("j", keymap.ModifiersPrimary|keymap.ModifiersShift)
	app.Settings.Apply(appsettings.Remap(keymap.NewPage, moved))
	if want, _ := keymap.Accelerator(moved, keymap.PlatformCurrent); item.Accelerator != want {
		t.Fatalf("remapped new page key %q, want %q", item.Accelerator, want)
	}
	if shortcut, ok := app.Settings.Data.Shortcut(keymap.NewPage); !ok || !shortcut.Overridden {
		t.Fatalf("settings did not see the remap: %#v", shortcut)
	}
	app.Settings.Apply(appsettings.Unbind(keymap.NewPage))
	if item.Accelerator != "" {
		t.Fatalf("unbound new page kept %q", item.Accelerator)
	}
	app.Settings.Apply(appsettings.ResetBinding(keymap.NewPage))
	if want, _ := keymap.Accelerator(keymap.NewChord("n", keymap.ModifiersPrimary), keymap.PlatformCurrent); item.Accelerator != want {
		t.Fatalf("reset new page key %q, want %q", item.Accelerator, want)
	}
}

// While a shortcut is recorded every menu item lets go of its key, so the
// recorder hears each chord; the system's keys cannot be given to actions.
func TestMenuStandsDownWhileRecording(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	menu := app.MainMenu()
	app.refreshSettingsData()
	before := map[*mygo.MenuItem]string{}
	eachMenuItem(menu, func(item *mygo.MenuItem) {
		if item.Accelerator != "" {
			before[item] = item.Accelerator
		}
	})
	var quit *mygo.MenuItem
	for item := range before {
		if item.Role == mygo.RoleQuit {
			quit = item
		}
	}
	if len(before) < 5 || quit == nil {
		t.Fatalf("keyed items %d, quit %v", len(before), quit)
	}

	app.Settings.Recording(true)
	eachMenuItem(menu, func(item *mygo.MenuItem) {
		if item.Accelerator != "" {
			t.Fatalf("%q kept %q while recording", item.Label, item.Accelerator)
		}
	})
	quitKey, _ := keymap.ParseAccelerator(before[quit], keymap.PlatformCurrent)
	if label, reserved := app.Settings.Reserved(quitKey); !reserved || label != quit.Label {
		t.Fatalf("quit key reserved = %v %q", reserved, label)
	}
	if _, reserved := app.Settings.Reserved(chordFor(t, app, keymap.NewPage)); reserved {
		t.Fatal("a command's own key reported as reserved")
	}

	app.Settings.Recording(false)
	for item, accelerator := range before {
		if item.Accelerator != accelerator {
			t.Fatalf("%q came back as %q, want %q", item.Label, item.Accelerator, accelerator)
		}
	}
}

func chordFor(t *testing.T, app *AppState, command keymap.CommandID) keymap.Chord {
	t.Helper()
	chord, ok := app.Product.Keymap.ChordFor(command)
	if !ok {
		t.Fatalf("%s has no key", command)
	}
	return chord
}
