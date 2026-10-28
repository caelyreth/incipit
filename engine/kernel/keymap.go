package kernel

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// BindingContext is a native input namespace. "editor" is the built-in context;
// other consumers may declare independent named contexts.
type BindingContext string

const EditorBindingContext BindingContext = "editor"

func NewBindingContext(name string) BindingContext {
	if name == string(EditorBindingContext) {
		return EditorBindingContext
	}
	return BindingContext(name)
}

func (c BindingContext) String() string { return string(c) }

// Modifier identifies one canonical modifier bit.
type Modifier uint8

const (
	ModifierAlt Modifier = iota
	ModifierControl
	ModifierFunction
	ModifierPrimary
	ModifierShift
	ModifierSuper
)

var AllModifiers = [...]Modifier{
	ModifierAlt,
	ModifierControl,
	ModifierFunction,
	ModifierPrimary,
	ModifierShift,
	ModifierSuper,
}

func (m Modifier) String() string {
	switch m {
	case ModifierAlt:
		return "alt"
	case ModifierControl:
		return "ctrl"
	case ModifierFunction:
		return "fn"
	case ModifierPrimary:
		return "primary"
	case ModifierShift:
		return "shift"
	case ModifierSuper:
		return "super"
	default:
		return fmt.Sprintf("modifier(%d)", m)
	}
}

type UnknownModifierError struct {
	Name string
}

func (e *UnknownModifierError) Error() string {
	return "unknown shortcut modifier " + e.Name
}

func ParseModifier(name string) (Modifier, error) {
	for _, modifier := range AllModifiers {
		if modifier.String() == name {
			return modifier, nil
		}
	}
	return 0, &UnknownModifierError{Name: name}
}

// Modifiers is a normalized set of canonical modifier bits.
type Modifiers uint8

const (
	ModifiersNone     Modifiers = 0
	ModifiersAlt      Modifiers = 1 << ModifierAlt
	ModifiersControl  Modifiers = 1 << ModifierControl
	ModifiersFunction Modifiers = 1 << ModifierFunction
	ModifiersPrimary  Modifiers = 1 << ModifierPrimary
	ModifiersShift    Modifiers = 1 << ModifierShift
	ModifiersSuper    Modifiers = 1 << ModifierSuper
)

func (m Modifiers) Has(modifier Modifier) bool {
	return m&(1<<modifier) != 0
}

func (m Modifiers) With(modifier Modifier) Modifiers {
	return m | (1 << modifier)
}

func (m Modifiers) Union(other Modifiers) Modifiers {
	return m | other
}

func (m Modifiers) All() []Modifier {
	modifiers := make([]Modifier, 0, len(AllModifiers))
	for _, modifier := range AllModifiers {
		if m.Has(modifier) {
			modifiers = append(modifiers, modifier)
		}
	}
	return modifiers
}

func (m Modifiers) String() string {
	names := make([]string, 0, len(AllModifiers))
	for _, modifier := range m.All() {
		names = append(names, modifier.String())
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, "+")
}

func ParseModifiers(names ...string) (Modifiers, error) {
	var result Modifiers
	for _, name := range names {
		modifier, err := ParseModifier(name)
		if err != nil {
			return 0, err
		}
		result = result.With(modifier)
	}
	return result, nil
}

// BindingTarget is one normalized context/key/modifier lookup key.
type BindingTarget struct {
	context   BindingContext
	key       string
	modifiers Modifiers
}

func NewBindingTarget(context BindingContext, key string, modifiers Modifiers) BindingTarget {
	return BindingTarget{context: NewBindingContext(string(context)), key: key, modifiers: modifiers}
}

func (t BindingTarget) Context() BindingContext { return t.context }

func (t BindingTarget) Key() string { return t.key }

func (t BindingTarget) Modifiers() Modifiers { return t.modifiers }

func (t BindingTarget) Validate() error {
	if strings.TrimSpace(string(t.context)) == "" {
		return fmt.Errorf("binding context cannot be empty")
	}
	if strings.TrimSpace(t.key) == "" {
		return fmt.Errorf("binding key cannot be empty")
	}
	for _, r := range t.key {
		if unicode.IsControl(r) {
			return fmt.Errorf("binding key cannot contain control characters")
		}
	}
	return nil
}

