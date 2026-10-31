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
	IconBack         IconName = "chevron-left"
	IconBold         IconName = "bold"
	IconCheck        IconName = "check"
	IconChevronDown  IconName = "chevron-down"
	IconChevronRight IconName = "chevron-right"
	IconClock        IconName = "clock"
	IconClose        IconName = "x"
	IconDocument     IconName = "file-text"
	IconFolder       IconName = "folder"
	IconInbox        IconName = "inbox"
	IconInfo         IconName = "info"
	IconItalic       IconName = "italic"
	IconKeyboard     IconName = "keyboard"
	IconLanguages    IconName = "languages"
	IconMinus        IconName = "minus"
	IconNewGroup     IconName = "folder-plus"
	IconNewSheet     IconName = "file-plus"
	IconPlus         IconName = "plus"
	IconRedo         IconName = "redo-2"
	IconSearch       IconName = "search"
	IconSettings     IconName = "settings"
	IconSheets       IconName = "files"
	IconSidebar      IconName = "panel-left"
	IconTrash        IconName = "trash-2"
	IconTypewriter   IconName = "align-vertical-justify-center"
	IconUndo         IconName = "undo-2"
	IconWarning      IconName = "triangle-alert"
	IconZen          IconName = "maximize"
)

var AllIcons = [...]IconName{
	IconBack,
	IconBold,
	IconCheck,
	IconChevronDown,
	IconChevronRight,
	IconClock,
	IconClose,
	IconDocument,
	IconFolder,
	IconInbox,
	IconInfo,
	IconItalic,
	IconKeyboard,
	IconLanguages,
	IconMinus,
	IconNewGroup,
	IconNewSheet,
	IconPlus,
	IconRedo,
	IconSearch,
	IconSettings,
	IconSheets,
	IconSidebar,
	IconTrash,
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
