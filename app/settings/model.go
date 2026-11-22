package settings

import (
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/policy"
)

// Category is one top-level settings page.
type Category uint8

const (
	Appearance Category = iota + 1
	Editor
	Keymap
)

// AllCategories is display order.
var AllCategories = [...]Category{Appearance, Editor, Keymap}

func (c Category) Title() string {
	switch c {
	case Editor:
		return "Editor"
	case Keymap:
		return "Keymap"
	default:
		return "Appearance"
	}
}

func (c Category) Description() string {
	switch c {
	case Editor:
		return "How the page answers what you write."
	case Keymap:
		return "Keys by what they act on."
	default:
		return "A comfortable space for your writing."
	}
}

// Presentation is one app-wide writing-mode switch.
type Presentation uint8

const (
	PresentationMotion Presentation = iota + 1
	PresentationTypewriter
)

// ToggleSetting is one declarative switch row.
type ToggleSetting struct {
	Mode        Presentation
	Label       string
	Description string
	Value       bool
	Selector    string
}

// Shortcut is one command's key and whether the key is the person's choice.
// Group titles the command's scope, which is how the Keymap groups it.
type Shortcut struct {
	Command keymap.CommandID
	Label   string
	Scope   keymap.Scope
	Group   string
	// Policy is the editor policy declaring an editing command.
	Policy     string
	Bindings   []keymap.Chord
	Overridden bool
}

// Data is the complete declarative snapshot consumed by any settings view.
type Data struct {
	Shortcuts []Shortcut
	Policies  []Policy
	// PolicyValues are the person's policy values, which the showcases
	// play under.
	PolicyValues     policy.Values
	Appearance       []ToggleSetting
	AppearanceValues AppearanceSettings
}

// Shortcut returns command's entry.
func (d Data) Shortcut(command keymap.CommandID) (Shortcut, bool) {
	for _, shortcut := range d.Shortcuts {
		if shortcut.Command == command {
			return shortcut, true
		}
	}
	return Shortcut{}, false
}

// Owner returns the shortcut whose command holds chord.
func (d Data) Owner(chord keymap.Chord) (Shortcut, bool) {
	for _, shortcut := range d.Shortcuts {
		for _, binding := range shortcut.Bindings {
			if binding == chord {
				return shortcut, true
			}
		}
	}
	return Shortcut{}, false
}

// ChangeKind identifies one typed settings mutation.
type ChangeKind uint8

const (
	ChangePresentation ChangeKind = iota + 1
	ChangeBinding
	ChangeUnbind
	ChangeResetBinding
	ChangeAppearance
	ChangePolicy
)

// Change is a complete settings mutation. Only fields selected by Kind are read.
type Change struct {
	Kind         ChangeKind
	Presentation Presentation
	Value        bool
	Command      keymap.CommandID
	Binding      keymap.Chord
	// Displaced is the command that held Binding and gives it up in the same
	// change, so reassigning a taken shortcut is one atomic edit.
	Displaced  keymap.CommandID
	Appearance AppearanceChange
	// Option and OptionValue are a policy option and its new value.
	Option      string
	OptionValue any
}

// SetPresentation turns one writing mode on or off.
func SetPresentation(mode Presentation, value bool) Change {
	return Change{Kind: ChangePresentation, Presentation: mode, Value: value}
}

// Remap gives command binding.
func Remap(command keymap.CommandID, binding keymap.Chord) Change {
	return Change{Kind: ChangeBinding, Command: command, Binding: binding}
}

// RemapReplacing assigns binding to command and unbinds displaced, the
// command that held it, in the same change.
func RemapReplacing(command keymap.CommandID, binding keymap.Chord, displaced keymap.CommandID) Change {
	return Change{Kind: ChangeBinding, Command: command, Binding: binding, Displaced: displaced}
}

// Unbind takes command's key away.
func Unbind(command keymap.CommandID) Change {
	return Change{Kind: ChangeUnbind, Command: command}
}

// ResetBinding gives command its default key again.
func ResetBinding(command keymap.CommandID) Change {
	return Change{Kind: ChangeResetBinding, Command: command}
}

// AppearanceKind identifies one app-wide appearance mutation.
type AppearanceKind uint8

const (
	SetTheme AppearanceKind = iota + 1
	SetUIFontSize
	SetProseFontSize
	SetLineSpacing
	SetCJKFace
)

// AppearanceChange is one neutral theme or text-size request. The accent
// is no setting: it follows the group of the sheet open.
type AppearanceChange struct {
	Kind     AppearanceKind
	Theme    uint8
	FontSize float32
	Spacing  float32
	// Face is SetCJKFace's face id (app/fonts), empty for the system's.
	Face string
}

func NewAppearanceChange(change AppearanceChange) Change {
	return Change{Kind: ChangeAppearance, Appearance: change}
}
