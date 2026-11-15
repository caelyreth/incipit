package shell

import (
	"github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Page is an open sheet.
type Page struct {
	ID    string
	Title string
}

// SheetFrame is how a sheet's page is set where the shell shows it: in the
// columns, or on the canvas, nearer or farther.
type SheetFrame struct {
	// Prose and UIFontSize are the type the page is set in, which the
	// canvas scales as it zooms.
	Prose      theme.ProseStyle
	UIFontSize float32
	// Top and Bottom are the reading space above its first line and below
	// its last, scrolled with it; Dock is the room the floating writing
	// actions take over its foot.
	Top, Bottom, Dock float32
	// Focus lets the page take the keyboard while nothing else holds it.
	Focus bool
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
	// Open are the open sheets, in order; ActivePage is the one in front,
	// which the columns show, and the canvas shows them all.
	Open       []Page
	ActivePage string
	// Library is what the sidebar shows and files; nil shows none.
	Library         Library
	Statistics      string
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
	libraryScroll     ui.ScrollState
	libraryOverscroll scrollfx.Overscroll
	WindowActive      bool

	// Sheet draws an open sheet's page, as frame sets it.
	Sheet        func(c *ui.Context, id string, frame SheetFrame)
	SelectPage   func(id string)
	ClosePageID  func(id string)
	OpenSettings func()
	OpenCommands func()
	// Find runs a sheet's find on the page in front; FindStep goes to the
	// next match or the one before; EndFind ends it.
	Find       func(query string)
	FindStep   func(forward bool)
	EndFind    func()
	Typewriter func(bool)
	Zen        func(bool)
	Strong     func()
	Emphasis   func()
	Code       func()
	Strike     func()

	// SidebarOverride shows or hides the library, SheetListOverride the
	// sheet list, against what the window's width would choose.
	SidebarOverride   *bool
	SheetListOverride *bool
	// panes are how wide the library and the sheet list were dragged, at
	// the reference UI size; zero is the layout's own width.
	panes     paneWidths
	edgeHover edgeHover
	nav       navigation
	// list is the sheet list's browser.
	list             browser
	lastWidth        float32
	lastUIFontSize   float32
	lastSidebarWidth float32
	lastChrome       float32
	// controlsWidth is how far the window's controls reach in from its
	// right edge, and controlsFoot down from its top, with the room around
	// them, as last laid out.
	controlsWidth float32
	controlsFoot  float32
	// listDock is how far the place's title has gone from the panes'
	// title line to the top of the sheet list, as it does where the library
	// is closed; dockTarget is where it goes, held while the list is shut.
	listDock   float32
	dockTarget float32
	// flight is whether that title is on its way this frame (takeFlight).
	flight bool
	// mode is how the notebook is laid out (Mode), and canvas the canvas's
	// state, kept as it was left.
	mode   Mode
	canvas canvasState
}

func New() *State {
	return &State{
		Appearance:   settings.DefaultPresentation(),
		Settings:     theme.DefaultSettings(),
		WindowActive: true,
		nav:          navigation{expanded: map[string]bool{}},
		list:         newBrowser(Place{}),
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
