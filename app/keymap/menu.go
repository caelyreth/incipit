package keymap

import "strings"

// Accelerator spells chord as a MyGo menu accelerator. A chord no menu can
// show, as one with Fn, has none.
func Accelerator(chord Chord, platform Platform) (string, bool) {
	if !chord.Valid() || chord.Modifiers.Has(ModifierFunction) {
		return "", false
	}
	platform = platform.resolved()
	parts := make([]string, 0, 5)
	modifiers := chord.Modifiers
	if modifiers.Has(ModifierPrimary) {
		parts = append(parts, "CmdOrCtrl")
	}
	if modifiers.Has(ModifierControl) {
		parts = append(parts, "Ctrl")
	}
	if modifiers.Has(ModifierSuper) {
		parts = append(parts, "Super")
	}
	if modifiers.Has(ModifierAlt) {
		parts = append(parts, "Alt")
	}
	if modifiers.Has(ModifierShift) {
		parts = append(parts, "Shift")
	}
	key := chord.Key
	if key == "+" {
		key = "Plus"
	}
	return strings.Join(append(parts, key), "+"), true
}

// ParseAccelerator reads a MyGo menu accelerator as the chord it takes.
func ParseAccelerator(accelerator string, platform Platform) (Chord, bool) {
	platform = platform.resolved()
	parts := strings.Split(accelerator, "+")
	if strings.HasSuffix(accelerator, "++") {
		parts = append(parts[:len(parts)-2], "+")
	}
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return Chord{}, false
	}
	var modifiers Modifiers
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "cmdorctrl", "commandorcontrol":
			modifiers = modifiers.With(ModifierPrimary)
		case "cmd", "command", "super", "meta":
			if platform == PlatformDarwin {
				modifiers = modifiers.With(ModifierPrimary)
			} else {
				modifiers = modifiers.With(ModifierSuper)
			}
		case "ctrl", "control":
			if platform == PlatformDarwin {
				modifiers = modifiers.With(ModifierControl)
			} else {
				modifiers = modifiers.With(ModifierPrimary)
			}
		case "alt", "option":
			modifiers = modifiers.With(ModifierAlt)
		case "shift":
			modifiers = modifiers.With(ModifierShift)
		default:
			return Chord{}, false
		}
	}
	key := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if key == "plus" {
		key = "+"
	}
	return NewChord(key, modifiers), true
}
