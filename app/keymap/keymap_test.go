package keymap

import (
	"errors"
	"slices"
	"testing"
)

func TestDefaultsBindEveryCommandOnce(t *testing.T) {
	t.Parallel()
	keymap := Default(declared())
	for _, command := range defaulted() {
		chord, ok := keymap.ChordFor(command.ID)
		if !ok || chord != command.Default {
			t.Fatalf("%s = %s,%v, want %s", command.ID, chord, ok, command.Default)
		}
		if !slices.Contains(keymap.CommandsFor(chord), command.ID) {
			t.Fatalf("%s runs %v, not %s", chord, keymap.CommandsFor(chord), command.ID)
		}
	}
}

// Editing commands may share a key, tried in order; no other command may.
func TestOnlyEditingCommandsShareAKey(t *testing.T) {
	t.Parallel()
	tab := NewChord("tab", ModifiersNone)
	if got := Default(declared()).CommandsFor(tab); !slices.Equal(got, []CommandID{"core.leave", "lists.nest"}) {
		t.Fatalf("tab runs %v", got)
	}
	if _, err := Default(declared()).Remap(Typewriter, tab, ""); err == nil {
		t.Fatal("a window command shared an editing key")
	}
}

func TestRemapUnbindAndReset(t *testing.T) {
	t.Parallel()
	chord := NewChord("y", ModifiersPrimary)
	remapped, err := Default(declared()).Remap(Typewriter, chord, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := remapped.CommandFor(chord); got != Typewriter || !remapped.Overridden(Typewriter) {
		t.Fatalf("remap: %s runs %q", chord, got)
	}
	typewriter, _ := Default(declared()).Command(Typewriter)
	if _, ok := remapped.CommandFor(typewriter.Default); ok {
		t.Fatal("the old default still runs Typewriter")
	}
	unbound, err := remapped.Unbind(Typewriter)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := unbound.ChordFor(Typewriter); ok || !unbound.Overridden(Typewriter) {
		t.Fatal("an unbound command keeps a key")
	}
	reset, err := unbound.Reset(Typewriter)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := reset.ChordFor(Typewriter); got != typewriter.Default || reset.Overridden(Typewriter) {
		t.Fatalf("reset gave %s", got)
	}
	if !remapped.Overrides().Equal(Overrides{Typewriter: &chord}) {
		t.Fatal("a later change altered an earlier keymap")
	}
}

func TestChoosingTheDefaultAgainIsNoOverride(t *testing.T) {
	t.Parallel()
	typewriter, _ := Default(declared()).Command(Typewriter)
	keymap, err := Default(declared()).Remap(Typewriter, typewriter.Default, "")
	if err != nil {
		t.Fatal(err)
	}
	if keymap.Overridden(Typewriter) {
		t.Fatal("the default chord was recorded as an override")
	}
}

func TestTakenChordConflictsUnlessDisplaced(t *testing.T) {
	t.Parallel()
	settings, _ := Default(declared()).Command(Settings)
	var conflict *ConflictError
	if _, err := Default(declared()).Remap(Typewriter, settings.Default, ""); !errors.As(err, &conflict) {
		t.Fatalf("taken chord: %v", err)
	}
	moved, err := Default(declared()).Remap(Typewriter, settings.Default, Settings)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := moved.CommandFor(settings.Default); got != Typewriter {
		t.Fatalf("%s runs %s", settings.Default, got)
	}
	if _, ok := moved.ChordFor(Settings); ok {
		t.Fatal("the displaced command kept its key")
	}
}

func TestUnknownCommandsAreRejected(t *testing.T) {
	t.Parallel()
	chord := NewChord("y", ModifiersPrimary)
	if _, err := New(declared(), Overrides{"nothing": &chord}); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("unknown override: %v", err)
	}
}
