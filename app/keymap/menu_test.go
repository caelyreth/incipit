package keymap

import "testing"

// Every default key a menu can show reads back as the same chord, so the
// menu's keys and the keymap's agree on both platforms.
func TestAcceleratorsRoundTripTheDefaultKeymap(t *testing.T) {
	t.Parallel()
	for _, platform := range []Platform{PlatformDarwin, PlatformOther} {
		for _, command := range defaulted() {
			accelerator, ok := Accelerator(command.Default, platform)
			if !ok {
				t.Fatalf("%s has no accelerator", command.Default)
			}
			back, ok := ParseAccelerator(accelerator, platform)
			if !ok || back != command.Default {
				t.Fatalf("%s → %q → %s", command.Default, accelerator, back)
			}
		}
	}
	quit, ok := ParseAccelerator("Cmd+Q", PlatformDarwin)
	if !ok || quit != NewChord("q", ModifiersPrimary) {
		t.Fatalf("Cmd+Q = %s", quit)
	}
	if _, ok := Accelerator(NewChord("j", ModifiersFunction), PlatformDarwin); ok {
		t.Fatal("an Fn chord has no menu accelerator")
	}
}
