package keymap

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// Every default key a menu can show reads back as the same chord, so the
// menu's keys and the keymap's agree on both platforms.
func TestAcceleratorsRoundTripTheDefaultKeymap(t *testing.T) {
	for _, platform := range []Platform{PlatformDarwin, PlatformOther} {
		for _, binding := range editor.DefaultKeymap() {
			accelerator, ok := Accelerator(binding.Target, platform)
			if !ok {
				t.Fatalf("%s has no accelerator", binding.Target)
			}
			back, ok := ParseAccelerator(accelerator, platform)
			if !ok || back != binding.Target {
				t.Fatalf("%s → %q → %s", binding.Target, accelerator, back)
			}
		}
	}
	quit, ok := ParseAccelerator("Cmd+Q", PlatformDarwin)
	if !ok || quit != kernel.NewBindingTarget(kernel.EditorBindingContext, "q", kernel.ModifiersPrimary) {
		t.Fatalf("Cmd+Q = %s", quit)
	}
	if _, ok := Accelerator(kernel.NewBindingTarget(kernel.EditorBindingContext, "j", kernel.ModifiersFunction), PlatformDarwin); ok {
		t.Fatal("an Fn chord has no menu accelerator")
	}
}

func TestMenuCommandsNameTheirActions(t *testing.T) {
	for _, entry := range MenuCommands {
		action, ok := MenuAction(entry.Name)
		if !ok || action.ID != entry.Command {
			t.Fatalf("%s → %#v", entry.Name, action)
		}
	}
	if command, ok := MenuCommandFor("pasteAndMatchStyle"); !ok || command != PasteCommandID {
		t.Fatalf("paste and match style → %s", command)
	}
}
