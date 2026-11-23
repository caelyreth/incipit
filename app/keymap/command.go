package keymap

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/app/phrases"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/locales"
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

// Key names a group of the Keymap in its section's identity, in every
// language.
func (s Scope) Key() string {
	switch s {
	case ScopeEditing:
		return "editing"
	case ScopePages:
		return "pages"
	case ScopeLibrary:
		return "library"
	case ScopeWindow:
		return "window"
	default:
		return "app"
	}
}

// Title names a group of the Keymap.
func (s Scope) Title() string {
	switch s {
	case ScopeEditing:
		return locales.ScopeEditing()
	case ScopePages:
		return locales.ScopeSheets()
	case ScopeLibrary:
		return locales.ScopeLibrary()
	case ScopeWindow:
		return locales.ScopeView()
	default:
		return locales.ScopeApp()
	}
}

// Description says what a group's keys act on.
func (s Scope) Description() string {
	switch s {
	case ScopeEditing:
		return locales.ScopeEditingDescription()
	case ScopePages:
		return locales.ScopeSheetsDescription()
	case ScopeLibrary:
		return locales.ScopeLibraryDescription()
	case ScopeWindow:
		return locales.ScopeViewDescription()
	default:
		return locales.ScopeAppDescription()
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
	Export       CommandID = "page.export"

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
	app := appCommands()
	out := make([]Command, 0, len(catalog.Commands)+len(app))
	for _, c := range catalog.Commands {
		if c.System {
			continue
		}
		chord, _ := ParseChord(c.Key)
		out = append(out, Command{ID: CommandID(c.ID), Label: phrases.Text(c.Label), Scope: ScopeEditing,
			Default: chord, Policy: c.Policy})
	}
	return append(out, app...)
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

// appCommands are the app's own: pages, the window and its tools,
// labeled in the language the app shows.
func appCommands() []Command {
	return []Command{
		{ID: NewPage, Label: locales.CommandNewSheet(), Scope: ScopePages, Default: NewChord("n", ModifiersPrimary)},
		{ID: NextPage, Label: locales.CommandNextSheet(), Scope: ScopePages,
			Default: NewChord("]", ModifiersPrimary|ModifiersShift)},
		{ID: PreviousPage, Label: locales.CommandPreviousSheet(), Scope: ScopePages,
			Default: NewChord("[", ModifiersPrimary|ModifiersShift)},
		{ID: ClosePage, Label: locales.CommandCloseSheet(), Scope: ScopePages, Default: NewChord("w", ModifiersPrimary)},
		{ID: CloseOthers, Label: locales.CommandCloseOtherSheets(), Scope: ScopePages,
			Default: NewChord("w", ModifiersPrimary|ModifiersAlt)},
		{ID: SplitPage, Label: locales.CommandSplitSheet(), Scope: ScopePages, Default: NewChord("\\", ModifiersPrimary)},
		{ID: FormatTools, Label: locales.CommandShowFormatting(), Scope: ScopePages, Default: NewChord(".", ModifiersPrimary)},
		{ID: FindPage, Label: locales.CommandGoToSheet(), Scope: ScopePages, Default: NewChord("p", ModifiersPrimary)},
		{ID: FindInSheet, Label: locales.CommandFind(), Scope: ScopePages, Default: NewChord("f", ModifiersPrimary)},
		{ID: Export, Label: locales.CommandExport(), Scope: ScopePages, Default: NewChord("e", ModifiersPrimary|ModifiersShift)},
		{ID: NewGroup, Label: locales.CommandNewGroup(), Scope: ScopeLibrary, Default: NewChord("n", ModifiersPrimary|ModifiersShift)},
		{ID: Duplicate, Label: locales.CommandDuplicate(), Scope: ScopeLibrary, Default: NewChord("d", ModifiersPrimary)},
		{ID: Rename, Label: locales.CommandRename(), Scope: ScopeLibrary, Default: NewChord("r", ModifiersPrimary)},
		{ID: Pin, Label: locales.CommandPin(), Scope: ScopeLibrary},
		{ID: Trash, Label: locales.CommandMoveToTrash(), Scope: ScopeLibrary, Default: NewChord("backspace", ModifiersPrimary)},
		{ID: EmptyTrash, Label: locales.CommandEmptyTrash(), Scope: ScopeLibrary,
			Default: NewChord("backspace", ModifiersPrimary|ModifiersShift)},
		{ID: ViewList, Label: locales.CommandViewAsList(), Scope: ScopeLibrary, Default: NewChord("l", ModifiersPrimary)},
		{ID: Back, Label: locales.CommandBack(), Scope: ScopeLibrary, Default: NewChord("[", ModifiersPrimary)},
		{ID: Forward, Label: locales.CommandForward(), Scope: ScopeLibrary, Default: NewChord("]", ModifiersPrimary)},
		{ID: Home, Label: locales.CommandHome(), Scope: ScopeWindow, Default: NewChord("h", ModifiersPrimary|ModifiersShift)},
		{ID: ShowTrash, Label: locales.CommandTrash(), Scope: ScopeWindow},
		{ID: Field, Label: locales.CommandShowField(), Scope: ScopeWindow, Default: NewChord("0", ModifiersPrimary)},
		{ID: Typewriter, Label: locales.CommandTypewriterMode(), Scope: ScopeWindow,
			Default: NewChord("t", ModifiersPrimary|ModifiersAlt)},
		{ID: Motion, Label: locales.CommandMotion(), Scope: ScopeWindow, Default: NewChord("m", ModifiersPrimary|ModifiersAlt)},
		{ID: Commands, Label: locales.CommandCommands(), Scope: ScopeApp, Default: NewChord("k", ModifiersPrimary)},
		{ID: Settings, Label: locales.CommandSettings(), Scope: ScopeApp, Default: NewChord(",", ModifiersPrimary)},
		{ID: Shortcuts, Label: locales.CommandKeyboardShortcuts(), Scope: ScopeApp, Default: NewChord("/", ModifiersPrimary)},
	}
}
