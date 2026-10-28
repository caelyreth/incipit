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
	commands = append(commands,
		overlay.Command{ID: "workspace.new", Label: "New Page", Group: "Workspace"},
		overlay.Command{ID: "surface.copy", Label: "Copy", Group: "Clipboard"},
		overlay.Command{ID: "surface.cut", Label: "Cut", Group: "Clipboard"},
		overlay.Command{ID: "surface.paste", Label: "Paste", Group: "Clipboard"},
	)
	return commands
}
