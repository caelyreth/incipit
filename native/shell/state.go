package shell

import (
	"github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Page is an open tab: the sheet it shows.
type Page struct {
	ID    string
	Title string
}

// FormatState is one formatting action's state: whether it applies at the
// selection or caret, and whether the engine preview says it can.
type FormatState struct {
	On      bool
	Enabled bool
}

// FormattingState is the formatting the floating toolbar shows from the
// open page's previews.
type FormattingState struct {
	Strong, Emphasis, Code, Strike FormatState
}

// ToolbarState is the writing modes and formatting the floating toolbar
// switches, and their current shortcuts.
type ToolbarState struct {
	Typewriter        bool
	Zen               bool
	TypewriterEnabled bool
	ZenEnabled        bool
	Formatting        FormattingState
	Bindings          ToolbarBindings
}

// ToolbarBindings carries the current shortcut text for action tooltips.
type ToolbarBindings struct {
	Typewriter string
	Zen        string
	Strong     string
	Emphasis   string
	Code       string
	Strike     string
}

type State struct {
	// root is the shell's outermost element, for where the focus is.
	root ui.Handle
	// tagWidths are tag chips' text widths, measured once each.
	tagWidths map[tagWidthKey]float32

	Appearance settings.PresentationState
	Settings   theme.Settings
	// Colors carries theme changes; nil shows Settings as they are.
	Colors *theme.Transition
	// Tabs are the open tabs, in order; ActivePage is the one in front.
	Tabs       []Page
	ActivePage string
	// Library is what the sidebar shows and files; nil shows none.
	Library         Library
	Statistics      string
	SearchQuery     string
	SearchRequested bool
	// FindQuery is a sheet's find as written after its fixed $sheet (the
	// query it runs is FindText), FindRequested asks its field
	// for the focus, and FindCount is what it found, as the window counts
	// it.
	FindQuery         string
	FindRequested     bool
	FindCount         FindCount
	finder            findBar
	Toolbar           ToolbarState
	TabsScroll        ui.ScrollState
	libraryScroll     ui.ScrollState
	sheetScroll       ui.ScrollState
	tabsOverscroll    scrollfx.Overscroll
	libraryOverscroll scrollfx.Overscroll
	sheetOverscroll   scrollfx.Overscroll
	WindowActive      bool

	Content      func(c *ui.Context)
	SelectPage   func(id string)
	NewPage      func()
	ClosePage    func()
	ClosePageID  func(id string)
	OpenSettings func()
	OpenCommands func()
	FindPage     func()
	// Find runs a sheet's find on the page in front; FindStep goes to the
	// next match or the one before; EndFind ends it.
	Find     func(query string)
	FindStep func(forward bool)
	EndFind  func()
	// MoveTab puts a tab at index among the tabs.
	MoveTab func(id string, index int)
	// CloseOtherTabs closes every tab but one; CloseTabsAfter those after
	// it.
	CloseOtherTabs func(id string)
	CloseTabsAfter func(id string)
	Typewriter     func(bool)
	Zen            func(bool)
	Strong         func()
	Emphasis       func()
	Code           func()
	Strike         func()

	// SidebarOverride shows or hides the library, SheetListOverride the
	// sheet list, against what the window's width would choose.
	SidebarOverride   *bool
	SheetListOverride *bool
	// panes are how wide the library and the sheet list were dragged, at
	// the reference UI size; zero is the layout's own width.
	panes            paneWidths
	edgeHover        edgeHover
	nav              navigation
	lastActive       string
	lastWidth        float32
	lastUIFontSize   float32
	lastSidebarWidth float32
	lastChrome       float32
	// listDock is how far the place's title has gone from the title bar
	// to the top of the sheet list, as it does where the library is closed.
	listDock float32
	// dockTarget is where listDock goes, held while the list is shut.
	dockTarget float32
	// flight is whether that title is on its way this frame (takeFlight).
	flight bool
	// canvas is the notebook on one surface, in place of the columns.
	canvas canvasState
}

func New() *State {
	return &State{
		Appearance:   settings.DefaultPresentation(),
		Settings:     theme.DefaultSettings(),
		WindowActive: true,
		nav:          navigation{expanded: map[string]bool{}, sheetDropSlot: -1},
		Toolbar: ToolbarState{
			TypewriterEnabled: true,
			ZenEnabled:        true,
		},
	}
}

func (s *State) SetWindowActive(active bool) {
	if s != nil {
		s.WindowActive = active
	}
}

func (s *State) ToggleSidebar() {
	if s == nil {
		return
	}
	visible := s.SidebarVisible()
	next := !visible
	s.SidebarOverride = &next
	s.closeCanvas()
}

// ColumnMargin is the room beside the page column: the gutter and half
// of what the page's width leaves over the column.
func (s *State) ColumnMargin() float32 {
	scale := s.Settings.UIFontSize
	gutter := theme.DefaultLayout.ContentGutter * scale / theme.DefaultUIFontSize
	column := s.Settings.Prose().Column()
	return gutter + max(0, (s.lastWidth-s.lastSidebarWidth-2*gutter-column)/2)
}

// ShowPanes shows the library and the sheet list as asked.
func (s *State) ShowPanes(library, sheets bool) {
	if s != nil {
		s.SidebarOverride, s.SheetListOverride = &library, &sheets
		s.closeCanvas()
	}
}

func (s *State) SidebarVisible() bool {
	if s == nil || s.Appearance.Zen {
		return false
	}
	if s.SidebarOverride != nil {
		return *s.SidebarOverride
	}
	scale := s.lastUIFontSize
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	return s.lastWidth >= theme.DefaultLayout.Narrow*scale/theme.DefaultUIFontSize
}

func (s *State) animate(c *ui.Context) bool {
	return s != nil && components.MotionEnabled(c, s.Appearance.Motion, s.WindowActive)
}

// paneWidths are the library's and the sheet list's dragged widths.
type paneWidths struct {
	library, sheets float32
}

// paneLayout is the window's reference layout with the library and the
// sheet list as wide as they were dragged, each within its range.
func (s *State) paneLayout() theme.Layout {
	layout := theme.DefaultLayout
	if s == nil {
		return layout
	}
	if s.panes.library > 0 {
		layout.Sidebar = max(layout.SidebarMin, min(s.panes.library, layout.SidebarMax))
	}
	if s.panes.sheets > 0 {
		layout.SheetList = max(layout.SheetListMin, min(s.panes.sheets, layout.SheetListMax))
	}
	return layout
}
