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

type State struct {
	Appearance settings.PresentationState
	Settings   theme.Settings
	Pages      []Page
	ActivePage string
	Status     string
	Statistics string

	Content       func(c *ui.Context)
	SelectPage    func(id string)
	NewPage       func()
	ClosePage     func()
	OpenSettings  func()
	OpenCommands  func()
	FindPage      func()
	ToggleSidebar func()
	MoveTab       func(id, delta int)
	FormatItalic  func()
	FormatBold    func()
	Undo          func()
	Redo          func()
	Background    func()
	CenterText    func()
	ClearFormat   func()
	CanUndo       bool
	CanRedo       bool
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
