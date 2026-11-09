package keymap

import (
	"errors"
	"slices"
	"testing"
)

func TestDefaultsBindEveryCommandOnce(t *testing.T) {
	t.Parallel()
	keymap := Default()
	for _, command := range AllCommands() {
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
	if got := Default().CommandsFor(tab); !slices.Equal(got, []CommandID{Leave, Nest}) {
		t.Fatalf("tab runs %v", got)
	}
	if _, err := Default().Remap(Zen, tab, ""); err == nil {
		t.Fatal("a window command shared an editing key")
	}
}

func TestRemapUnbindAndReset(t *testing.T) {
	t.Parallel()
	chord := NewChord("y", ModifiersPrimary)
	remapped, err := Default().Remap(Zen, chord, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := remapped.CommandFor(chord); got != Zen || !remapped.Overridden(Zen) {
		t.Fatalf("remap: %s runs %q", chord, got)
	}
	zen, _ := CommandFor(Zen)
	if _, ok := remapped.CommandFor(zen.Default); ok {
		t.Fatal("the old default still runs Zen")
	}
	unbound, err := remapped.Unbind(Zen)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := unbound.ChordFor(Zen); ok || !unbound.Overridden(Zen) {
		t.Fatal("an unbound command keeps a key")
	}
	reset, err := unbound.Reset(Zen)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := reset.ChordFor(Zen); got != zen.Default || reset.Overridden(Zen) {
		t.Fatalf("reset gave %s", got)
	}
	if !remapped.Overrides().Equal(Overrides{Zen: &chord}) {
		t.Fatal("a later change altered an earlier keymap")
	}
}

func TestChoosingTheDefaultAgainIsNoOverride(t *testing.T) {
	t.Parallel()
	zen, _ := CommandFor(Zen)
	keymap, err := Default().Remap(Zen, zen.Default, "")
	if err != nil {
		t.Fatal(err)
	}
	if keymap.Overridden(Zen) {
		t.Fatal("the default chord was recorded as an override")
	}
}

func TestTakenChordConflictsUnlessDisplaced(t *testing.T) {
	t.Parallel()
	settings, _ := CommandFor(Settings)
	var conflict *ConflictError
	if _, err := Default().Remap(Zen, settings.Default, ""); !errors.As(err, &conflict) {
		t.Fatalf("taken chord: %v", err)
	}
	moved, err := Default().Remap(Zen, settings.Default, Settings)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := moved.CommandFor(settings.Default); got != Zen {
		t.Fatalf("%s runs %s", settings.Default, got)
	}
	if _, ok := moved.ChordFor(Settings); ok {
		t.Fatal("the displaced command kept its key")
	}
}

func TestUnknownCommandsAreRejected(t *testing.T) {
	t.Parallel()
	chord := NewChord("y", ModifiersPrimary)
	if _, err := New(Overrides{"nothing": &chord}); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("unknown override: %v", err)
	}
}
