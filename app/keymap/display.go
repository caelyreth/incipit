package keymap

import (
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
)

// Keycaps returns the keys a person presses for target, one cap each, in the
// platform's conventional order and notation: symbols on macOS (⌃⌥⇧⌘),
// words elsewhere (Ctrl, Alt, Shift, Super).
func Keycaps(target kernel.BindingTarget, platform Platform) []string {
	if platform == PlatformCurrent {
		platform = CurrentPlatform()
	}
	modifiers := target.Modifiers()
	caps := make([]string, 0, 5)
	order := []struct {
		modifier kernel.Modifier
		mac      string
		other    string
	}{
		{kernel.ModifierFunction, "fn", "Fn"},
		{kernel.ModifierControl, "⌃", "Ctrl"},
		{kernel.ModifierAlt, "⌥", "Alt"},
		{kernel.ModifierShift, "⇧", "Shift"},
		{kernel.ModifierPrimary, "⌘", "Ctrl"},
		{kernel.ModifierSuper, "⌘", "Super"},
	}
	for _, entry := range order {
		if !modifiers.Has(entry.modifier) {
			continue
		}
		label := entry.other
		if platform == PlatformDarwin {
			label = entry.mac
		}
		if len(caps) > 0 && caps[len(caps)-1] == label {
			continue
		}
		caps = append(caps, label)
	}
	return append(caps, keyLabel(target.Key(), platform))
}

// Chord is target as one compact string: "⌘⇧Z" on macOS, "Ctrl+Shift+Z"
// elsewhere.
func Chord(target kernel.BindingTarget, platform Platform) string {
	if platform == PlatformCurrent {
		platform = CurrentPlatform()
	}
	caps := Keycaps(target, platform)
	if platform == PlatformDarwin {
		return strings.Join(caps, "")
	}
	return strings.Join(caps, "+")
}

// ChordWords spells target in words for searching: "cmd shift z" on macOS,
// "ctrl shift z" elsewhere, so a query typed as words finds symbol chords.
func ChordWords(target kernel.BindingTarget, platform Platform) string {
	if platform == PlatformCurrent {
		platform = CurrentPlatform()
	}
	words := make([]string, 0, 5)
	for _, modifier := range target.Modifiers().All() {
		switch {
		case modifier == kernel.ModifierPrimary && platform == PlatformDarwin:
			words = append(words, "cmd", "command")
		case modifier == kernel.ModifierPrimary:
			words = append(words, "ctrl", "control")
		case modifier == kernel.ModifierAlt && platform == PlatformDarwin:
			words = append(words, "opt", "option", "alt")
		default:
			words = append(words, modifier.String())
		}
	}
	return strings.Join(append(words, target.Key()), " ")
}

func keyLabel(key string, platform Platform) string {
	mac := platform == PlatformDarwin
	pick := func(symbol, word string) string {
		if mac {
			return symbol
		}
		return word
	}
	switch key {
	case "enter":
		return pick("↩", "Enter")
	case "escape":
		return pick("⎋", "Esc")
	case "backspace":
		return pick("⌫", "Backspace")
	case "delete":
		return pick("⌦", "Delete")
	case "tab":
		return pick("⇥", "Tab")
	case "space":
		return "Space"
	case "left":
		return "←"
	case "right":
		return "→"
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "home":
		return pick("↖", "Home")
	case "end":
		return pick("↘", "End")
	case "pageup":
		return pick("⇞", "PgUp")
	case "pagedown":
		return pick("⇟", "PgDn")
	case "insert":
		return "Ins"
	case "contextmenu":
		return "Menu"
	case "back":
		return "Back"
	case "forward":
		return "Forward"
	}
	if len(key) == 1 || (len(key) <= 3 && key[0] == 'f') {
		return strings.ToUpper(key)
	}
	return key
}

// GuideKeycaps are target's keycaps for a hint inside the text: modifiers
// as Keycaps draws them, but a named key spelled as a word ("Tab", not ⇥),
// since a glyph alone at hint size reads as punctuation in the prose.
func GuideKeycaps(target kernel.BindingTarget, platform Platform) []string {
	if platform == PlatformCurrent {
		platform = CurrentPlatform()
	}
	caps := Keycaps(target, platform)
	word := keyLabel(target.Key(), PlatformOther)
	if word == "Enter" && platform == PlatformDarwin {
		word = "Return"
	}
	caps[len(caps)-1] = word
	return caps
}
