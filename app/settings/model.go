// Package settings owns declarative product settings and validated changes
// independent of native widgets. Policy state stays in engine/kernel types.
package settings

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// Category is one top-level settings section.
type Category uint8

const (
	Editor Category = iota + 1
	Keymap
	Appearance
)

// AllCategories is display order.
var AllCategories = [...]Category{Editor, Keymap, Appearance}

func (c Category) Title() string {
	switch c {
	case Keymap:
		return "Keymap"
	case Appearance:
		return "Appearance"
	default:
		return "Editor"
	}
}

func (c Category) Description() string {
	switch c {
	case Keymap:
		return "Keys by what they act on. A feature's name marks the keys it adds."
	case Appearance:
		return "A comfortable space for your writing."
	default:
		return "Shape the way your words are edited."
	}
}

// Presentation is one app-wide writing-mode switch.
type Presentation uint8

const (
	PresentationMotion Presentation = iota + 1
	PresentationTypewriter
	PresentationZen
)

// BooleanKind distinguishes policy enablement, policy option, and presentation.
type BooleanKind uint8

const (
	BooleanPolicy BooleanKind = iota + 1
	BooleanOption
	BooleanPresentation
)

// BooleanSetting identifies one independently controlled switch.
type BooleanSetting struct {
	Kind         BooleanKind
	Policy       kernel.PolicyID
	Setting      kernel.SettingKey
	Presentation Presentation
}

func PolicyBoolean(policy kernel.PolicyID) BooleanSetting {
	return BooleanSetting{Kind: BooleanPolicy, Policy: policy}
}

func OptionBoolean(key kernel.SettingKey) BooleanSetting {
	return BooleanSetting{Kind: BooleanOption, Setting: key}
}

func PresentationBoolean(mode Presentation) BooleanSetting {
	return BooleanSetting{Kind: BooleanPresentation, Presentation: mode}
}

// ToggleSetting is one declarative switch row.
type ToggleSetting struct {
	Key         BooleanSetting
	Label       string
	Description string
	Value       bool
	Enabled     bool
	Selector    string
}

// Section is one group of editor policy settings: the policy's switches and
// the actions it answers to. An action its text names, as a key reference,
// is edited there, in the sentence; Shortcuts are the rest, as rows. Both are
// the entries the Keymap lists, so either page edits one binding.
type Section struct {
	Policy      kernel.PolicyID
	Title       string
	Description string
	Settings    []ToggleSetting
	Shortcuts   []Shortcut
}

// Shortcut is one command's complete binding display and override state.
//
// Scope says what the action acts on, which is how the Keymap groups it;
// Group is that scope's title. Policy is the policy the command is about.
// Origin is the switchable feature that adds the action, empty for the
// built-in ones: an action with an origin also lives on that feature's
// Editor card, and turns off with it.
type Shortcut struct {
	Command     kernel.CommandID
	Label       string
	Scope       editor.Scope
	Group       string
	Policy      kernel.PolicyID
	Origin      kernel.PolicyID
	OriginLabel string
	Bindings    []kernel.BindingTarget
	Enabled     bool
	Overridden  bool
}

// Data is the complete declarative snapshot consumed by any settings view.
type Data struct {
	PageID              *int
	Page                string
	Sections            []Section
	Shortcuts           []Shortcut
	Appearance          []ToggleSetting
	AppearanceValues    AppearanceSettings
	PolicyConfiguration kernel.PolicyConfiguration
}

// ChangeKind identifies one typed settings mutation.
type ChangeKind uint8

const (
	ChangeBoolean ChangeKind = iota + 1
	ChangeBinding
	ChangeUnbind
	ChangeResetBinding
	ChangeAppearance
)

// Change is a complete settings mutation. Only fields selected by Kind are read.
type Change struct {
	Kind    ChangeKind
	Boolean BooleanSetting
	Value   bool
	Command kernel.CommandID
	Binding kernel.BindingTarget
	// Displaced is the command that held Binding and gives it up in the same
	// change, so reassigning a taken shortcut is one atomic edit.
	Displaced  kernel.CommandID
	Appearance AppearanceChange
}

func ToggleBoolean(setting BooleanSetting, value bool) Change {
	return Change{Kind: ChangeBoolean, Boolean: setting, Value: value}
}

func Remap(command kernel.CommandID, binding kernel.BindingTarget) Change {
	return Change{Kind: ChangeBinding, Command: command, Binding: binding}
}

// RemapReplacing assigns binding to command and unbinds displaced, the
// command that held it, in the same policy change.
func RemapReplacing(command kernel.CommandID, binding kernel.BindingTarget, displaced kernel.CommandID) Change {
	return Change{Kind: ChangeBinding, Command: command, Binding: binding, Displaced: displaced}
}

// Shortcut returns command's entry.
func (d Data) Shortcut(command kernel.CommandID) (Shortcut, bool) {
	for _, shortcut := range d.Shortcuts {
		if shortcut.Command == command {
			return shortcut, true
		}
	}
	return Shortcut{}, false
}

// Owner returns the shortcut whose command holds target.
func (d Data) Owner(target kernel.BindingTarget) (Shortcut, bool) {
	for _, shortcut := range d.Shortcuts {
		for _, binding := range shortcut.Bindings {
			if binding == target {
				return shortcut, true
			}
		}
	}
	return Shortcut{}, false
}

func Unbind(command kernel.CommandID) Change {
	return Change{Kind: ChangeUnbind, Command: command}
}

func ResetBinding(command kernel.CommandID) Change {
	return Change{Kind: ChangeResetBinding, Command: command}
}

// AppearanceKind identifies one app-wide appearance mutation.
type AppearanceKind uint8

const (
	SetTheme AppearanceKind = iota + 1
	SetUIFontSize
	SetProseFontSize
	SetLineSpacing
)

// AppearanceChange is one neutral theme or text-size request. The accent
// is no setting: it follows the group of the sheet open.
type AppearanceChange struct {
	Kind     AppearanceKind
	Theme    uint8
	FontSize float32
	Spacing  float32
}

func NewAppearanceChange(change AppearanceChange) Change {
	return Change{Kind: ChangeAppearance, Appearance: change}
}
