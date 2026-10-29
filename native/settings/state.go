// Package settings composes MyGo settings windows from declarative product
// data. Validation and policy publication remain in the application layer.
package settings

import (
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type State struct {
	Category appsettings.Category
	Query    string
	Data     appsettings.Data
	Theme    theme.Settings
	// Colors carries theme changes; nil shows Theme as it is.
	Colors       *theme.Transition
	Apply        func(appsettings.Change)
	Animate      bool
	WindowActive bool

	expanded      map[appsettings.Category]bool
	contentScroll ui.ScrollState

	// anchors are the current page's section tops in content coordinates,
	// in page order, as the previous frame laid them out. pending is a
	// section the navigation asked to show once its anchor is known, and
	// target the one a scroll is gliding to.
	anchors    []anchor
	building   []anchor
	contentTop float32
	pending    string
	target     string
	lastActive string

	// The keymap page's own search text or recorded chord. Then, on either
	// page, the command whose shortcut is being recorded and the row or
	// sentence it was started from, and a recorded shortcut another command
	// already holds, awaiting a decision there.
	keyQuery    string
	keyFilter   *kernel.BindingTarget
	keySearch   bool
	keyEditing  kernel.CommandID
	keyHost     string
	keyConflict *keyConflict
	// recorders is what each open or closing recorder shows, by host.
	recorders map[string]recorderView
}

type keyConflict struct {
	Command kernel.CommandID
	Target  kernel.BindingTarget
	Owner   appsettings.Shortcut
	Host    string
}

type anchor struct {
	Label string
	Top   float32
}

func New() *State {
	expanded := make(map[appsettings.Category]bool, len(appsettings.AllCategories))
	for _, category := range appsettings.AllCategories {
		expanded[category] = category == appsettings.Editor
	}
	return &State{
		Category:     appsettings.Editor,
		Theme:        theme.DefaultSettings(),
		expanded:     expanded,
		Animate:      true,
		WindowActive: true,
	}
}

func (s *State) apply(change appsettings.Change) {
	if s.Apply != nil {
		s.Apply(change)
	}
}

func (s *State) SetWindowActive(active bool) {
	if s != nil {
		s.WindowActive = active
	}
}

func (s *State) motion(c *ui.Context) bool {
	return s != nil && s.Animate && s.WindowActive && !c.Preferences().ReduceMotion
}
