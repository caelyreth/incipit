package window

import (
	"strings"

	"github.com/caelyreth/neverland/app/keymap"
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
	snapshot := a.Store.Snapshot()
	prepared, catalog := snapshot.Prepared, snapshot.Catalog
	if prepared == nil {
		return nil
	}
	commands := make([]overlay.Command, 0, 48)
	seen := make(map[kernel.CommandID]bool)
	for _, policy := range catalog.Policies() {
		for _, id := range policy.Commands {
			if seen[id] || !prepared.IsCommandEnabled(id) {
				continue
			}
			command, ok := editor.CommandFor(id)
			if !ok || command.ID == editor.SurfaceCommandsCommandID {
				continue
			}
			action, ok := keymap.ResolveCommand(id)
			if !ok || !a.commandEnabledWith(prepared, action) {
				continue
			}
			if command.Intent.Kind == editor.IntentMove && command.Intent.By == editor.ByRow {
				continue
			}
			seen[id] = true
			commands = append(commands, overlay.Command{
				ID:      id,
				Label:   command.Label,
				Group:   commandGroup(policy.ID, id),
				Binding: bindingLabel(prepared, id),
				Keys:    bindingKeys(prepared, id),
			})
		}
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
	case editor.WritingPolicyID:
		switch {
		case command == editor.InsertTableCommandID:
			return "Insert"
		case strings.HasPrefix(string(command), "table."):
			return "Table"
		}
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
		case editor.SurfaceInsert:
			return "Insert"
		default:
			return "Workspace"
		}
	default:
		return "Application"
	}
}
