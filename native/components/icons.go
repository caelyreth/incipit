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
	IconBookmark     IconName = "bookmark"
	IconCheck        IconName = "check"
	IconCheckCircle  IconName = "circle-check"
	IconChevronDown  IconName = "chevron-down"
	IconChevronRight IconName = "chevron-right"
	IconChevronUp    IconName = "chevron-up"
	IconClock        IconName = "clock"
	IconClose        IconName = "x"
	IconCode         IconName = "code"
	IconCommand      IconName = "command"
	IconDocument     IconName = "file-text"
	IconError        IconName = "circle-alert"
	IconFolder       IconName = "folder"
	IconFolderSearch IconName = "folder-search"
	IconGrip         IconName = "grip-vertical"
	IconGripAcross   IconName = "grip-horizontal"
	IconImage        IconName = "image"
	IconImageOff     IconName = "image-off"
	IconInbox        IconName = "inbox"
	IconInfo         IconName = "info"
	IconItalic       IconName = "italic"
	IconKeyboard     IconName = "keyboard"
	IconLanguages    IconName = "languages"
	IconMap          IconName = "map"
	IconMinus        IconName = "minus"
	IconNewGroup     IconName = "folder-plus"
	IconNewSheet     IconName = "file-plus"
	IconPlus         IconName = "plus"
	IconRedo         IconName = "redo-2"
	IconSearch       IconName = "search"
	IconSettings     IconName = "settings"
	IconSheets       IconName = "files"
	IconSidebar      IconName = "panel-left"
	IconSidebarClose IconName = "panel-left-close"
	IconStrike       IconName = "strikethrough"
	IconTrash        IconName = "trash-2"
	IconTypewriter   IconName = "align-vertical-justify-center"
	IconUndo         IconName = "undo-2"
	IconWarning      IconName = "triangle-alert"
	IconZen          IconName = "maximize"
)

var AllIcons = [...]IconName{
	IconBack,
	IconBold,
	IconBookmark,
	IconCheck,
	IconCheckCircle,
	IconChevronDown,
	IconChevronRight,
	IconChevronUp,
	IconClock,
	IconClose,
	IconCode,
	IconCommand,
	IconDocument,
	IconError,
	IconFolder,
	IconFolderSearch,
	IconGrip,
	IconGripAcross,
	IconImage,
	IconImageOff,
	IconInbox,
	IconInfo,
	IconItalic,
	IconKeyboard,
	IconLanguages,
	IconMap,
	IconMinus,
	IconNewGroup,
	IconNewSheet,
	IconPlus,
	IconRedo,
	IconSearch,
	IconSettings,
	IconSheets,
	IconSidebar,
	IconSidebarClose,
	IconStrike,
	IconTrash,
	IconTypewriter,
	IconUndo,
	IconWarning,
	IconZen,
}

// GroupIcons are the icons a group can wear, in the order the picker
// shows them: the folder first, as every group starts.
var GroupIcons = [...]IconName{
	"folder", "book", "book-open", "notebook-pen", "feather", "rocket",
	"pen-line", "quote", "scroll", "newspaper", "bookmark", "tag",
	"lightbulb", "sparkles", "star", "heart", "flag", "archive",
	"briefcase", "graduation-cap", "code", "puzzle", "palette", "music",
	"camera", "film", "globe", "map", "compass", "mountain",
	"leaf", "coffee", "moon", "sun", "users", "calendar",
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
		icons = make(map[IconName]*ui.SVG, len(AllIcons)+len(GroupIcons))
		for _, name := range append(AllIcons[:], GroupIcons[:]...) {
			if icons[name] != nil {
				continue
			}
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
