package settings

import (
	"fmt"
	"maps"
	"slices"

	"github.com/caelyreth/incipit/internal/app/manifest"
	"github.com/caelyreth/incipit/internal/app/phrases"
	"github.com/caelyreth/incipit/internal/editor/policy"
)

// The Editor page is the program's catalog: a card per policy, its switch
// and the options it reads while it builds, valued as the person chose.
// A change compiles the candidate values first, so the program only ever
// changes to one that compiles.

// Policy is one policy's card.
type Policy struct {
	ID          string
	Title       string
	Description string
	// Switch is the enable option's key, empty for a policy that cannot
	// be turned off.
	Switch    string
	Enabled   bool
	Options   []Option
	Shortcuts []Shortcut
	// Showcase is the policy's demonstration, when it declares one.
	Showcase *policy.Showcase
}

// Option is one option of a policy.
type Option struct {
	Key         string
	Kind        policy.OptionKind
	Label       string
	Description string
	Value       any
	Choices     []Choice
	Min, Max    int
	// Applies is false while an option it requires has another value.
	Applies bool
}

// Choice is one value of an enum option.
type Choice struct {
	Value any
	Label string
}

// ErrInvalidOption rejects a value outside its option's domain.
var ErrInvalidOption = fmt.Errorf("settings: invalid option value")

// SetOption gives a policy option value; a policy's switch is its enabled
// option (policy.EnabledKey).
func SetOption(key string, value any) Change {
	return Change{Kind: ChangePolicy, Option: key, OptionValue: value}
}

func mustCompile(values policy.Values) *policy.Program {
	prog, err := manifest.Compile(values, false)
	if err != nil {
		panic(err)
	}
	return prog
}

// PrintProgram is the program a printed page is read through: the one
// chosen, with the marks' signs not drawn, as a page is read rather than
// written.
func (s *State) PrintProgram() (*policy.Program, error) {
	values := maps.Clone(s.Policy)
	if values == nil {
		values = policy.Values{}
	}
	values["notation.reveal"] = "hidden"
	return manifest.Compile(values, false)
}

func (s *State) applyPolicy(change Change) (Outcome, error) {
	info, ok := s.option(change.Option)
	if !ok || !valid(info, change.OptionValue) {
		return Outcome{}, ErrInvalidOption
	}
	candidate := maps.Clone(s.Policy)
	if candidate == nil {
		candidate = policy.Values{}
	}
	if change.OptionValue == info.Default {
		delete(candidate, change.Option)
	} else {
		candidate[change.Option] = change.OptionValue
	}
	if maps.Equal(candidate, s.Policy) {
		return Outcome{Applied: true}, nil
	}
	prog, err := manifest.Compile(candidate, false)
	if err != nil {
		return Outcome{}, err
	}
	s.Policy, s.Program = candidate, prog
	return Outcome{Applied: true, PolicyChanged: true}, nil
}

func (s *State) option(key string) (policy.OptionInfo, bool) {
	for _, p := range s.Program.Catalog().Policies {
		for _, o := range p.Options {
			if o.Key == key {
				return o, true
			}
		}
	}
	return policy.OptionInfo{}, false
}

func valid(info policy.OptionInfo, value any) bool {
	switch info.Kind {
	case policy.OptionBool:
		_, ok := value.(bool)
		return ok
	case policy.OptionInt:
		v, ok := value.(int)
		return ok && v >= info.Min && v <= info.Max
	case policy.OptionEnum:
		return slices.Contains(info.Choices, value)
	}
	return false
}

// policies are the catalog's cards, in manifest order.
func (s State) policies(shortcuts []Shortcut) []Policy {
	cards := []Policy{}
	for _, info := range s.Program.Catalog().Policies {
		if info.Unlisted {
			continue
		}
		card := Policy{ID: info.ID, Title: phrases.Text(info.Label), Description: phrases.Text(info.Description), Enabled: info.Enabled,
			Showcase: info.Showcase}
		for _, o := range info.Options {
			if o.Key == policy.EnabledKey(info.ID) {
				if info.ID != policy.CoreID {
					card.Switch = o.Key
				}
				continue
			}
			card.Options = append(card.Options, s.optionOf(info.ID, o))
		}
		for _, sc := range shortcuts {
			if sc.Policy == info.ID && info.ID != policy.CoreID {
				card.Shortcuts = append(card.Shortcuts, sc)
			}
		}
		cards = append(cards, card)
	}
	return cards
}

func (s State) optionOf(policyID string, o policy.OptionInfo) Option {
	opt := Option{Key: o.Key, Kind: o.Kind, Label: phrases.Text(o.Label), Description: phrases.Text(o.Description),
		Value: s.value(o), Min: o.Min, Max: o.Max, Applies: true}
	for i, v := range o.Choices {
		label := fmt.Sprint(v)
		if i < len(o.ChoiceLabels) {
			label = phrases.Text(o.ChoiceLabels[i])
		}
		opt.Choices = append(opt.Choices, Choice{Value: v, Label: label})
	}
	for _, r := range o.Requires {
		info, ok := s.option(policyID + "." + r.Option)
		opt.Applies = opt.Applies && ok && s.value(info) == r.Value
	}
	return opt
}

func (s State) value(o policy.OptionInfo) any {
	if v, ok := s.Policy[o.Key]; ok {
		return v
	}
	return o.Default
}
