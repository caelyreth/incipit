// Package settings owns declarative product settings and validated changes
// independent of native widgets. Policy state stays in engine/kernel types.
package settings

import "github.com/caelyreth/neverland/engine/kernel"

// Category is one top-level settings section.
type Category uint8

const (
	Editor Category = iota + 1
	Keymap
	Appearance
	Diagnostics
)

// AllCategories is display order.
var AllCategories = [...]Category{Editor, Keymap, Appearance, Diagnostics}

func (c Category) Title() string {
	switch c {
	case Keymap:
		return "Keymap"
	case Appearance:
		return "Appearance"
	case Diagnostics:
		return "Diagnostics"
	default:
		return "Editor"
	}
}

func (c Category) Description() string {
	switch c {
	case Keymap:
		return "Make your most useful actions easy to reach."
	case Appearance:
		return "A comfortable space for your writing."
	case Diagnostics:
		return "Inspect the current page and its recent changes."
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

// Section is one group of editor policy settings.
type Section struct {
	Title       string
	Description string
	Settings    []ToggleSetting
}

// Shortcut is one command's complete binding display and override state.
type Shortcut struct {
	Command    kernel.CommandID
	Label      string
	Group      string
	Bindings   []kernel.BindingTarget
	Enabled    bool
	Overridden bool
}

// ActivityEntry is one published document revision snapshot.
type ActivityEntry struct {
	Revision   uint64
	Characters int
	Marks      int
}

// Data is the complete declarative snapshot consumed by any settings view.
type Data struct {
	PageID              *int
	Page                string
	Sections            []Section
	Shortcuts           []Shortcut
	Appearance          []ToggleSetting
	AppearanceValues    AppearanceSettings
	Activity            []string
	ActivityEntries     []ActivityEntry
	Scopes              []string
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
	Kind       ChangeKind
	Boolean    BooleanSetting
	Value      bool
	Command    kernel.CommandID
	Binding    kernel.BindingTarget
	Appearance AppearanceChange
}

func ToggleBoolean(setting BooleanSetting, value bool) Change {
	return Change{Kind: ChangeBoolean, Boolean: setting, Value: value}
}

func Remap(command kernel.CommandID, binding kernel.BindingTarget) Change {
	return Change{Kind: ChangeBinding, Command: command, Binding: binding}
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
	SetAccent
	SetUIFontSize
	SetProseFontSize
)

// AppearanceChange is one neutral theme, accent, or text-size request.
type AppearanceChange struct {
	Kind     AppearanceKind
	Theme    uint8
	Accent   uint8
	FontSize float32
}

func NewAppearanceChange(change AppearanceChange) Change {
	return Change{Kind: ChangeAppearance, Appearance: change}
}
