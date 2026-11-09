package keymap

import "strings"

// Keycaps returns the keys a person presses for chord, one cap each, in the
// platform's conventional order and notation: symbols on macOS (⌃⌥⇧⌘),
// words elsewhere (Ctrl, Alt, Shift, Super).
func Keycaps(chord Chord, platform Platform) []string {
	platform = platform.resolved()
	modifiers := chord.Modifiers
	caps := make([]string, 0, 5)
	order := []struct {
		modifier Modifier
		mac      string
		other    string
	}{
		{ModifierFunction, "fn", "Fn"},
		{ModifierControl, "⌃", "Ctrl"},
		{ModifierAlt, "⌥", "Alt"},
		{ModifierShift, "⇧", "Shift"},
		{ModifierPrimary, "⌘", "Ctrl"},
		{ModifierSuper, "⌘", "Super"},
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
	return append(caps, keyLabel(chord.Key, platform))
}

// Spell is chord as one compact string: "⌘⇧Z" on macOS, "Ctrl+Shift+Z"
// elsewhere.
func Spell(chord Chord, platform Platform) string {
	platform = platform.resolved()
	caps := Keycaps(chord, platform)
	if platform == PlatformDarwin {
		return strings.Join(caps, "")
	}
	return strings.Join(caps, "+")
}

// Words spells chord in words for searching: "cmd shift z" on macOS,
// "ctrl shift z" elsewhere, so a query typed as words finds symbol chords.
func Words(chord Chord, platform Platform) string {
	platform = platform.resolved()
	words := make([]string, 0, 5)
	for _, modifier := range chord.Modifiers.All() {
		switch {
		case modifier == ModifierPrimary && platform == PlatformDarwin:
			words = append(words, "cmd", "command")
		case modifier == ModifierPrimary:
			words = append(words, "ctrl", "control")
		case modifier == ModifierAlt && platform == PlatformDarwin:
			words = append(words, "opt", "option", "alt")
		default:
			words = append(words, modifier.String())
		}
	}
	return strings.Join(append(words, chord.Key), " ")
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
