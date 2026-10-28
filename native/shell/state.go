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
	Bold   bool
	Italic bool
	CJK    bool
}

type State struct {
	Appearance settings.PresentationState
	Settings   theme.Settings
	Pages      []Page
	ActivePage string
	Status     string
	Statistics string
	PageQuery  string
	Toolbar    ToolbarState
	CanUndo    bool
	CanRedo    bool
	TabsScroll ui.ScrollState

	Content       func(c *ui.Context)
	SelectPage    func(id string)
	NewPage       func()
	ClosePage     func()
	ClosePageID   func(id string)
	RenamePage    func(id string)
	OpenSettings  func()
	OpenCommands  func()
	FindPage      func()
	ToggleSidebar func()
	MoveTab       func(id, delta int)
	FormatItalic  func(bool)
	FormatBold    func(bool)
	Undo          func()
	Redo          func()
	ToggleCJK     func(bool)
	Typewriter    func(bool)
	Zen           func(bool)

	lastActive string
}

func New() *State {
	return &State{
		Appearance: settings.DefaultPresentation(),
		Settings:   theme.DefaultSettings(),
		Pages:      []Page{{ID: "1", Title: "Untitled"}},
		ActivePage: "1",
	}
}
