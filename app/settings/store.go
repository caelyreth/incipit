package settings

import (
	"errors"

	"github.com/caelyreth/neverland/engine/kernel"
)

var (
	// ErrPageRequired rejects editor settings while every tab is closed.
	ErrPageRequired = errors.New("settings: open a page before changing editor settings")
	// ErrInvalidChange rejects a partially populated or unknown typed change.
	ErrInvalidChange = errors.New("settings: invalid change")
	// ErrInvalidAppearance rejects an out-of-range theme choice.
	ErrInvalidAppearance = errors.New("settings: invalid appearance change")
)

// Store owns the app-wide validated policy source. It has no native UI
// dependency. Adopt must be atomic and preserve the previous configuration on
// failure.
type Store interface {
	Snapshot() PolicySnapshot
	PolicyCatalog() kernel.PolicyCatalog
	Configuration() kernel.PolicyConfiguration
	PreparedConfiguration() *kernel.PreparedPolicyConfiguration
	Adopt(kernel.PolicyConfiguration) error
}

// PolicySnapshot is one owned generation of the settings policy source.
type PolicySnapshot struct {
	Catalog       kernel.PolicyCatalog
	Configuration kernel.PolicyConfiguration
	Prepared      *kernel.PreparedPolicyConfiguration
}

// Session exposes only the settings-relevant read surface of the mounted page.
type Session interface {
	Page() (int, string, bool)
}

// State is the reducer-owned settings state shared by the settings window.
type State struct {
	Presentation PresentationState
	Appearance   AppearanceSettings
}

// DefaultState matches Neverland's initial product settings.
func DefaultState() State {
	return State{
		Presentation: DefaultPresentation(),
		Appearance:   DefaultAppearance(),
	}
}

// Data builds a declarative snapshot from this state.
func (s State) Data(store Store, session Session) Data {
	return BuildData(store, session, s.Presentation, s.Appearance)
}

// Outcome reports the applied change and whether a page is required. A false
// Applied value always means every piece of state remained unchanged.
type Outcome struct {
	Applied       bool
	Changed       bool
	PolicyChanged bool
	Configuration kernel.PolicyConfiguration
}

// Apply validates and commits one change atomically. Policy changes are staged
// against the store's current source and installed only if the store accepts the
// complete candidate. Presentation and appearance are local product state.
func (s *State) Apply(
	change Change,
	store Store,
	session Session,
) (Outcome, error) {
	if s == nil {
		return Outcome{}, ErrInvalidChange
	}
	applied := *s
	outcome, err := applied.apply(change, store, session)
	if err != nil {
		return Outcome{}, err
	}
	*s = applied
	return outcome, nil
}

func (s *State) apply(
	change Change,
	store Store,
	session Session,
) (Outcome, error) {
	switch change.Kind {
	case ChangeBoolean:
		switch change.Boolean.Kind {
		case BooleanPresentation:
			if !s.Presentation.Set(change.Boolean.Presentation, change.Value) {
				return Outcome{}, ErrInvalidChange
			}
			return Outcome{
				Applied: true,
				Changed: true,
			}, nil
		case BooleanPolicy, BooleanOption:
			return s.applyPolicy(store, session, func(configuration *kernel.PolicyConfiguration) bool {
				if change.Boolean.Kind == BooleanPolicy {
					configuration.SetEnabled(change.Boolean.Policy, change.Value)
				} else {
					configuration.SetSetting(
						change.Boolean.Setting,
						kernel.BoolSetting(change.Value),
					)
				}
				return true
			})
		default:
			return Outcome{}, ErrInvalidChange
		}
	case ChangeBinding:
		if change.Command == "" {
			return Outcome{}, ErrInvalidChange
		}
		return s.applyPolicy(store, session, func(configuration *kernel.PolicyConfiguration) bool {
			if change.Displaced != "" && change.Displaced != change.Command {
				configuration.Unbind(change.Displaced)
			}
			// Choosing a command's own default again is no customization.
			defaults := store.PolicyCatalog().DefaultBindings(change.Command)
			if len(defaults) == 1 && defaults[0] == change.Binding {
				configuration.ResetRemap(change.Command)
			} else {
				configuration.Remap(change.Command, change.Binding)
			}
			return true
		})
	case ChangeUnbind:
		if change.Command == "" {
			return Outcome{}, ErrInvalidChange
		}
		return s.applyPolicy(store, session, func(configuration *kernel.PolicyConfiguration) bool {
			configuration.Unbind(change.Command)
			return true
		})
	case ChangeResetBinding:
		if change.Command == "" {
			return Outcome{}, ErrInvalidChange
		}
		return s.applyPolicy(store, session, func(configuration *kernel.PolicyConfiguration) bool {
			configuration.ResetRemap(change.Command)
			return true
		})
	case ChangeAppearance:
		candidate := s.Appearance
		if !candidate.Apply(change.Appearance) {
			return Outcome{}, ErrInvalidAppearance
		}
		s.Appearance = candidate
		return Outcome{
			Applied: true,
			Changed: true,
		}, nil
	default:
		return Outcome{}, ErrInvalidChange
	}
}

func (s *State) applyPolicy(
	store Store,
	session Session,
	mutate func(*kernel.PolicyConfiguration) bool,
) (Outcome, error) {
	if store == nil || session == nil {
		return Outcome{}, ErrPageRequired
	}
	page, _, ok := session.Page()
	if !ok || page < 0 {
		return Outcome{}, ErrPageRequired
	}
	current := store.Configuration()
	candidate := current.Clone()
	if !mutate(&candidate) {
		return Outcome{}, ErrInvalidChange
	}
	if err := store.Adopt(candidate); err != nil {
		return Outcome{}, err
	}
	return Outcome{
		Applied:       true,
		Changed:       !current.Equal(candidate),
		PolicyChanged: !current.Equal(candidate),
		Configuration: candidate.Clone(),
	}, nil
}
