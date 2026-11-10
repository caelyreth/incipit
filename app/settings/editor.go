package settings

import (
	"fmt"
	"maps"
	"slices"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/policy"
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
		text := policyTexts[info.ID]
		card := Policy{ID: info.ID, Title: text.title, Description: text.description, Enabled: info.Enabled}
		if card.Title == "" {
			card.Title = info.ID
		}
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
	opt := Option{Key: o.Key, Kind: o.Kind, Label: phraseText(o.Label), Description: phraseText(o.Description),
		Value: s.value(o), Min: o.Min, Max: o.Max, Applies: true}
	for i, v := range o.Choices {
		label := fmt.Sprint(v)
		if i < len(o.ChoiceLabels) {
			label = phraseText(o.ChoiceLabels[i])
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

// commandPolicy is the policy declaring an editing command.
func (s State) commandPolicy(id keymap.CommandID) string {
	for _, c := range s.Program.Catalog().Commands {
		if string(c.ID) == string(id) {
			return c.Policy
		}
	}
	return ""
}

func phraseText(key string) string {
	if text, ok := optionTexts[key]; ok {
		return text
	}
	return key
}

type policyText struct{ title, description string }

// policyTexts title and describe each policy's card. A key named in braces
// is the command's live key.
var policyTexts = map[string]policyText{
	policy.CoreID: {"Text", "Every keystroke is an edit of the page: {core.break} starts a new block, " +
		"{core.softBreak} breaks the line inside one."},
	"notation":   {"Notation", "How the page spells its marks and markers as you write them."},
	"markdown":   {"Markdown", "Type markdown and the page takes the shape it spells."},
	"lists":      {"Lists", "{core.nest} nests an item, {core.outdent} steps it out; {core.break} in an empty item leaves the list."},
	"pairs":      {"Pairs", "Brackets and quotes close themselves as you open them; deleting into a pair selects it first."},
	"navigation": {"Navigation", "{core.leave} leaves the pair or scope you are in, {core.return} goes back into it."},
	"guides":     {"Guides", "Show a key where it would act: leaving a scope, or going back."},
	"history":    {"History", "Undo and redo step through your edits, a burst of typing at a time."},
}

// optionTexts are the catalog's label and description keys in words.
var optionTexts = map[string]string{
	"settings.markdown.asterisks.description":   "*soft* and **firm** become emphasis and strong.",
	"settings.markdown.underscores.description": "_soft_ and __firm__ do the same.",
	"settings.markdown.headings.description":    "# at a line's start makes a heading, one level per #.",
	"settings.markdown.lists.description":       "- or 1. at a line's start starts a list.",
	"settings.markdown.quotes.description":      "> at a line's start starts a quote.",
	"settings.markdown.code.description":        "`code` marks code; ``` and Enter opens a code block.",
	"settings.markdown.math.description":        "$x$ writes a formula; $$ alone opens a math block.",
	"settings.markdown.strike.description":      "~~text~~ strikes text through.",
	"settings.markdown.rules.description":       "--- and Enter draws a rule.",
	"settings.markdown.tables.description":      "A pipe row and its delimiter row make a table.",
	"settings.notation.reveal.description":      "Whether marks show their signs, as *this*, or only their style.",
	"settings.notation.layout.description":      "Whether signs take room in the line or sit over the text.",
	"settings.notation.underscores.description": "Emphasis and strong show _ and __ instead of * and **.",
	"settings.pairs.autoClose.description":      "Typing an opener types its closer, unless a word follows.",
	"settings.pairs.overtype.description":       "Typing a closer the page typed for you moves past it.",
	"settings.pairs.quotes.description":         "Straight double quotes pair like brackets; after a word one closes.",
	"settings.pairs.mates.description":          "Backspace or Delete into a pair selects it whole; the next press deletes both sides together.",
	"settings.history.typingWindow.description": "Typing within this pause of the last keystroke undoes together.",
	"settings.markdown.asterisks":               "Asterisks for emphasis",
	"settings.markdown.underscores":             "Underscores for emphasis",
	"settings.markdown.headings":                "Headings",
	"settings.markdown.lists":                   "Lists",
	"settings.markdown.quotes":                  "Quotes",
	"settings.markdown.code":                    "Code",
	"settings.markdown.math":                    "Math",
	"settings.markdown.strike":                  "Strikethrough",
	"settings.markdown.rules":                   "Rules",
	"settings.markdown.tables":                  "Tables",
	"settings.notation.reveal":                  "Mark notation",
	"settings.notation.reveal.shown":            "Shown",
	"settings.notation.reveal.hidden":           "Hidden",
	"settings.notation.layout":                  "Notation layout",
	"settings.notation.layout.room":             "In line",
	"settings.notation.layout.overlay":          "Over the text",
	"settings.notation.underscores":             "Spell emphasis with underscores",
	"settings.pairs.autoClose":                  "Close brackets and quotes",
	"settings.pairs.overtype":                   "Step over a closer typed ahead",
	"settings.pairs.quotes":                     "Pair straight quotes",
	"settings.pairs.mates":                      "Select a pair before deleting it",
	"settings.history.typingWindow":             "Typing burst (ms)",
}
