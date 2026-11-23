// Package keymap names the application's commands and the keys that run
// them: the window, page and app actions the shell answers. It resolves MyGo
// key events and menu accelerators to commands, spells chords for display,
// and keeps the person's remaps above the defaults.
package keymap

import (
	"fmt"
	"strings"
	"unicode"
)

// Modifier is one canonical modifier key. Primary is Command on macOS and
// Control elsewhere; Control and Super are the remaining keys of each.
type Modifier uint8

const (
	ModifierAlt Modifier = iota
	ModifierControl
	ModifierFunction
	ModifierPrimary
	ModifierShift
	ModifierSuper
)

// AllModifiers is every modifier in canonical order.
var AllModifiers = [...]Modifier{
	ModifierAlt,
	ModifierControl,
	ModifierFunction,
	ModifierPrimary,
	ModifierShift,
	ModifierSuper,
}

func (m Modifier) String() string {
	switch m {
	case ModifierAlt:
		return "alt"
	case ModifierControl:
		return "ctrl"
	case ModifierFunction:
		return "fn"
	case ModifierPrimary:
		return "primary"
	case ModifierShift:
		return "shift"
	case ModifierSuper:
		return "super"
	default:
		return fmt.Sprintf("modifier(%d)", m)
	}
}

// Modifiers is a set of modifiers.
type Modifiers uint8

const (
	ModifiersNone     Modifiers = 0
	ModifiersAlt      Modifiers = 1 << ModifierAlt
	ModifiersControl  Modifiers = 1 << ModifierControl
	ModifiersFunction Modifiers = 1 << ModifierFunction
	ModifiersPrimary  Modifiers = 1 << ModifierPrimary
	ModifiersShift    Modifiers = 1 << ModifierShift
	ModifiersSuper    Modifiers = 1 << ModifierSuper
)

func (m Modifiers) Has(modifier Modifier) bool { return m&(1<<modifier) != 0 }

func (m Modifiers) With(modifier Modifier) Modifiers { return m | (1 << modifier) }

// All lists the set's modifiers in canonical order.
func (m Modifiers) All() []Modifier {
	modifiers := make([]Modifier, 0, len(AllModifiers))
	for _, modifier := range AllModifiers {
		if m.Has(modifier) {
			modifiers = append(modifiers, modifier)
		}
	}
	return modifiers
}

func (m Modifiers) String() string {
	names := make([]string, 0, len(AllModifiers))
	for _, modifier := range m.All() {
		names = append(names, modifier.String())
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, "+")
}

// Chord is one key pressed with a set of modifiers. Key is the key's stable
// name (KeyName): a lowercase letter or digit, a US-layout punctuation
// character, or a word such as "enter" or "left".
type Chord struct {
	Key       string
	Modifiers Modifiers
}

// NewChord is key pressed with modifiers.
func NewChord(key string, modifiers Modifiers) Chord {
	return Chord{Key: key, Modifiers: modifiers}
}

// Valid reports whether the chord names a key.
func (c Chord) Valid() bool {
	if strings.TrimSpace(c.Key) == "" {
		return false
	}
	for _, r := range c.Key {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (c Chord) String() string {
	if c.Modifiers == ModifiersNone {
		return c.Key
	}
	return c.Modifiers.String() + "+" + c.Key
}
