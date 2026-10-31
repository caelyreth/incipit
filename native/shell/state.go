// Package shell composes Neverland's native window chrome around a custom
// document surface.
package shell

import (
	"github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Page is an open tab: the sheet it shows.
type Page struct {
	ID    string
	Title string
}

// ToolbarState is the editor-owned state rendered by the floating writing
// toolbar. Presentation modes live in Appearance; CJK lives in the policy
// configuration.
type ToolbarState struct {
	Bold          bool
	Italic        bool
	BoldEnabled   bool
	ItalicEnabled bool
	CJK           bool
	Bindings      ToolbarBindings
}

// ToolbarBindings carries the current shortcut text for action tooltips.
type ToolbarBindings struct {
	Bold   string
	Italic string
	Undo   string
	Redo   string
}

type State struct {
	Appearance settings.PresentationState
	Settings   theme.Settings
	// Colors carries theme changes; nil shows Settings as they are.
	Colors *theme.Transition
	// Tabs are the open tabs, in order; ActivePage is the one in front.
	Tabs       []Page
	ActivePage string
	// Library is what the sidebar shows and files; nil shows none.
	Library           Library
	Status            string
	Statistics        string
	PageQuery         string
	FindPageRequested bool
	Toolbar           ToolbarState
	CanUndo           bool
	CanRedo           bool
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
	// MoveTab puts a tab at index among the tabs.
	MoveTab func(id string, index int)
	// CloseOtherTabs closes every tab but one; CloseTabsAfter those after
	// it.
	CloseOtherTabs func(id string)
	CloseTabsAfter func(id string)
	FormatItalic   func(bool)
	FormatBold     func(bool)
	Undo           func()
	Redo           func()
	ToggleCJK      func(bool)
	Typewriter     func(bool)
	Zen            func(bool)

	// SidebarOverride shows or hides the library, SheetListOverride the
	// sheet list, against what the window's width would choose.
	SidebarOverride   *bool
	SheetListOverride *bool
	nav               navigation
	lastActive        string
	lastWidth         float32
	lastUIFontSize    float32
	lastSidebarWidth  float32
	lastChrome        float32
	// listDock is how far the place's title has gone from the title bar
	// to the top of the sheet list, as it does where the library is closed.
	listDock float32
	// dockTarget is where listDock goes, held while the list is shut.
	dockTarget float32
	// flight is whether that title is on its way this frame (takeFlight).
	flight bool
}

func New() *State {
	return &State{
		Appearance:   settings.DefaultPresentation(),
		Settings:     theme.DefaultSettings(),
		WindowActive: true,
		nav:          navigation{expanded: map[string]bool{}},
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
}

// ColumnMargin is the room beside the document column: the gutter and half
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
	return s != nil && s.Appearance.Motion && s.WindowActive &&
		!c.Preferences().ReduceMotion
}
