// Package shell composes Neverland's native window chrome around a custom
// document surface.
package shell

import (
	"github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

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
}

type State struct {
	Appearance   settings.PresentationState
	Settings     theme.Settings
	Pages        []Page
	ActivePage   string
	Status       string
	Statistics   string
	PageQuery    string
	Toolbar      ToolbarState
	CanUndo      bool
	CanRedo      bool
	TabsScroll   ui.ScrollState
	WindowActive bool

	Content      func(c *ui.Context)
	SelectPage   func(id string)
	NewPage      func()
	ClosePage    func()
	ClosePageID  func(id string)
	RenamePage   func(id string)
	OpenSettings func()
	OpenCommands func()
	FindPage     func()
	MoveTab      func(id, delta int)
	FormatItalic func(bool)
	FormatBold   func(bool)
	Undo         func()
	Redo         func()
	ToggleCJK    func(bool)
	Typewriter   func(bool)
	Zen          func(bool)

	SidebarOverride  *bool
	lastActive       string
	lastWidth        float32
	lastUIFontSize   float32
	lastSidebarWidth float32
	lastChrome       float32
}

func New() *State {
	return &State{
		Appearance:   settings.DefaultPresentation(),
		Settings:     theme.DefaultSettings(),
		Pages:        []Page{{ID: "1", Title: "Untitled"}},
		ActivePage:   "1",
		WindowActive: true,
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
