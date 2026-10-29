package window

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/native/overlay"
)

// commandCatalog returns every command the palette and native menus can
// invoke through the controller. Grouping is display-only; execution uses
// the stable command IDs.
func commandCatalog() []overlay.Command {
	commands := make([]overlay.Command, 0, 48)
	appendMeta := func(group string, meta editor.Command) {
		commands = append(commands, overlay.Command{
			ID:      meta.ID,
			Label:   meta.Label,
			Group:   group,
			Binding: meta.Binding().String(),
		})
	}
	for _, command := range editor.AllTextCommands {
		appendMeta("Text", command.Description())
	}
	for _, command := range editor.AllHistoryCommands {
		appendMeta("History", command.Description())
	}
	for _, command := range editor.AllWordCommands {
		appendMeta("Words", command.Description())
	}
	appendMeta("Format", editor.MarkupEmphasis.Description())
	appendMeta("Format", editor.MarkupStrong.Description())
	appendMeta("Navigation", editor.TabCommand())
	for _, command := range editor.SurfaceCommands() {
		group := "Workspace"
		switch command.Kind {
		case editor.SurfaceVisual:
			group = "Navigation"
		case editor.SurfaceClipboard:
			group = "Clipboard"
		case editor.SurfacePresentation, editor.SurfacePolicy:
			group = "Appearance"
		case editor.SurfaceApp:
			group = "Application"
		}
		appendMeta(group, command.Description())
	}
	return commands
}
