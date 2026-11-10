package settings

import (
	"errors"
	"maps"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/policy"
)

var (
	// ErrInvalidChange rejects a partially populated or unknown typed change.
	ErrInvalidChange = errors.New("settings: invalid change")
	// ErrInvalidAppearance rejects an out-of-range theme choice.
	ErrInvalidAppearance = errors.New("settings: invalid appearance change")
)

// State is every application setting. Encode and Decode keep it beside the
// notebook; the workspace keeps each page's selection separately.
type State struct {
	Presentation PresentationState
	Appearance   AppearanceSettings
	Keymap       *keymap.Keymap
	// Policy are the person's policy values, and Program the program they
	// compile to.
	Policy  policy.Values
	Program *policy.Program
}

// DefaultState matches Neverland's initial settings.
func DefaultState() State {
	return State{
		Presentation: DefaultPresentation(),
		Appearance:   DefaultAppearance(),
		Keymap:       keymap.Default(),
		Program:      mustCompile(nil),
	}
}

// Outcome reports what a change did. A false Applied value always means
// every piece of state remained unchanged.
type Outcome struct {
	Applied       bool
	KeymapChanged bool
	PolicyChanged bool
}

// Apply validates and commits one change atomically.
func (s *State) Apply(change Change) (Outcome, error) {
	if s == nil {
		return Outcome{}, ErrInvalidChange
	}
	switch change.Kind {
	case ChangePresentation:
		candidate := s.Presentation
		if !candidate.Set(change.Presentation, change.Value) {
			return Outcome{}, ErrInvalidChange
		}
		s.Presentation = candidate
		return Outcome{Applied: true}, nil
	case ChangeAppearance:
		candidate := s.Appearance
		if !candidate.Apply(change.Appearance) {
			return Outcome{}, ErrInvalidAppearance
		}
		s.Appearance = candidate
		return Outcome{Applied: true}, nil
	case ChangeBinding, ChangeUnbind, ChangeResetBinding:
		return s.applyKeymap(change)
	case ChangePolicy:
		return s.applyPolicy(change)
	default:
		return Outcome{}, ErrInvalidChange
	}
}

func (s *State) applyKeymap(change Change) (Outcome, error) {
	if change.Command == "" {
		return Outcome{}, ErrInvalidChange
	}
	current := s.Keymap
	if current == nil {
		current = keymap.Default()
	}
	var (
		next *keymap.Keymap
		err  error
	)
	switch change.Kind {
	case ChangeBinding:
		next, err = current.Remap(change.Command, change.Binding, change.Displaced)
	case ChangeUnbind:
		next, err = current.Unbind(change.Command)
	default:
		next, err = current.Reset(change.Command)
	}
	if err != nil {
		return Outcome{}, err
	}
	s.Keymap = next
	return Outcome{Applied: true, KeymapChanged: !current.Overrides().Equal(next.Overrides())}, nil
}

// Data builds the declarative snapshot of this state.
func (s State) Data() Data {
	keys := s.Keymap
	if keys == nil {
		keys = keymap.Default()
	}
	data := Data{
		AppearanceValues: s.Appearance.Normalized(),
		Appearance:       appearanceToggles(s.Presentation),
	}
	commands := keymap.AllCommands()
	data.Shortcuts = make([]Shortcut, 0, len(commands))
	for _, scope := range []keymap.Scope{keymap.ScopeEditing, keymap.ScopePages, keymap.ScopeWindow, keymap.ScopeApp} {
		for _, command := range commands {
			if command.Scope != scope {
				continue
			}
			shortcut := Shortcut{
				Command:    command.ID,
				Label:      command.Label,
				Scope:      scope,
				Group:      scope.Title(),
				Policy:     s.commandPolicy(command.ID),
				Overridden: keys.Overridden(command.ID),
			}
			if chord, ok := keys.ChordFor(command.ID); ok {
				shortcut.Bindings = []keymap.Chord{chord}
			}
			data.Shortcuts = append(data.Shortcuts, shortcut)
		}
	}
	data.Policies = s.policies(data.Shortcuts)
	data.PolicyValues = maps.Clone(s.Policy)
	return data
}

func appearanceToggles(presentation PresentationState) []ToggleSetting {
	return []ToggleSetting{
		{
			Mode:        PresentationMotion,
			Label:       "Animations",
			Description: "Animate panels, lists and theme changes. System reduced motion takes precedence.",
			Value:       presentation.Motion,
			Selector:    "appearance-motion",
		},
		{
			Mode:        PresentationTypewriter,
			Label:       "Typewriter mode",
			Description: "Keep the writing line near the center of the page.",
			Value:       presentation.Typewriter,
			Selector:    "appearance-typewriter",
		},
		{
			Mode:        PresentationZen,
			Label:       "Zen mode",
			Description: "Hide the surrounding notebook and window chrome.",
			Value:       presentation.Zen,
			Selector:    "appearance-zen",
		},
	}
}
