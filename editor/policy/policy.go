// Package policy is the policy framework: declarations, options and the
// catalog, analyzers and the fact store, annotations, rules, the compiler
// and dispatch.
//
// A policy is the unit of behaviour, ownership and configuration. It is
// built by a function of its option values: the values read while
// building decide what exists. Policies never import each other; roles in
// the vocabulary connect them, and only the manifest orders them.
package policy

import (
	"fmt"
	"slices"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Policy is a declared policy.
type Policy struct {
	id    string
	build func(*P)
	meta  optionMeta
}

// Define declares a policy. Its id is persisted: it is the prefix of its
// option keys. Label and Description give its card's title and text keys,
// as they do an option's.
func Define(id string, build func(p *P), settings ...Setting) *Policy {
	pol := &Policy{id: id, build: build}
	for _, s := range settings {
		s(&pol.meta)
	}
	return pol
}

// ID is the policy's stable id.
func (p *Policy) ID() string { return p.id }

// P is a policy being built. Each declaration it receives exists in the
// compiled program only because the build made it.
type P struct {
	policy      *Policy
	values      Values
	options     []optionInfo
	facts       []*factDecl
	analyzers   []*analyzer
	annotations []annotationDecl
	decorations []Decoration
	rules       []*rule
	rewrites    []*rewrite
	observers   []*observer
	commands    []Command
	showcase    *Showcase
	paints      []Paint
	guides      []Guide
	grouping    func(prev, next *doc.Entry) bool
	errs        []error
}

// ID is the id of the policy being built.
func (p *P) ID() string { return p.policy.id }

func (p *P) fail(format string, args ...any) {
	p.errs = append(p.errs, fmt.Errorf("policy %s: %s", p.policy.id, fmt.Sprintf(format, args...)))
}

// --- options ----------------------------------------------------------

// Values are option values by key, "policy.option". The settings layer
// validates domains, so a policy never receives a value out of its domain.
type Values map[string]any

// OptionKind is the type of an option.
type OptionKind uint8

const (
	OptionBool OptionKind = iota + 1
	OptionEnum
	OptionInt
	OptionList
)

// Choice is one value of an enum or list domain.
type Choice[T comparable] struct {
	Value T
	Label string
}

// Setting refines an option's declaration.
type Setting func(*optionMeta)

type optionMeta struct {
	label, description, group string
	requires                  []Requirement
}

// Requirement makes an option apply only while another has a value.
type Requirement struct {
	Option string
	Value  any
}

// Label is the option's label key.
func Label(key string) Setting { return func(m *optionMeta) { m.label = key } }

// Description is the option's description key.
func Description(key string) Setting { return func(m *optionMeta) { m.description = key } }

// Group places the option in a settings group.
func Group(key string) Setting { return func(m *optionMeta) { m.group = key } }

// Requires makes the option apply only while option (a key within the
// same policy) has value.
func Requires(option string, value any) Setting {
	return func(m *optionMeta) { m.requires = append(m.requires, Requirement{Option: option, Value: value}) }
}

// Opt is a typed option.
type Opt[T comparable] struct {
	id      string
	def     T
	kind    OptionKind
	choices []Choice[T]
	lo, hi  int
	meta    optionMeta
}

// ListOpt is an option holding a subset of a closed domain.
type ListOpt[T comparable] struct {
	id     string
	def    []T
	domain []Choice[T]
	meta   optionMeta
}

func apply(settings []Setting) optionMeta {
	var m optionMeta
	for _, s := range settings {
		s(&m)
	}
	return m
}

// Bool declares a bool option.
func Bool(id string, def bool, settings ...Setting) *Opt[bool] {
	return &Opt[bool]{id: id, def: def, kind: OptionBool, meta: apply(settings)}
}

// Enum declares an option with one of choices.
func Enum[T comparable](id string, def T, choices []Choice[T], settings ...Setting) *Opt[T] {
	return &Opt[T]{id: id, def: def, kind: OptionEnum, choices: choices, meta: apply(settings)}
}

// Int declares an option in [lo, hi].
func Int(id string, def, lo, hi int, settings ...Setting) *Opt[int] {
	return &Opt[int]{id: id, def: def, kind: OptionInt, lo: lo, hi: hi, meta: apply(settings)}
}

// List declares an option holding a subset of domain.
func List[T comparable](id string, def []T, domain []Choice[T], settings ...Setting) *ListOpt[T] {
	return &ListOpt[T]{id: id, def: def, domain: domain, meta: apply(settings)}
}

// optionInfo is an option as the catalog shows it.
type optionInfo struct {
	Key     string
	Kind    OptionKind
	Default any
	Choices []any
	// ChoiceLabels are the choices' label keys.
	ChoiceLabels []string
	Min          int
	Max          int
	meta         optionMeta
}

func (p *P) note(info optionInfo) {
	if !slices.ContainsFunc(p.options, func(o optionInfo) bool { return o.Key == info.Key }) {
		p.options = append(p.options, info)
	}
}

// Get reads an option while building; reading it makes it part of the
// policy's catalog.
func Get[T comparable](p *P, o *Opt[T]) T {
	key := p.policy.id + "." + o.id
	info := optionInfo{Key: key, Kind: o.kind, Default: o.def, Min: o.lo, Max: o.hi, meta: o.meta}
	for _, c := range o.choices {
		info.Choices = append(info.Choices, c.Value)
		info.ChoiceLabels = append(info.ChoiceLabels, c.Label)
	}
	p.note(info)
	if v, ok := p.values[key].(T); ok {
		return v
	}
	return o.def
}

// GetList reads a list option while building.
func GetList[T comparable](p *P, o *ListOpt[T]) []T {
	key := p.policy.id + "." + o.id
	info := optionInfo{Key: key, Kind: OptionList, Default: o.def, meta: o.meta}
	for _, c := range o.domain {
		info.Choices = append(info.Choices, c.Value)
		info.ChoiceLabels = append(info.ChoiceLabels, c.Label)
	}
	p.note(info)
	if v, ok := p.values[key].([]T); ok {
		return v
	}
	return o.def
}

// EnabledKey is the key of a policy's built-in enabled option.
func EnabledKey(policy string) string { return policy + ".enabled" }

// --- commands, presentation and history --------------------------------

// Command is a command a policy declares: a stable id, a label key, the
// intent it resolves to, and its default key.
type Command struct {
	ID     vocab.CommandID
	Label  string
	Intent vocab.Intent
	Policy string
	// Key is the default key, as modifiers and a key joined by "+":
	// "primary+b", "alt+left", "shift+tab". Empty is none.
	Key string
	// System is set on a command the platform's own key runs, as an Edit
	// menu role does: the keymap neither binds nor lists it.
	System bool
}

// CommandSetting refines a command declaration.
type CommandSetting func(*Command)

// Key is a command's default key (Command.Key).
func Key(spec string) CommandSetting { return func(c *Command) { c.Key = spec } }

// SystemKey marks a command the platform's key runs (Command.System).
func SystemKey() CommandSetting { return func(c *Command) { c.System = true } }

// Command declares a command.
func (p *P) Command(id vocab.CommandID, label string, intent vocab.Intent, settings ...CommandSetting) {
	c := Command{ID: id, Label: label, Intent: intent, Policy: p.policy.id}
	for _, s := range settings {
		s(&c)
	}
	p.commands = append(p.commands, c)
}

// Showcase is a policy's demonstration: beats played on an empty page,
// in a loop, after which the page holds for Hold.
type Showcase struct {
	Beats []Beat
	Hold  time.Duration
}

// BeatKind is what a beat does.
type BeatKind uint8

const (
	BeatType BeatKind = iota + 1
	BeatPress
	BeatWait
)

// Beat is one step of a showcase.
type Beat struct {
	Kind   BeatKind
	Text   string
	Intent vocab.Intent
	Wait   time.Duration
}

// Types types text a keystroke at a time.
func Types(text string) Beat { return Beat{Kind: BeatType, Text: text} }

// Presses sends one intent, as a key bound to it would.
func Presses(in vocab.Intent) Beat { return Beat{Kind: BeatPress, Intent: in} }

// Waits pauses.
func Waits(d time.Duration) Beat { return Beat{Kind: BeatWait, Wait: d} }

// Showcase declares the policy's demonstration.
func (p *P) Showcase(hold time.Duration, beats ...Beat) {
	p.showcase = &Showcase{Beats: beats, Hold: hold}
}

// PaintWhen says when a paint contribution shows.
type PaintWhen uint8

const (
	PaintAlways PaintWhen = iota
	// WhenCaretInside shows only while the caret is inside the span.
	WhenCaretInside
)

// Paint styles the spans of a fact kind with a highlight group.
type Paint struct {
	Group vocab.Group
	Fact  FactID
	When  PaintWhen
}

// Paint declares a paint contribution over a fact kind's spans.
func (p *P) Paint(group vocab.Group, fact FactID, when PaintWhen) {
	p.paints = append(p.paints, Paint{Group: group, Fact: fact, When: when})
}

// Guide is a key hint: wherever a preview of the command finds it
// available at a collapsed caret, the view shows the command's key where
// the command would put the caret (§12.1: guides are previews).
type Guide struct {
	Command vocab.CommandID
}

// Guide declares a key hint for a command another policy declares.
func (p *P) Guide(command vocab.CommandID) {
	p.guides = append(p.guides, Guide{Command: command})
}

// Grouping declares how history groups consecutive entries. At most one
// enabled policy may declare it.
func (p *P) Grouping(fn func(prev, next *doc.Entry) bool) {
	if p.grouping != nil {
		p.fail("grouping declared twice")
	}
	p.grouping = fn
}