func (t BindingTarget) String() string {
	names := make([]string, 0, 4)
	for _, modifier := range t.modifiers.All() {
		names = append(names, modifier.String())
	}
	if len(names) == 0 {
		return fmt.Sprintf("%s/%s", t.context, t.key)
	}
	return fmt.Sprintf("%s/%s+%s", t.context, strings.Join(names, "+"), t.key)
}

// BindingSpec declares one default command binding.
type BindingSpec struct {
	Command CommandID
	Target  BindingTarget
}

type bindingLookupKey struct {
	context   BindingContext
	modifiers Modifiers
	key       string
}

// PreparedKeymap is immutable after preparation.
type PreparedKeymap struct {
	byTarget  map[bindingLookupKey]CommandID
	byCommand map[CommandID][]BindingTarget
}

func newPreparedKeymap() *PreparedKeymap {
	return &PreparedKeymap{
		byTarget:  make(map[bindingLookupKey]CommandID),
		byCommand: make(map[CommandID][]BindingTarget),
	}
}

// Clone returns an independent prepared keymap.
func (k *PreparedKeymap) Clone() *PreparedKeymap {
	if k == nil {
		return nil
	}
	cloned := newPreparedKeymap()
	for lookup, command := range k.byTarget {
		cloned.byTarget[lookup] = command
	}
	for command, targets := range k.byCommand {
		cloned.byCommand[command] = cloneSlice(targets)
	}
	return cloned
}

func (k *PreparedKeymap) bind(command CommandID, targets []BindingTarget) error {
	sorted := cloneSlice(targets)
	sort.Slice(sorted, func(i, j int) bool {
		return compareBindingTargets(sorted[i], sorted[j]) < 0
	})
	deduped := sorted[:0]
	for _, target := range sorted {
		if len(deduped) == 0 || deduped[len(deduped)-1] != target {
			deduped = append(deduped, target)
		}
	}
	for _, target := range deduped {
		lookup := bindingLookupKey{
			context:   target.context,
			modifiers: target.modifiers,
			key:       target.key,
		}
		if existing, ok := k.byTarget[lookup]; ok {
			return &KeyConflictError{Target: target, Existing: existing, Incoming: command}
		}
		k.byTarget[lookup] = command
	}
	k.byCommand[command] = deduped
	return nil
}

func compareBindingTargets(first, second BindingTarget) int {
	if first.context != second.context {
		return strings.Compare(string(first.context), string(second.context))
	}
	if first.key != second.key {
		return strings.Compare(first.key, second.key)
	}
	switch {
	case first.modifiers < second.modifiers:
		return -1
	case first.modifiers > second.modifiers:
		return 1
	default:
		return 0
	}
}

type KeyConflictError struct {
	Target   BindingTarget
	Existing CommandID
	Incoming CommandID
}

func (e *KeyConflictError) Error() string {
	return fmt.Sprintf("key conflict at %s: %s and %s", e.Target, e.Existing, e.Incoming)
}

func (k *PreparedKeymap) CommandFor(target BindingTarget) (CommandID, bool) {
	return k.CommandForKey(target.context, target.key, target.modifiers)
}

func (k *PreparedKeymap) CommandForKey(context BindingContext, key string, modifiers Modifiers) (CommandID, bool) {
	command, ok := k.byTarget[bindingLookupKey{
		context:   NewBindingContext(string(context)),
		modifiers: modifiers,
		key:       key,
	}]
	return command, ok
}

func (k *PreparedKeymap) IsCommandEnabled(command CommandID) bool {
	_, ok := k.byCommand[command]
	return ok
}

func (k *PreparedKeymap) BindingsFor(command CommandID) []BindingTarget {
	return cloneSlice(k.byCommand[command])
}

func (k *PreparedKeymap) Bindings() []Binding {
	bindings := make([]Binding, 0, len(k.byTarget))
	for _, command := range sortedCommands(k.byCommand) {
		for _, target := range k.byCommand[command] {
			bindings = append(bindings, Binding{Target: target, Command: command})
		}
	}
	return bindings
}

type Binding struct {
	Target  BindingTarget
	Command CommandID
}

func sortedCommands(commands map[CommandID][]BindingTarget) []CommandID {
	result := make([]CommandID, 0, len(commands))
	for command := range commands {
		result = append(result, command)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
