package keymap

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNormalizeModifiersUsesPlatformPrimary(t *testing.T) {
	t.Parallel()
	native := ui.Ctrl | ui.Alt | ui.Shift | ui.Super
	if got, want := NormalizeModifiers(native, PlatformDarwin),
		ModifiersControl|ModifiersAlt|ModifiersShift|ModifiersPrimary; got != want {
		t.Fatalf("darwin modifiers = %v, want %v", got, want)
	}
	if got, want := NormalizeModifiers(native, PlatformOther),
		ModifiersPrimary|ModifiersAlt|ModifiersShift|ModifiersSuper; got != want {
		t.Fatalf("other modifiers = %v, want %v", got, want)
	}
	if got := NormalizeModifiers(ui.Ctrl, PlatformDarwin); got != ModifiersControl {
		t.Fatalf("darwin control = %v", got)
	}
	if got := NormalizeModifiers(ui.Super, PlatformDarwin); got != ModifiersPrimary {
		t.Fatalf("darwin command = %v", got)
	}
}

// Every chord a default binds can be pressed: its native spelling reads
// back as the same chord on both platforms.
func TestDefaultChordsRoundTripThroughNativeKeys(t *testing.T) {
	t.Parallel()
	for _, platform := range []Platform{PlatformDarwin, PlatformOther} {
		for _, command := range AllCommands() {
			modifiers, key, ok := Native(command.Default, platform)
			if !ok {
				t.Fatalf("%s: %s has no native spelling", command.ID, command.Default)
			}
			name, _ := KeyName(key)
			back := NewChord(name, NormalizeModifiers(modifiers, platform))
			if back != command.Default {
				t.Fatalf("%s: %s → %s", command.ID, command.Default, back)
			}
		}
	}
	if _, _, ok := Native(NewChord("j", ModifiersFunction), PlatformDarwin); ok {
		t.Fatal("an Fn chord has no native spelling")
	}
}

func TestKeyNamesAreUniqueAndInvertible(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, entry := range keyNames {
		if seen[entry.name] {
			t.Fatalf("%q names two keys", entry.name)
		}
		seen[entry.name] = true
		if key, ok := NativeKey(entry.name); !ok || key != entry.key {
			t.Fatalf("%q → %v", entry.name, key)
		}
	}
	if _, ok := KeyName(ui.KeyUnknown); ok {
		t.Fatal("KeyUnknown has a name")
	}
}

func TestResolveEventPrefersLogicalAndFallsBackToPhysical(t *testing.T) {
	t.Parallel()
	a, s := NewChord("a", ModifiersPrimary), NewChord("s", ModifiersPrimary)
	keymap, err := New(Overrides{NewPage: &a, ClosePage: &s})
	if err != nil {
		t.Fatal(err)
	}
	logical := ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyA, PhysicalKey: ui.KeyS, Mods: ui.Super}
	if got, ok := ResolveEvent(keymap, logical, PlatformDarwin); !ok || got != NewPage {
		t.Fatalf("logical lookup = %q,%v", got, ok)
	}
	physical := ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUnknown, PhysicalKey: ui.KeyS, Mods: ui.Super}
	if got, ok := ResolveEvent(keymap, physical, PlatformDarwin); !ok || got != ClosePage {
		t.Fatalf("physical lookup = %q,%v", got, ok)
	}
	unmodified := physical
	unmodified.Mods = 0
	if _, ok := ResolveEvent(keymap, unmodified, PlatformDarwin); ok {
		t.Fatal("positional shortcut resolved without a shortcut modifier")
	}
	otherLogical := physical
	otherLogical.Key = ui.KeyB
	if _, ok := ResolveEvent(keymap, otherLogical, PlatformDarwin); ok {
		t.Fatal("physical key replaced a logical key that is not configured")
	}
}

func TestNonKeyEventsProduceNoChords(t *testing.T) {
	t.Parallel()
	for _, kind := range []ui.InputKind{
		ui.InputKeyUp, ui.InputText, ui.InputCompose, ui.InputCommand, ui.InputPointerDown,
	} {
		event := ui.InputEvent{Kind: kind, Key: ui.KeyRight}
		if chords := EventChords(event, PlatformDarwin); len(chords) != 0 {
			t.Fatalf("%v produced chords: %#v", kind, chords)
		}
	}
}
