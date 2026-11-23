// Package settings draws the app's settings, from declarative settings
// data, as sections the field shows as cards (view.go). Validation stays
// in the application layer.
package settings

import (
	"github.com/caelyreth/neverland/internal/app/keymap"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

type State struct {
	// Query is the words the sections are found by, as last drawn.
	Query string
	Data  appsettings.Data
	Theme theme.Settings
	Apply func(appsettings.Change)
	// Recording hears when a shortcut recording starts and ends, so the
	// host can stand its menu's keys down meanwhile. Reserved names what
	// keeps a key for itself, as a menu item of the system's, which no
	// action can be given.
	Recording func(bool)
	Reserved  func(keymap.Chord) (string, bool)
	// Find searches the settings for words, as a card that names another
	// asks to show it.
	Find    func(words string)
	Animate bool
	// Fonts are the downloadable faces for Chinese and Japanese, and Font
	// does what the Typeface section asks of one.
	Fonts []FontRow
	Font  func(id string, act FontAct)

	// Languages are those the app is written in, its own first; Language
	// is the one chosen, "" while the app follows the system's, and
	// SetLanguage chooses one, "" for the system's.
	Languages   []string
	Language    string
	SetLanguage func(tag string)

	// The chord the keymap is searched by, as pressed. Then, in any
	// section, the command whose shortcut is being recorded and the row or
	// sentence it was started from, and a recorded shortcut another command
	// already holds, awaiting a decision there.
	keyFilter   *keymap.Chord
	keySearch   bool
	keyEditing  keymap.CommandID
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
	Command keymap.CommandID
	Target  keymap.Chord
	Owner   appsettings.Shortcut
	// Reserved marks a key the system keeps, which no action can take.
	Reserved bool
}

func New() *State {
	return &State{Theme: theme.DefaultSettings(), Animate: true}
}

func (s *State) apply(change appsettings.Change) {
	if s.Apply != nil {
		s.Apply(change)
	}
}

func (s *State) motion(c *ui.Context) bool {
	return s != nil && components.MotionEnabled(c, s.Animate)
}
