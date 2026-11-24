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
	IconBack            IconName = "chevron-left"
	IconBold            IconName = "bold"
	IconBookmark        IconName = "bookmark"
	IconCheck           IconName = "check"
	IconCheckCircle     IconName = "circle-check"
	IconChevronDown     IconName = "chevron-down"
	IconChevronRight    IconName = "chevron-right"
	IconChevronUp       IconName = "chevron-up"
	IconClose           IconName = "x"
	IconCode            IconName = "code"
	IconCommand         IconName = "command"
	IconDesk            IconName = "lamp-desk"
	IconError           IconName = "circle-alert"
	IconFolder          IconName = "folder"
	IconFileText        IconName = "file-text"
	IconExpandSelection IconName = "expand-selection"
	IconShrinkSelection IconName = "shrink-selection"
	IconFolderSearch    IconName = "folder-search"
	IconGrip            IconName = "grip-vertical"
	IconGripAcross      IconName = "grip-horizontal"
	IconHome            IconName = "house"
	IconImage           IconName = "image"
	IconImageOff        IconName = "image-off"
	IconInbox           IconName = "inbox"
	IconInfo            IconName = "info"
	IconItalic          IconName = "italic"
	IconKeyboard        IconName = "keyboard"
	IconMap             IconName = "map"
	IconMinus           IconName = "minus"
	IconNewGroup        IconName = "folder-plus"
	IconNewSheet        IconName = "file-plus"
	IconPen             IconName = "pen-line"
	IconPin             IconName = "pin"
	IconPencilSpark     IconName = "pencil-sparkles"
	IconPlus            IconName = "plus"
	IconSearch          IconName = "search"
	IconSettings        IconName = "settings"
	IconStrike          IconName = "strikethrough"
	IconTable           IconName = "table"
	IconTrash           IconName = "trash-2"
	IconList            IconName = "list"
	IconTray            IconName = "layers"
	IconTypewriter      IconName = "align-vertical-justify-center"
	IconUndo            IconName = "undo-2"
	IconWarning         IconName = "triangle-alert"
	IconExport          IconName = "share"
	IconDuplicate       IconName = "files"
	IconColumns         IconName = "columns-2"
	IconPalette         IconName = "palette"
	IconAlignStart      IconName = "text-align-start"
	IconAlignCenter     IconName = "text-align-center"
	IconAlignEnd        IconName = "text-align-end"
	IconRowAbove        IconName = "row-insert-above"
	IconRowBelow        IconName = "row-insert-below"
	IconColumnLeft      IconName = "column-insert-left"
	IconColumnRight     IconName = "column-insert-right"
	IconRowRemove       IconName = "row-remove"
	IconColumnRemove    IconName = "column-remove"
	IconPreview         IconName = "eye"
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
	IconClose,
	IconCode,
	IconCommand,
	IconDesk,
	IconError,
	IconExpandSelection,
	IconFileText,
	IconFolder,
	IconFolderSearch,
	IconGrip,
	IconGripAcross,
	IconHome,
	IconImage,
	IconImageOff,
	IconInbox,
	IconInfo,
	IconItalic,
	IconKeyboard,
	IconMap,
	IconMinus,
	IconNewGroup,
	IconNewSheet,
	IconPen,
	IconPin,
	IconPencilSpark,
	IconPlus,
	IconSearch,
	IconSettings,
	IconShrinkSelection,
	IconStrike,
	IconTable,
	IconTrash,
	IconList,
	IconTray,
	IconTypewriter,
	IconUndo,
	IconWarning,
	IconExport,
	IconDuplicate,
	IconColumns,
	IconPalette,
	IconAlignStart,
	IconAlignCenter,
	IconAlignEnd,
	IconRowAbove,
	IconRowBelow,
	IconColumnLeft,
	IconColumnRight,
	IconRowRemove,
	IconColumnRemove,
	IconPreview,
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
