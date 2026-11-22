package keymap

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/app/phrases"
	"github.com/caelyreth/neverland/editor/policy"
)

// CommandID is a command's stable identity: how menus, the palette, the
// keymap and the person's remaps name it.
type CommandID string

// Scope is what a command acts on, which is how the Keymap groups it.
type Scope uint8

const (
	ScopeEditing Scope = iota + 1
	ScopePages
	// ScopeLibrary is what is done to the field's place and what is
	// selected in it, and going between places. Its keys act only while
	// the field is in front, so they may share a key with an editing
	// command, which acts only on the desk.
	ScopeLibrary
	ScopeWindow
	ScopeApp
)

// Scopes are the scopes in the order the Keymap lists them.
var Scopes = [...]Scope{ScopeEditing, ScopePages, ScopeLibrary, ScopeWindow, ScopeApp}

// Title names a group of the Keymap.
func (s Scope) Title() string {
	switch s {
	case ScopeEditing:
		return "Editing"
	case ScopePages:
		return "Sheets"
	case ScopeLibrary:
		return "Library"
	case ScopeWindow:
		return "View"
	default:
		return "App"
	}
}

// Description says what a group's keys act on.
func (s Scope) Description() string {
	switch s {
	case ScopeEditing:
		return "Write, move and select on the page."
	case ScopePages:
		return "Make, find, switch and close sheets."
	case ScopeLibrary:
		return "File, rename and trash what the field shows, and go between places."
	case ScopeWindow:
		return "Change what the window shows around your sheet."
	default:
		return "Open Neverland's own tools."
	}
}

// Shares reports whether a command of scope s may share a key with one
// of other: editing commands with each other, tried in order, and the
// library's with editing ones, as the field and the desk are never in
// front together.
func (s Scope) Shares(other Scope) bool {
	switch {
	case s == ScopeEditing && other == ScopeEditing:
		return true
	case s == ScopeLibrary && other == ScopeEditing, s == ScopeEditing && other == ScopeLibrary:
		return true
	}
	return false
}

// Command identities.
const (
	NewPage      CommandID = "page.new"
	NextPage     CommandID = "page.next"
	PreviousPage CommandID = "page.previous"
	ClosePage    CommandID = "page.close"
	CloseOthers  CommandID = "page.close-others"
	SplitPage    CommandID = "page.split"
	FormatTools  CommandID = "page.format-tools"
	FindPage     CommandID = "page.find"
	FindInSheet  CommandID = "page.find-in-sheet"

	NewGroup   CommandID = "library.new-group"
	Duplicate  CommandID = "library.duplicate"
	Rename     CommandID = "library.rename"
	Pin        CommandID = "library.pin"
	Trash      CommandID = "library.trash"
	EmptyTrash CommandID = "library.empty-trash"
	ViewList   CommandID = "library.view"
	Back       CommandID = "go.back"
	Forward    CommandID = "go.forward"
	Home       CommandID = "go.home"
	ShowTrash  CommandID = "go.trash"

	Typewriter CommandID = "window.typewriter"
	Motion     CommandID = "window.motion"
	Field      CommandID = "window.field"
	Commands   CommandID = "app.commands"
	Settings   CommandID = "app.settings"
	Shortcuts  CommandID = "app.shortcuts"
)

// Command is one declared command.
type Command struct {
	ID    CommandID
	Label string
	Scope Scope
	// Default is the key the command starts with; a zero chord is none.
	Default Chord
	// Policy is the editor policy declaring an editing command.
	Policy string
}

// Declared is every command: the editing commands the program's policies
// declare (§9.1), in manifest order, each with its label's words and
// default key, then the app's own. Editing commands sharing a key are
// tried in this order, as Tab leaves a scope or, with none to leave,
// nests. A command the platform's key runs, such as undo, is not listed.
func Declared(catalog policy.Catalog) []Command {
	out := make([]Command, 0, len(catalog.Commands)+len(appCommands))
	for _, c := range catalog.Commands {
		if c.System {
			continue
		}
		chord, _ := ParseChord(c.Key)
		out = append(out, Command{ID: CommandID(c.ID), Label: phrases.Text(c.Label), Scope: ScopeEditing,
			Default: chord, Policy: c.Policy})
	}
	return append(out, appCommands[:]...)
}

