package components

import (
	"embed"
	"fmt"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg
var iconFiles embed.FS

type IconName string

const (
	IconBold         IconName = "bold"
	IconCheck        IconName = "check"
	IconChevronDown  IconName = "chevron-down"
	IconChevronRight IconName = "chevron-right"
	IconClose        IconName = "x"
	IconDocument     IconName = "file-text"
	IconInfo         IconName = "info"
	IconItalic       IconName = "italic"
	IconKeyboard     IconName = "keyboard"
	IconLanguages    IconName = "languages"
	IconMinus        IconName = "minus"
	IconPlus         IconName = "plus"
	IconRedo         IconName = "redo-2"
	IconSearch       IconName = "search"
	IconSidebar      IconName = "panel-left"
	IconSettings     IconName = "settings"
	IconTypewriter   IconName = "align-vertical-justify-center"
	IconUndo         IconName = "undo-2"
	IconWarning      IconName = "triangle-alert"
	IconZen          IconName = "maximize"
)

var AllIcons = [...]IconName{
	IconBold,
	IconCheck,
	IconChevronDown,
	IconChevronRight,
	IconClose,
	IconDocument,
	IconInfo,
	IconItalic,
	IconKeyboard,
	IconLanguages,
	IconMinus,
	IconPlus,
	IconRedo,
	IconSearch,
	IconSidebar,
	IconSettings,
	IconTypewriter,
	IconUndo,
	IconWarning,
	IconZen,
}

var (
	iconOnce sync.Once
	icons    map[IconName]*ui.SVG
	iconErr  error
)

func (n IconName) SVG() *ui.SVG {
	loadIcons()
	return icons[n]
}

func loadIcons() {
	iconOnce.Do(func() {
		icons = make(map[IconName]*ui.SVG, len(AllIcons))
		for _, name := range AllIcons {
			data, err := iconFiles.ReadFile("icons/" + string(name) + ".svg")
			if err != nil {
				iconErr = fmt.Errorf("components: read icon %s: %w", name, err)
				return
			}
			icons[name] = ui.MustParseSVG(data)
		}
	})
	if iconErr != nil {
		panic(iconErr)
	}
}
