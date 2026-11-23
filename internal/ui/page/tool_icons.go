package page

import (
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
)

// toolIcons are the icons the tools draw offered commands with: the one
// table of the page that names commands, for how they look only. A
// command it does not name draws the default (toolIcon); a choice's draw
// none.
var toolIcons = map[vocab.CommandID]components.IconName{
	"figures.insert":   components.IconImage,
	"table.insert":     components.IconTable,
	"formats.strong":   components.IconBold,
	"formats.emphasis": components.IconItalic,
	"formats.code":     components.IconCode,
	"formats.strike":   components.IconStrike,
	"scopes.shrink":    components.IconShrinkSelection,
	"scopes.expand":    components.IconExpandSelection,
}

// toolIcon is an offered command's icon, the default where the table
// names none.
func toolIcon(id vocab.CommandID) components.IconName {
	if icon, ok := toolIcons[id]; ok {
		return icon
	}
	return components.IconCommand
}
