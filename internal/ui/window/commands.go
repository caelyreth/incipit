package window

import (
	"github.com/caelyreth/incipit/internal/app/keymap"
	"github.com/caelyreth/incipit/internal/ui/shell"
)

// commandCatalog returns every command the pill can run, grouped by what
// it acts on. The command that asks for them is left out.
func (a *AppState) commandCatalog() []shell.Command {
	keys := a.Product.Keymap
	declared := keys.Commands()
	commands := make([]shell.Command, 0, len(declared))
	for _, scope := range keymap.Scopes {
		if scope == keymap.ScopeEditing {
			continue
		}
		for _, command := range declared {
			if command.Scope != scope || command.ID == keymap.Commands || !a.commandState(command.ID, command.Label()).Enabled {
				continue
			}
			commands = append(commands, shell.Command{
				ID:      string(command.ID),
				Label:   a.commandState(command.ID, command.Label()).Label,
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
