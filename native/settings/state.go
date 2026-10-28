// Package settings composes MyGo settings windows from declarative product
// data. Validation and policy publication remain in the application layer.
package settings

import (
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type State struct {
	Category     appsettings.Category
	Query        string
	Data         appsettings.Data
	Theme        theme.Settings
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
