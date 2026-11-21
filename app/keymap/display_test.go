package keymap

import (
	"reflect"
	"testing"
)

func TestKeycapsUsePlatformNotationAndOrder(t *testing.T) {
	t.Parallel()
	chord := NewChord("z", ModifiersPrimary|ModifiersShift)
	if got := Keycaps(chord, PlatformDarwin); !reflect.DeepEqual(got, []string{"⇧", "⌘", "Z"}) {
		t.Fatalf("mac caps = %#v", got)
	}
	if got := Spell(chord, PlatformDarwin); got != "⇧⌘Z" {
		t.Fatalf("mac chord = %q", got)
	}
	if got := Spell(chord, PlatformOther); got != "Shift+Ctrl+Z" {
		t.Fatalf("other chord = %q", got)
	}
	if got := Spell(NewChord("up", ModifiersPrimary|ModifiersAlt), PlatformDarwin); got != "⌥⌘↑" {
		t.Fatalf("arrow chord = %q", got)
	}
	if got := Keycaps(NewChord("backspace", ModifiersNone), PlatformOther); !reflect.DeepEqual(got, []string{"Backspace"}) {
		t.Fatalf("plain caps = %#v", got)
	}
	if got := Spell(NewChord("f5", ModifiersNone), PlatformDarwin); got != "F5" {
		t.Fatalf("function key = %q", got)
	}
}

func TestWordsFindSymbolChordsByName(t *testing.T) {
	t.Parallel()
	library := NewChord("b", ModifiersPrimary)
	if got := Words(library, PlatformDarwin); got != "cmd command b" {
		t.Fatalf("mac words = %q", got)
	}
	if got := Words(library, PlatformOther); got != "ctrl control b" {
		t.Fatalf("other words = %q", got)
	}
}
