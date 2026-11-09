package keymap

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// ErrUnknownCommand rejects a remap of a command that is not declared.
var ErrUnknownCommand = errors.New("keymap: unknown command")

// ConflictError reports two commands given the same chord.
type ConflictError struct {
	Chord    Chord
	Existing CommandID
	Incoming CommandID
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("keymap: %s is given to both %s and %s", e.Chord, e.Existing, e.Incoming)
}

// Overrides are the person's changes to the default keys: a command maps to
// the one chord it takes instead, or to no chord when it is unbound. A
// command absent from the map keeps its default.
type Overrides map[CommandID]*Chord

// Clone returns overrides that share nothing with o.
func (o Overrides) Clone() Overrides {
	cloned := make(Overrides, len(o))
	for command, chord := range o {
		if chord != nil {
			chord := *chord
			cloned[command] = &chord
			continue
		}
		cloned[command] = nil
	}
	return cloned
}

// Equal reports whether two overrides say the same thing.
func (o Overrides) Equal(other Overrides) bool {
	return maps.EqualFunc(o, other, func(a, b *Chord) bool {
		return (a == nil) == (b == nil) && (a == nil || *a == *b)
	})
}

// Keymap is the defaults with overrides applied, checked to give no chord
// to two commands. It is immutable.
type Keymap struct {
	overrides Overrides
	byChord   map[Chord][]CommandID
	byCommand map[CommandID]Chord
}

// Default is the keymap with no overrides.
func Default() *Keymap {
	keymap, err := New(nil)
	if err != nil {
		panic(err)
	}
	return keymap
}

// New prepares the defaults with overrides applied.
func New(overrides Overrides) (*Keymap, error) {
	k := &Keymap{
		overrides: overrides.Clone(),
		byChord:   make(map[Chord][]CommandID, len(commands)),
		byCommand: make(map[CommandID]Chord, len(commands)),
	}
	for command := range overrides {
		if _, ok := CommandFor(command); !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownCommand, command)
		}
	}
	for _, command := range commands {
		chord := command.Default
		if override, ok := overrides[command.ID]; ok {
			chord = Chord{}
			if override != nil {
				chord = *override
			}
		}
		if !chord.Valid() {
			continue
		}
		// Editing commands may share a key, tried in declaration order:
		// the page runs the first the engine finds available, as Tab
		// leaves a scope or, with none to leave, nests.
		for _, existing := range k.byChord[chord] {
			if held, _ := CommandFor(existing); held.Scope != ScopeEditing || command.Scope != ScopeEditing {
				return nil, &ConflictError{Chord: chord, Existing: existing, Incoming: command.ID}
			}
		}
		k.byChord[chord] = append(k.byChord[chord], command.ID)
		k.byCommand[command.ID] = chord
	}
	return k, nil
}

// Overrides returns the overrides the keymap was prepared with.
func (k *Keymap) Overrides() Overrides {
	if k == nil {
		return Overrides{}
	}
	return k.overrides.Clone()
}

// Overridden reports whether command's key is the person's choice.
func (k *Keymap) Overridden(command CommandID) bool {
	if k == nil {
		return false
	}
	_, ok := k.overrides[command]
	return ok
}

// CommandFor returns the command chord runs.
func (k *Keymap) CommandFor(chord Chord) (CommandID, bool) {
	if k == nil {
		return "", false
	}
	commands := k.byChord[chord]
	if len(commands) == 0 {
		return "", false
	}
	return commands[0], true
}

// CommandsFor returns the commands chord runs, in the order they are
// tried.
func (k *Keymap) CommandsFor(chord Chord) []CommandID {
	if k == nil {
		return nil
	}
	return slices.Clone(k.byChord[chord])
}

// ChordFor returns command's chord, when it has one.
func (k *Keymap) ChordFor(command CommandID) (Chord, bool) {
	if k == nil {
		return Chord{}, false
	}
	chord, ok := k.byCommand[command]
	return chord, ok
}

// Chords lists every bound chord in a stable order.
func (k *Keymap) Chords() []Chord {
	if k == nil {
		return nil
	}
	chords := slices.Collect(maps.Keys(k.byChord))
	slices.SortFunc(chords, func(a, b Chord) int {
		if a.Key != b.Key {
			if a.Key < b.Key {
				return -1
			}
			return 1
		}
		return int(a.Modifiers) - int(b.Modifiers)
	})
	return chords
}

// Remap gives command chord, unbinding displaced (when set) so a chord
// another command holds moves in one change. Choosing the command's own
// default again drops the override.
func (k *Keymap) Remap(command CommandID, chord Chord, displaced CommandID) (*Keymap, error) {
	declared, ok := CommandFor(command)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
	overrides := k.Overrides()
	if displaced != "" && displaced != command {
		overrides[displaced] = nil
	}
	if chord == declared.Default {
		delete(overrides, command)
	} else {
		overrides[command] = &chord
	}
	return New(overrides)
}

// Unbind takes command's key away.
func (k *Keymap) Unbind(command CommandID) (*Keymap, error) {
	overrides := k.Overrides()
	overrides[command] = nil
	return New(overrides)
}

// Reset gives command its default key again.
func (k *Keymap) Reset(command CommandID) (*Keymap, error) {
	overrides := k.Overrides()
	delete(overrides, command)
	return New(overrides)
}
