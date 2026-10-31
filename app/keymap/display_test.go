package keymap

import (
	"reflect"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
)

func TestKeycapsUsePlatformNotationAndOrder(t *testing.T) {
	redo := kernel.NewBindingTarget(kernel.EditorBindingContext, "z",
		kernel.ModifiersPrimary|kernel.ModifiersShift)
	if got := Keycaps(redo, PlatformDarwin); !reflect.DeepEqual(got, []string{"⇧", "⌘", "Z"}) {
		t.Fatalf("mac caps = %#v", got)
	}
	if got := Chord(redo, PlatformDarwin); got != "⇧⌘Z" {
		t.Fatalf("mac chord = %q", got)
	}
	if got := Chord(redo, PlatformOther); got != "Shift+Ctrl+Z" {
		t.Fatalf("other chord = %q", got)
	}
	start := kernel.NewBindingTarget(kernel.EditorBindingContext, "up",
		kernel.ModifiersPrimary|kernel.ModifiersAlt)
	if got := Chord(start, PlatformDarwin); got != "⌥⌘↑" {
		t.Fatalf("arrow chord = %q", got)
	}
	plain := kernel.NewBindingTarget(kernel.EditorBindingContext, "backspace", kernel.ModifiersNone)
	if got := Keycaps(plain, PlatformOther); !reflect.DeepEqual(got, []string{"Backspace"}) {
		t.Fatalf("plain caps = %#v", got)
	}
	f5 := kernel.NewBindingTarget(kernel.EditorBindingContext, "f5", kernel.ModifiersNone)
	if got := Chord(f5, PlatformDarwin); got != "F5" {
		t.Fatalf("function key = %q", got)
	}
}

func TestChordWordsFindSymbolChordsByName(t *testing.T) {
	bold := kernel.NewBindingTarget(kernel.EditorBindingContext, "b", kernel.ModifiersPrimary)
	if got := ChordWords(bold, PlatformDarwin); got != "cmd command b" {
		t.Fatalf("mac words = %q", got)
	}
	if got := ChordWords(bold, PlatformOther); got != "ctrl control b" {
		t.Fatalf("other words = %q", got)
	}
}
