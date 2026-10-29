package window

import (
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/overlay"
)

func paletteCommand(t *testing.T, commands []overlay.Command, id kernel.CommandID) overlay.Command {
	t.Helper()
	for _, command := range commands {
		if command.ID == id {
			return command
		}
	}
	t.Fatalf("command %q missing from %#v", id, commands)
	return overlay.Command{}
}

func TestPaletteUsesLiveConfiguredBindings(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	target := kernel.NewBindingTarget(
		kernel.EditorBindingContext,
		"g",
		kernel.ModifiersPrimary|kernel.ModifiersAlt|kernel.ModifiersShift,
	)
	if _, err := app.Product.Apply(
		appsettings.Remap(editor.SurfaceSettingsCommandID, target),
		app.Store,
		app.Session,
	); err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()

	command := paletteCommand(t, app.commandCatalog(), editor.SurfaceSettingsCommandID)
	if want := keymap.Chord(target, keymap.PlatformCurrent); command.Binding != want {
		t.Fatalf("binding = %q, want %q", command.Binding, want)
	}
}

func TestPaletteOmitsDisabledCommandsAndRequiresAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	if _, err := app.Product.Apply(
		appsettings.ToggleBoolean(
			appsettings.PolicyBoolean(editor.MarkupPolicyID),
			false,
		),
		app.Store,
		app.Session,
	); err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	for _, command := range app.commandCatalog() {
		if command.ID == editor.MarkupStrong.ID() {
			t.Fatal("disabled markup command remained in palette")
		}
	}

	app.closePage()
	commands := app.commandCatalog()
	paletteCommand(t, commands, editor.SurfaceNewPageCommandID)
	paletteCommand(t, commands, editor.SurfaceSettingsCommandID)
	if commands := app.commandCatalog(); len(commands) == 0 {
		t.Fatal("page-less palette lost application commands")
	}
	for _, command := range commands {
		if command.ID == editor.TextBackspace.ID() {
			t.Fatal("document command remained without an open page")
		}
	}
}
