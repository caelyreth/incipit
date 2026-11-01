// Package settings composes MyGo settings windows from declarative product
// data. Validation and policy publication remain in the application layer.
package settings

import (
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type State struct {
	Category appsettings.Category
	Query    string
	Data     appsettings.Data
	Theme    theme.Settings
	// Colors carries theme changes; nil shows Theme as it is.
	Colors *theme.Transition
	Apply  func(appsettings.Change)
	// Recording hears when a shortcut recording starts and ends, so the
	// host can stand its menu's keys down meanwhile. Reserved names what
	// keeps a key for itself, as a menu item of the system's, which no
	// action can be given.
	Recording    func(bool)
	Reserved     func(kernel.BindingTarget) (string, bool)
	Animate      bool
	WindowActive bool

	expanded      map[appsettings.Category]bool
	contentScroll ui.ScrollState
	navScroll     ui.ScrollState
	contentOver   scrollfx.Overscroll
	navOver       scrollfx.Overscroll

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
	keyReturn   string
	keyQuiet    string
	keyConflict *keyConflict
	// recorders is what each open or closing recorder shows, by host.
	recorders map[string]recorderView
	// reported is the recording last told to Recording.
	reported bool
}

type keyConflict struct {
	Command kernel.CommandID
	Target  kernel.BindingTarget
	Owner   appsettings.Shortcut
	// Reserved marks a key the system keeps, which no action can take.
	Reserved bool
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
	return s != nil && components.MotionEnabled(c, s.Animate, s.WindowActive)
}
