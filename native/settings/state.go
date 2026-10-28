// Package settings composes MyGo settings windows from declarative product
// data. Validation and policy publication remain in the application layer.
package settings

import (
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/theme"
)

type State struct {
	Category appsettings.Category
	Query    string
	Data     appsettings.Data
	Theme    theme.Settings
	Apply    func(appsettings.Change)
	Close    func()
}

func New() *State {
	return &State{
		Category: appsettings.Editor,
		Theme:    theme.DefaultSettings(),
	}
}

func (s *State) apply(change appsettings.Change) {
	if s.Apply != nil {
		s.Apply(change)
	}
}
