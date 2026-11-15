package window

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/overlay"
)

func paletteCommand(t *testing.T, commands []overlay.Command, id keymap.CommandID) overlay.Command {
	t.Helper()
	for _, command := range commands {
		if command.ID == id {
			return command
		}
	}
	t.Fatalf("command %q missing from %#v", id, commands)
	return overlay.Command{}
}

func TestPaletteUsesLiveBindings(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	moved := keymap.NewChord("g", keymap.ModifiersPrimary|keymap.ModifiersAlt|keymap.ModifiersShift)
	app.Settings.Apply(appsettings.Remap(keymap.Settings, moved))

	command := paletteCommand(t, app.commandCatalog(), keymap.Settings)
	if want := keymap.Spell(moved, keymap.PlatformCurrent); command.Binding != want {
		t.Fatalf("binding = %q, want %q", command.Binding, want)
	}
	if want := keymap.Keycaps(moved, keymap.PlatformCurrent); !slices.Equal(command.Keys, want) {
		t.Fatalf("keys = %q, want %q", command.Keys, want)
	}

	app.Settings.Apply(appsettings.Unbind(keymap.Settings))
	command = paletteCommand(t, app.commandCatalog(), keymap.Settings)
	if command.Binding != "" || len(command.Keys) != 0 {
		t.Fatalf("unbound command still shows %q %q", command.Binding, command.Keys)
	}
}

func TestPaletteKeepsItsCommandsWithoutAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	before := len(app.commandCatalog())
	app.closeActive()
	commands := app.commandCatalog()
	paletteCommand(t, commands, keymap.NewPage)
	paletteCommand(t, commands, keymap.Settings)
	if len(commands) != before {
		t.Fatalf("page-less palette has %d commands, want %d", len(commands), before)
	}
}
