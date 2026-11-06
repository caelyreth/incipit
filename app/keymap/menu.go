package keymap

import (
	"strings"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// A native menu takes the keys of its items before any view sees them: on
// macOS a menu's key equivalent turns the key into the item's command, so a
// view hears "bold" rather than the keys pressed. Neverland keeps the two in
// step. The items that perform keymap actions take their keys from the
// keymap, so a remap moves the menu's key too; and while a shortcut is being
// recorded the menu stands its keys down, so the recorder hears every chord.

// MenuCommand pairs a menu's edit or format command, as MyGo names it, with
// the action it performs.
type MenuCommand struct {
	Name    string
	Command kernel.CommandID
}

// MenuCommands are the edit and format commands the editor answers.
var MenuCommands = [...]MenuCommand{
	{"undo", editor.UndoCommandID},
	{"redo", editor.RedoCommandID},
	{"cut", CutCommandID},
	{"copy", CopyCommandID},
	{"paste", PasteCommandID},
	{"selectAll", editor.SelectAllCommandID},
	{"bold", editor.StrongCommandID},
	{"italic", editor.EmphasisCommandID},
}

// MenuCommandFor returns the action a menu command performs.
func MenuCommandFor(name string) (kernel.CommandID, bool) {
	if name == "pasteAndMatchStyle" {
		name = "paste"
	}
	for _, entry := range MenuCommands {
		if entry.Name == name {
			return entry.Command, true
		}
	}
	return "", false
}

// Accelerator spells target as a MyGo menu accelerator. A chord no menu can
// show, as one in another binding context or with Fn, has none.
func Accelerator(target kernel.BindingTarget, platform Platform) (string, bool) {
	if target.Context() != kernel.EditorBindingContext || target.Key() == "" ||
		target.Modifiers().Has(kernel.ModifierFunction) {
		return "", false
	}
	if platform == PlatformCurrent {
		platform = CurrentPlatform()
	}
	parts := make([]string, 0, 5)
	modifiers := target.Modifiers()
	if modifiers.Has(kernel.ModifierPrimary) {
		parts = append(parts, "CmdOrCtrl")
	}
	if modifiers.Has(kernel.ModifierControl) {
		parts = append(parts, "Ctrl")
	}
	if modifiers.Has(kernel.ModifierSuper) {
		parts = append(parts, "Super")
	}
	if modifiers.Has(kernel.ModifierAlt) {
		parts = append(parts, "Alt")
	}
	if modifiers.Has(kernel.ModifierShift) {
		parts = append(parts, "Shift")
	}
	key := target.Key()
	if key == "+" {
		key = "Plus"
	}
	return strings.Join(append(parts, key), "+"), true
}

// ParseAccelerator reads a MyGo menu accelerator as the chord it takes.
func ParseAccelerator(accelerator string, platform Platform) (kernel.BindingTarget, bool) {
	if platform == PlatformCurrent {
		platform = CurrentPlatform()
	}
	parts := strings.Split(accelerator, "+")
	if strings.HasSuffix(accelerator, "++") {
		parts = append(parts[:len(parts)-2], "+")
	}
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return kernel.BindingTarget{}, false
	}
	var modifiers kernel.Modifiers
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "cmdorctrl", "commandorcontrol":
			modifiers = modifiers.With(kernel.ModifierPrimary)
		case "cmd", "command", "super", "meta":
			if platform == PlatformDarwin {
				modifiers = modifiers.With(kernel.ModifierPrimary)
			} else {
				modifiers = modifiers.With(kernel.ModifierSuper)
			}
		case "ctrl", "control":
			if platform == PlatformDarwin {
				modifiers = modifiers.With(kernel.ModifierControl)
			} else {
				modifiers = modifiers.With(kernel.ModifierPrimary)
			}
		case "alt", "option":
			modifiers = modifiers.With(kernel.ModifierAlt)
		case "shift":
			modifiers = modifiers.With(kernel.ModifierShift)
		default:
			return kernel.BindingTarget{}, false
		}
	}
	key := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if key == "plus" {
		key = "+"
	}
	return kernel.NewBindingTarget(kernel.EditorBindingContext, key, modifiers), true
}
