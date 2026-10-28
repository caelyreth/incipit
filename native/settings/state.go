// Package settings composes MyGo settings windows from declarative product
// data. Validation and policy publication remain in the application layer.
package settings

import (
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type State struct {
	Category appsettings.Category
	Query    string
	Data     appsettings.Data
	Theme    theme.Settings
	Apply    func(appsettings.Change)
	Close    func()

	expanded      map[appsettings.Category]bool
	contentScroll ui.ScrollState
}

func New() *State {
	expanded := make(map[appsettings.Category]bool, len(appsettings.AllCategories))
	for _, category := range appsettings.AllCategories {
		expanded[category] = true
	}
	return &State{
		Category: appsettings.Editor,
		Theme:    theme.DefaultSettings(),
		expanded: expanded,
	}
}

func (s *State) apply(change appsettings.Change) {
	if s.Apply != nil {
		s.Apply(change)
	}
}
