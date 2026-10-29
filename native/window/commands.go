package window

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/overlay"
)

// commandCatalog returns every command the palette and native menus can
// invoke through the controller. Grouping is display-only; execution uses
// the stable command IDs.
func (a *AppState) commandCatalog() []overlay.Command {
	if a == nil || a.Store == nil {
		return nil
	}
	prepared := a.Store.PreparedConfiguration()
	catalog := a.Store.PolicyCatalog()
	if prepared == nil {
		return nil
	}
	metadata := commandMetadata()
	active := false
	if a.Workspace != nil {
		_, active = a.Workspace.Active()
	}
	commands := make([]overlay.Command, 0, 48)
	seen := make(map[kernel.CommandID]bool)
	for _, policy := range catalog.Policies() {
		for _, id := range policy.Commands {
			if seen[id] || !prepared.IsCommandEnabled(id) {
				continue
			}
			command, ok := metadata[id]
			if !ok || command.ID == editor.SurfaceCommandsCommandID {
				continue
			}
			if surface, ok := editor.SurfaceCommandForID(id); ok {
				if surface.Kind == editor.SurfaceVisual {
					continue
				}
				if !active && surface.Kind != editor.SurfaceApp &&
					id != editor.SurfaceNewPageCommandID {
					continue
				}
			} else if !active {
				continue
			}
			seen[id] = true
			commands = append(commands, overlay.Command{
				ID:      id,
				Label:   command.Label,
				Group:   commandGroup(policy.ID, id),
				Binding: bindingLabel(prepared, id),
			})
		}
	}
	return commands
}

func commandMetadata() map[kernel.CommandID]editor.Command {
	commands := make(map[kernel.CommandID]editor.Command, 48)
	for _, command := range editor.AllTextCommands {
		meta := command.Description()
		commands[meta.ID] = meta
	}
	for _, command := range editor.AllHistoryCommands {
		meta := command.Description()
		commands[meta.ID] = meta
	}
	for _, command := range editor.AllWordCommands {
		meta := command.Description()
		commands[meta.ID] = meta
	}
	for _, meta := range []editor.Command{
		editor.MarkupEmphasis.Description(),
		editor.MarkupStrong.Description(),
		editor.TabCommand(),
	} {
		commands[meta.ID] = meta
	}
	for _, command := range editor.SurfaceCommands() {
		meta := command.Description()
		commands[meta.ID] = meta
	}
	return commands
}

func commandGroup(policy kernel.PolicyID, command kernel.CommandID) string {
	switch policy {
	case editor.TextPolicyID:
		return "Text"
	case editor.HistoryPolicyID:
		return "History"
	case editor.WordsPolicyID:
		return "Words"
	case editor.MarkupPolicyID:
		return "Format"
	case editor.NavigationPolicyID:
		return "Navigation"
	case editor.SurfacePolicyID:
		surface, _ := editor.SurfaceCommandForID(command)
		switch surface.Kind {
		case editor.SurfaceClipboard:
			return "Clipboard"
		case editor.SurfacePage:
			return "Workspace"
		case editor.SurfacePresentation, editor.SurfacePolicy:
			return "Appearance"
		case editor.SurfaceApp:
			return "Application"
		default:
			return "Workspace"
		}
	default:
		return "Application"
	}
}
