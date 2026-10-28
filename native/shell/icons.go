package shell

import "github.com/egoist/mygo/ui"

// Icon identifies a chrome glyph. SVGs are parsed once when the package
// loads, as MyGo's SVG documentation requires.
type Icon uint8

const (
	IconToolbar Icon = iota
	IconSidebar
	IconDocument
	IconClose
	IconPlus
	IconSearch
	IconSettings
	IconItalic
	IconBold
	IconUndo
	IconRedo
	IconBackground
	IconFocus
	IconClearFormatting
)

type iconDefinition struct {
	label string
	svg   *ui.SVG
}

var iconSet = map[Icon]iconDefinition{
	IconToolbar: {
		label: "Toolbar",
		svg:   mustIcon(`<path d="M4 5h16v5H4zM4 14h7v5H4zM15 14h5v5h-5z"/>`),
	},
	IconSidebar: {
		label: "Sidebar",
		svg:   mustIcon(`<rect x="3" y="4" width="18" height="16" rx="3"/><path d="M9 4v16"/>`),
	},
	IconDocument: {
		label: "Page",
		svg:   mustIcon(`<path d="M6 3h8l4 4v14H6zM14 3v5h4M9 12h6M9 16h6"/>`),
	},
	IconClose: {
		label: "Close",
		svg:   mustIcon(`<path d="M6 6l12 12M18 6 6 18"/>`),
	},
	IconPlus: {
		label: "Add",
		svg:   mustIcon(`<path d="M12 5v14M5 12h14"/>`),
	},
	IconSearch: {
		label: "Search",
		svg:   mustIcon(`<circle cx="10.5" cy="10.5" r="6.5"/><path d="m15.5 15.5 4 4"/>`),
	},
	IconSettings: {
		label: "Settings",
		svg: mustIcon(`<circle cx="12" cy="12" r="3"/>
			<path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.86 2.86-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21H9.55v-.1A1.7 1.7 0 0 0 8.5 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.86-2.86.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H2.8V9.55h.1A1.7 1.7 0 0 0 4.6 8.5a1.7 1.7 0 0 0-.34-1.88l-.06-.06L7.06 3.7l.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1v-.1h4.05v.1A1.7 1.7 0 0 0 15.5 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.86 2.86-.06.06A1.7 1.7 0 0 0 19.4 9c.16.36.38.7.66.98.29.28.65.47 1.04.47h.1v4.05h-.1a1.7 1.7 0 0 0-1.7.5z"/>`),
	},
	IconItalic: {
		label: "Italic",
		svg:   mustIcon(`<path d="M10 5h8M6 19h8M15 5l-4 14"/>`),
	},
	IconBold: {
		label: "Bold",
		svg:   mustIcon(`<path d="M7 5h6a4 4 0 0 1 0 8H7zM7 13h7a3 3 0 0 1 0 6H7z"/>`),
	},
	IconUndo: {
		label: "Undo",
		svg:   mustIcon(`<path d="m9 6-5 5 5 5M4 11h9a5 5 0 0 1 5 5v1"/>`),
	},
	IconRedo: {
		label: "Redo",
		svg:   mustIcon(`<path d="m15 6 5 5-5 5M20 11h-9a5 5 0 0 0-5 5v1"/>`),
	},
	IconBackground: {
		label: "Background",
		svg: mustIcon(`<rect x="7" y="4" width="10" height="5" rx="2"/>
			<path d="M3 12h18"/><rect x="7" y="15" width="10" height="5" rx="2"/>`),
	},
	IconFocus: {
		label: "Focus",
		svg:   mustIcon(`<path d="M8 3H5a2 2 0 0 0-2 2v3M16 3h3a2 2 0 0 1 2 2v3M8 21H5a2 2 0 0 1-2-2v-3M16 21h3a2 2 0 0 0 2-2v-3"/>`),
	},
	IconClearFormatting: {
		label: "Clear formatting",
		svg:   mustIcon(`<path d="M4 19h16M7 15 11 6h2l4 9M8 12h7M3 3l18 18"/>`),
	},
}

// allIcons keeps test coverage and future palettes from silently missing a
// glyph in the shared set.
var allIcons = [...]Icon{
	IconToolbar,
	IconSidebar,
	IconDocument,
	IconClose,
	IconPlus,
	IconSearch,
	IconSettings,
	IconItalic,
	IconBold,
	IconUndo,
	IconRedo,
	IconBackground,
	IconFocus,
	IconClearFormatting,
}

func mustIcon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

func glyph(icon Icon) *ui.SVG {
	return iconSet[icon].svg
}

func glyphLabel(icon Icon) string {
	return iconSet[icon].label
}