// ParseChord reads a key as policies declare it: modifiers by name, then
// the key, joined by "+", as "primary+b" or "shift+tab". Empty is no key.
func ParseChord(spec string) (Chord, bool) {
	if spec == "" {
		return Chord{}, false
	}
	parts := strings.Split(spec, "+")
	var modifiers Modifiers
	for _, part := range parts[:len(parts)-1] {
		i := slices.IndexFunc(AllModifiers[:], func(m Modifier) bool { return m.String() == part })
		if i < 0 {
			return Chord{}, false
		}
		modifiers = modifiers.With(AllModifiers[i])
	}
	chord := NewChord(parts[len(parts)-1], modifiers)
	return chord, chord.Valid()
}

// appCommands are the app's own: pages, the window and its tools.
var appCommands = [...]Command{
	{ID: NewPage, Label: "New Sheet", Scope: ScopePages, Default: NewChord("n", ModifiersPrimary)},
	{ID: NextPage, Label: "Next Sheet", Scope: ScopePages,
		Default: NewChord("]", ModifiersPrimary|ModifiersShift)},
	{ID: PreviousPage, Label: "Previous Sheet", Scope: ScopePages,
		Default: NewChord("[", ModifiersPrimary|ModifiersShift)},
	{ID: ClosePage, Label: "Close Sheet", Scope: ScopePages, Default: NewChord("w", ModifiersPrimary)},
	{ID: CloseOthers, Label: "Close Other Sheets", Scope: ScopePages,
		Default: NewChord("w", ModifiersPrimary|ModifiersAlt)},
	{ID: SplitPage, Label: "Split Sheet", Scope: ScopePages, Default: NewChord("\\", ModifiersPrimary)},
	{ID: FormatTools, Label: "Show Formatting", Scope: ScopePages, Default: NewChord(".", ModifiersPrimary)},
	{ID: FindPage, Label: "Go to Sheet", Scope: ScopePages, Default: NewChord("p", ModifiersPrimary)},
	{ID: FindInSheet, Label: "Find", Scope: ScopePages, Default: NewChord("f", ModifiersPrimary)},
	{ID: NewGroup, Label: "New Group", Scope: ScopeLibrary, Default: NewChord("n", ModifiersPrimary|ModifiersShift)},
	{ID: Duplicate, Label: "Duplicate", Scope: ScopeLibrary, Default: NewChord("d", ModifiersPrimary)},
	{ID: Rename, Label: "Rename", Scope: ScopeLibrary, Default: NewChord("r", ModifiersPrimary)},
	{ID: Pin, Label: "Pin", Scope: ScopeLibrary},
	{ID: Trash, Label: "Move to Trash", Scope: ScopeLibrary, Default: NewChord("backspace", ModifiersPrimary)},
	{ID: EmptyTrash, Label: "Empty Trash", Scope: ScopeLibrary,
		Default: NewChord("backspace", ModifiersPrimary|ModifiersShift)},
	{ID: ViewList, Label: "View as List", Scope: ScopeLibrary, Default: NewChord("l", ModifiersPrimary)},
	{ID: Back, Label: "Back", Scope: ScopeLibrary, Default: NewChord("[", ModifiersPrimary)},
	{ID: Forward, Label: "Forward", Scope: ScopeLibrary, Default: NewChord("]", ModifiersPrimary)},
	{ID: Home, Label: "Home", Scope: ScopeWindow, Default: NewChord("h", ModifiersPrimary|ModifiersShift)},
	{ID: ShowTrash, Label: "Trash", Scope: ScopeWindow},
	{ID: Field, Label: "Show Field", Scope: ScopeWindow, Default: NewChord("0", ModifiersPrimary)},
	{ID: Typewriter, Label: "Typewriter Mode", Scope: ScopeWindow,
		Default: NewChord("t", ModifiersPrimary|ModifiersAlt)},
	{ID: Motion, Label: "Motion", Scope: ScopeWindow, Default: NewChord("m", ModifiersPrimary|ModifiersAlt)},
	{ID: Commands, Label: "Commands", Scope: ScopeApp, Default: NewChord("k", ModifiersPrimary)},
	{ID: Settings, Label: "Settings", Scope: ScopeApp, Default: NewChord(",", ModifiersPrimary)},
	{ID: Shortcuts, Label: "Keyboard Shortcuts", Scope: ScopeApp, Default: NewChord("/", ModifiersPrimary)},
}
