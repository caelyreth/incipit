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

// Formatting is the toolbar's current presentation state. Apps can update it
// from the active document before building the next frame.
type Formatting struct {
	Bold       bool
	Italic     bool
	Background bool
}

type State struct {
	Appearance settings.PresentationState
	Settings   theme.Settings
	Pages      []Page
	ActivePage string
	Status     string
	Statistics string
	PageQuery  string
	Formatting Formatting
	CanUndo    bool
	CanRedo    bool

	Content       func(c *ui.Context)
	SelectPage    func(id string)
	NewPage       func()
	ClosePage     func()
	OpenSettings  func()
	OpenCommands  func()
	FindPage      func()
	ToggleSidebar func()
	MoveTab       func(id, delta int)
	FormatItalic  func(bool)
	FormatBold    func(bool)
	Undo          func()
	Redo          func()
	Background    func(bool)
	CenterText    func()
	ClearFormat   func()

	FocusDocument func()
}

func New() *State {
	return &State{
		Appearance: settings.DefaultPresentation(),
		Settings:   theme.DefaultSettings(),
		Pages:      []Page{{ID: "1", Title: "Untitled"}},
		ActivePage: "1",
		Status:     "Ready",
	}
}
