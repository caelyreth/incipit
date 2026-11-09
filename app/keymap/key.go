package keymap

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// Platform selects the native meaning of the primary modifier.
// PlatformCurrent follows the operating system running this process.
type Platform uint8

const (
	PlatformCurrent Platform = iota
	PlatformDarwin
	PlatformOther
)

// CurrentPlatform is the platform this process runs on.
func CurrentPlatform() Platform {
	if runtime.GOOS == "darwin" {
		return PlatformDarwin
	}
	return PlatformOther
}

func (p Platform) resolved() Platform {
	if p == PlatformCurrent {
		return CurrentPlatform()
	}
	return p
}

// NormalizeModifiers converts MyGo's modifier bits to canonical modifiers.
// On macOS Command is primary and Control stays distinct; elsewhere Control
// is primary and Super stays distinct. MyGo reports no Function modifier.
func NormalizeModifiers(modifiers ui.Modifiers, platform Platform) Modifiers {
	var normalized Modifiers
	if modifiers&ui.Shift != 0 {
		normalized = normalized.With(ModifierShift)
	}
	if modifiers&ui.Alt != 0 {
		normalized = normalized.With(ModifierAlt)
	}
	if platform.resolved() == PlatformDarwin {
		if modifiers&ui.Ctrl != 0 {
			normalized = normalized.With(ModifierControl)
		}
		if modifiers&ui.Super != 0 {
			normalized = normalized.With(ModifierPrimary)
		}
		return normalized
	}
	if modifiers&ui.Ctrl != 0 {
		normalized = normalized.With(ModifierPrimary)
	}
	if modifiers&ui.Super != 0 {
		normalized = normalized.With(ModifierSuper)
	}
	return normalized
}

// NativeModifiers is the inverse of NormalizeModifiers. A chord with Fn, or
// with both Control and Super where the platform folds one into primary,
// has no MyGo spelling.
func NativeModifiers(modifiers Modifiers, platform Platform) (ui.Modifiers, bool) {
	if modifiers.Has(ModifierFunction) {
		return 0, false
	}
	var native ui.Modifiers
	if modifiers.Has(ModifierShift) {
		native |= ui.Shift
	}
	if modifiers.Has(ModifierAlt) {
		native |= ui.Alt
	}
	if platform.resolved() == PlatformDarwin {
		if modifiers.Has(ModifierSuper) {
			return 0, false
		}
		if modifiers.Has(ModifierControl) {
			native |= ui.Ctrl
		}
		if modifiers.Has(ModifierPrimary) {
			native |= ui.Super
		}
		return native, true
	}
	if modifiers.Has(ModifierControl) {
		return 0, false
	}
	if modifiers.Has(ModifierPrimary) {
		native |= ui.Ctrl
	}
	if modifiers.Has(ModifierSuper) {
		native |= ui.Super
	}
	return native, true
}

