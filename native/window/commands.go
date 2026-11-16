package window

import (
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/native/shell"
)

// commandCatalog returns every command the pill can run, grouped by what
// it acts on. The command that asks for them is left out.
func (a *AppState) commandCatalog() []shell.Command {
	keys := a.Product.Keymap
	declared := keys.Commands()
	commands := make([]shell.Command, 0, len(declared))
	for _, scope := range []keymap.Scope{keymap.ScopePages, keymap.ScopeWindow, keymap.ScopeApp} {
		for _, command := range declared {
			if command.Scope != scope || command.ID == keymap.Commands {
				continue
			}
			commands = append(commands, shell.Command{
				ID:      string(command.ID),
				Label:   command.Label,
				Group:   scope.Title(),
				Binding: bindingLabel(keys, command.ID),
				Keys:    bindingKeys(keys, command.ID),
			})
		}
	}
	return commands
}

// bindingLabel is command's key as one chord, or nothing.
func bindingLabel(keys *keymap.Keymap, command keymap.CommandID) string {
	return bindingLabelFor(keys, command, keymap.PlatformCurrent)
}

func bindingLabelFor(keys *keymap.Keymap, command keymap.CommandID, platform keymap.Platform) string {
	chord, ok := keys.ChordFor(command)
	if !ok {
		return ""
	}
	return keymap.Spell(chord, platform)
}

// bindingKeys is the caps of command's key.
func bindingKeys(keys *keymap.Keymap, command keymap.CommandID) []string {
	chord, ok := keys.ChordFor(command)
	if !ok {
		return nil
	}
	return keymap.Keycaps(chord, keymap.PlatformCurrent)
}
