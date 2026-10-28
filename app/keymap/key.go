package keymap

import (
	"runtime"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

// Platform selects the native meaning of the primary shortcut modifier.
// PlatformCurrent follows the operating system running this process.
type Platform uint8

const (
	PlatformCurrent Platform = iota
	PlatformDarwin
	PlatformOther
)

// CurrentPlatform returns the platform used by a default Dispatcher.
func CurrentPlatform() Platform {
	if runtime.GOOS == "darwin" {
		return PlatformDarwin
	}
	return PlatformOther
}

// NormalizeModifiers converts MyGo's native modifier bits to the canonical
// modifier namespace used by prepared keymaps. On macOS Command is primary and
// Control remains a distinct modifier; elsewhere Control is primary and Super
// remains distinct. MyGo does not expose a Function modifier.
func NormalizeModifiers(modifiers ui.Modifiers, platform Platform) kernel.Modifiers {
	var normalized kernel.Modifiers
	if modifiers&ui.Shift != 0 {
		normalized = normalized.With(kernel.ModifierShift)
	}
	if modifiers&ui.Alt != 0 {
		normalized = normalized.With(kernel.ModifierAlt)
	}

	if platform == PlatformCurrent {
		platform = CurrentPlatform()
	}
	if platform == PlatformDarwin {
		if modifiers&ui.Ctrl != 0 {
			normalized = normalized.With(kernel.ModifierControl)
		}
		if modifiers&ui.Super != 0 {
			normalized = normalized.With(kernel.ModifierPrimary)
		}
		return normalized
	}

	if modifiers&ui.Ctrl != 0 {
		normalized = normalized.With(kernel.ModifierPrimary)
	}
	if modifiers&ui.Super != 0 {
		normalized = normalized.With(kernel.ModifierSuper)
	}
	return normalized
}

// KeyName returns the stable binding name for a MyGo key. Punctuation uses its
// literal US-layout character because that is how Shoreward declares shortcuts
// such as primary-comma. KeyUnknown has no binding name.
func KeyName(key ui.Key) (string, bool) {
	switch key {
	case ui.KeyEnter:
		return "enter", true
	case ui.KeyEscape:
		return "escape", true
	case ui.KeyBackspace:
		return "backspace", true
	case ui.KeyTab:
		return "tab", true
	case ui.KeySpace:
		return "space", true
	case ui.KeyDelete:
		return "delete", true
	case ui.KeyInsert:
		return "insert", true
	case ui.KeyHome:
		return "home", true
	case ui.KeyEnd:
		return "end", true
	case ui.KeyPageUp:
		return "pageup", true
	case ui.KeyPageDown:
		return "pagedown", true
	case ui.KeyLeft:
		return "left", true
	case ui.KeyRight:
		return "right", true
	case ui.KeyUp:
		return "up", true
	case ui.KeyDown:
		return "down", true
	case ui.KeyF1:
		return "f1", true
	case ui.KeyF2:
		return "f2", true
	case ui.KeyF3:
		return "f3", true
	case ui.KeyF4:
		return "f4", true
	case ui.KeyF5:
		return "f5", true
	case ui.KeyF6:
		return "f6", true
	case ui.KeyF7:
		return "f7", true
	case ui.KeyF8:
		return "f8", true
	case ui.KeyF9:
		return "f9", true
	case ui.KeyF10:
		return "f10", true
	case ui.KeyF11:
		return "f11", true
	case ui.KeyF12:
		return "f12", true
	case ui.KeyMinus:
		return "-", true
	case ui.KeyEqual:
		return "=", true
	case ui.KeyComma:
		return ",", true
	case ui.KeyPeriod:
		return ".", true
	case ui.KeySlash:
		return "/", true
	case ui.KeySemicolon:
		return ";", true
	case ui.KeyQuote:
		return "'", true
	case ui.KeyBracketLeft:
		return "[", true
	case ui.KeyBracketRight:
		return "]", true
	case ui.KeyBackslash:
		return "\\", true
	case ui.KeyBackquote:
		return "`", true
	case ui.KeyContextMenu:
		return "contextmenu", true
	case ui.Key0:
		return "0", true
	case ui.Key1:
		return "1", true
	case ui.Key2:
		return "2", true
	case ui.Key3:
		return "3", true
	case ui.Key4:
		return "4", true
	case ui.Key5:
		return "5", true
	case ui.Key6:
		return "6", true
	case ui.Key7:
		return "7", true
	case ui.Key8:
		return "8", true
	case ui.Key9:
		return "9", true
	case ui.KeyA:
		return "a", true
	case ui.KeyB:
		return "b", true
	case ui.KeyC:
		return "c", true
	case ui.KeyD:
		return "d", true
	case ui.KeyE:
		return "e", true
	case ui.KeyF:
		return "f", true
	case ui.KeyG:
		return "g", true
	case ui.KeyH:
		return "h", true
	case ui.KeyI:
		return "i", true
	case ui.KeyJ:
		return "j", true
	case ui.KeyK:
		return "k", true
	case ui.KeyL:
		return "l", true
	case ui.KeyM:
		return "m", true
	case ui.KeyN:
		return "n", true
	case ui.KeyO:
		return "o", true
	case ui.KeyP:
		return "p", true
	case ui.KeyQ:
		return "q", true
	case ui.KeyR:
		return "r", true
	case ui.KeyS:
		return "s", true
	case ui.KeyT:
		return "t", true
	case ui.KeyU:
		return "u", true
	case ui.KeyV:
		return "v", true
	case ui.KeyW:
		return "w", true
	case ui.KeyX:
		return "x", true
	case ui.KeyY:
		return "y", true
	case ui.KeyZ:
		return "z", true
	case ui.KeyBack:
		return "back", true
	case ui.KeyForward:
		return "forward", true
	default:
		return "", false
	}
}

// BindingTargets returns editor binding candidates for one MyGo event in
// lookup order. Logical layout keys are authoritative. The ANSI physical key is
// considered only when the layout reports no logical key and a shortcut
// modifier is held, matching MyGo's positional-shortcut contract without
// disrupting typing on non-US layouts.
func BindingTargets(event ui.InputEvent, platform Platform) []kernel.BindingTarget {
	if event.Kind != ui.InputKeyDown {
		return nil
	}
	modifiers := NormalizeModifiers(event.Mods, platform)
	targets := make([]kernel.BindingTarget, 0, 2)
	if key, ok := KeyName(event.Key); ok {
		targets = append(targets, kernel.NewBindingTarget(
			kernel.EditorBindingContext,
			key,
			modifiers,
		))
	}
	if event.Key == ui.KeyUnknown &&
		event.PhysicalKey != ui.KeyUnknown &&
		hasShortcutModifier(event.Mods) {
		if key, ok := KeyName(event.PhysicalKey); ok {
			target := kernel.NewBindingTarget(
				kernel.EditorBindingContext,
				key,
				modifiers,
			)
			if len(targets) == 0 || targets[0] != target {
				targets = append(targets, target)
			}
		}
	}
	return targets
}

// ResolveKey returns the configured command for a key-down event. It performs
// no action and never consults mutable element or buffer state.
func ResolveKey(
	configuration *kernel.PreparedPolicyConfiguration,
	event ui.InputEvent,
	platform Platform,
) (kernel.CommandID, bool) {
	if configuration == nil {
		return "", false
	}
	for _, target := range BindingTargets(event, platform) {
		if command, ok := configuration.CommandFor(target); ok {
			return command, true
		}
	}
	return "", false
}

func hasShortcutModifier(modifiers ui.Modifiers) bool {
	return modifiers&(ui.Ctrl|ui.Alt|ui.Super) != 0
}