// keyNames pairs each MyGo key with its stable name. Punctuation uses its
// literal US-layout character, as in primary-comma.
var keyNames = [...]struct {
	key  ui.Key
	name string
}{
	{ui.KeyEnter, "enter"},
	{ui.KeyEscape, "escape"},
	{ui.KeyBackspace, "backspace"},
	{ui.KeyTab, "tab"},
	{ui.KeySpace, "space"},
	{ui.KeyDelete, "delete"},
	{ui.KeyInsert, "insert"},
	{ui.KeyHome, "home"},
	{ui.KeyEnd, "end"},
	{ui.KeyPageUp, "pageup"},
	{ui.KeyPageDown, "pagedown"},
	{ui.KeyLeft, "left"},
	{ui.KeyRight, "right"},
	{ui.KeyUp, "up"},
	{ui.KeyDown, "down"},
	{ui.KeyF1, "f1"},
	{ui.KeyF2, "f2"},
	{ui.KeyF3, "f3"},
	{ui.KeyF4, "f4"},
	{ui.KeyF5, "f5"},
	{ui.KeyF6, "f6"},
	{ui.KeyF7, "f7"},
	{ui.KeyF8, "f8"},
	{ui.KeyF9, "f9"},
	{ui.KeyF10, "f10"},
	{ui.KeyF11, "f11"},
	{ui.KeyF12, "f12"},
	{ui.KeyMinus, "-"},
	{ui.KeyEqual, "="},
	{ui.KeyComma, ","},
	{ui.KeyPeriod, "."},
	{ui.KeySlash, "/"},
	{ui.KeySemicolon, ";"},
	{ui.KeyQuote, "'"},
	{ui.KeyBracketLeft, "["},
	{ui.KeyBracketRight, "]"},
	{ui.KeyBackslash, "\\"},
	{ui.KeyBackquote, "`"},
	{ui.KeyContextMenu, "contextmenu"},
	{ui.Key0, "0"},
	{ui.Key1, "1"},
	{ui.Key2, "2"},
	{ui.Key3, "3"},
	{ui.Key4, "4"},
	{ui.Key5, "5"},
	{ui.Key6, "6"},
	{ui.Key7, "7"},
	{ui.Key8, "8"},
	{ui.Key9, "9"},
	{ui.KeyA, "a"},
	{ui.KeyB, "b"},
	{ui.KeyC, "c"},
	{ui.KeyD, "d"},
	{ui.KeyE, "e"},
	{ui.KeyF, "f"},
	{ui.KeyG, "g"},
	{ui.KeyH, "h"},
	{ui.KeyI, "i"},
	{ui.KeyJ, "j"},
	{ui.KeyK, "k"},
	{ui.KeyL, "l"},
	{ui.KeyM, "m"},
	{ui.KeyN, "n"},
	{ui.KeyO, "o"},
	{ui.KeyP, "p"},
	{ui.KeyQ, "q"},
	{ui.KeyR, "r"},
	{ui.KeyS, "s"},
	{ui.KeyT, "t"},
	{ui.KeyU, "u"},
	{ui.KeyV, "v"},
	{ui.KeyW, "w"},
	{ui.KeyX, "x"},
	{ui.KeyY, "y"},
	{ui.KeyZ, "z"},
	{ui.KeyBack, "back"},
	{ui.KeyForward, "forward"},
}

var keysByName = func() map[string]ui.Key {
	byName := make(map[string]ui.Key, len(keyNames))
	for _, entry := range keyNames {
		byName[entry.name] = entry.key
	}
	return byName
}()

// KeyName returns a MyGo key's stable name. KeyUnknown has none.
func KeyName(key ui.Key) (string, bool) {
	for _, entry := range keyNames {
		if entry.key == key {
			return entry.name, true
		}
	}
	return "", false
}

// NativeKey returns the MyGo key a stable name names.
func NativeKey(name string) (ui.Key, bool) {
	key, ok := keysByName[name]
	return key, ok
}

// Native spells chord as the MyGo modifiers and key that press it.
func Native(chord Chord, platform Platform) (ui.Modifiers, ui.Key, bool) {
	key, ok := NativeKey(chord.Key)
	if !ok {
		return 0, 0, false
	}
	modifiers, ok := NativeModifiers(chord.Modifiers, platform)
	return modifiers, key, ok
}

// EventChords returns the chords one key-down event may mean, in lookup
// order. The layout's logical key is authoritative; the ANSI physical key is
// considered only when the layout reports none and a shortcut modifier is
// held, so typing on non-US layouts is not disturbed.
func EventChords(event ui.InputEvent, platform Platform) []Chord {
	if event.Kind != ui.InputKeyDown {
		return nil
	}
	modifiers := NormalizeModifiers(event.Mods, platform)
	chords := make([]Chord, 0, 2)
	if key, ok := KeyName(event.Key); ok {
		chords = append(chords, NewChord(key, modifiers))
	}
	if event.Key == ui.KeyUnknown && event.PhysicalKey != ui.KeyUnknown &&
		event.Mods&(ui.Ctrl|ui.Alt|ui.Super) != 0 {
		if key, ok := KeyName(event.PhysicalKey); ok {
			chord := NewChord(key, modifiers)
			if len(chords) == 0 || chords[0] != chord {
				chords = append(chords, chord)
			}
		}
	}
	return chords
}

// ResolveEvent returns the command a key-down event runs.
func ResolveEvent(keymap *Keymap, event ui.InputEvent, platform Platform) (CommandID, bool) {
	for _, chord := range EventChords(event, platform) {
		if command, ok := keymap.CommandFor(chord); ok {
			return command, true
		}
	}
	return "", false
}
