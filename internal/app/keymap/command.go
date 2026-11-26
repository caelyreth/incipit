package keymap

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/app/phrases"
	"github.com/caelyreth/neverland/internal/editor/policy"
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
	Commands     CommandID = "app.commands"
	Settings     CommandID = "app.settings"
	Shortcuts    CommandID = "app.shortcuts"
	CheckUpdates CommandID = "app.check-updates"

	// CheckSpelling, CheckGrammar, CorrectSpelling and PredictSpelling turn
	// the spelling policy and its options on and off, from Edit › Spelling
	// and Grammar.
	CheckSpelling   CommandID = "window.check-spelling"
	CheckGrammar    CommandID = "window.check-grammar"
	CorrectSpelling CommandID = "window.correct-spelling"
	PredictSpelling CommandID = "window.predict-spelling"
)

// Command is one declared command.
type Command struct {
	ID CommandID
	// Words is the key of its label's words: a policy's text key, or a
	// message of the app's catalog; Label reads them.
	Words string
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
		out = append(out, Command{ID: CommandID(c.ID), Words: c.Label, Scope: ScopeEditing,
			Default: chord, Policy: c.Policy})
	}
	return append(out, app...)
}

// Label is the command's name in the language the app shows now: read
// each time, never kept, so a keymap made in one language names its
// commands in the next.
func (c Command) Label() string { return phrases.Text(c.Words) }

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

// appCommands are the app's own: pages, the window and its tools, their
// labels the app's catalog's.
func appCommands() []Command {
	return []Command{
		{ID: NewPage, Words: "commandNewSheet", Scope: ScopePages, Default: NewChord("n", ModifiersPrimary)},
		{ID: NextPage, Words: "commandNextSheet", Scope: ScopePages,
			Default: NewChord("]", ModifiersPrimary|ModifiersShift)},
		{ID: PreviousPage, Words: "commandPreviousSheet", Scope: ScopePages,
			Default: NewChord("[", ModifiersPrimary|ModifiersShift)},
		{ID: ClosePage, Words: "commandCloseSheet", Scope: ScopePages, Default: NewChord("w", ModifiersPrimary)},
		{ID: CloseOthers, Words: "commandCloseOtherSheets", Scope: ScopePages,
			Default: NewChord("w", ModifiersPrimary|ModifiersAlt)},
		{ID: SplitPage, Words: "commandSplitSheet", Scope: ScopePages, Default: NewChord("\\", ModifiersPrimary)},
		{ID: FormatTools, Words: "commandShowFormatting", Scope: ScopePages, Default: NewChord(".", ModifiersPrimary)},
		{ID: FindPage, Words: "commandGoToSheet", Scope: ScopePages, Default: NewChord("p", ModifiersPrimary)},
		{ID: FindInSheet, Words: "commandFind", Scope: ScopePages, Default: NewChord("f", ModifiersPrimary)},
		{ID: Export, Words: "commandExport", Scope: ScopePages, Default: NewChord("e", ModifiersPrimary|ModifiersShift)},
		{ID: NewGroup, Words: "commandNewGroup", Scope: ScopeLibrary, Default: NewChord("n", ModifiersPrimary|ModifiersShift)},
		{ID: Duplicate, Words: "commandDuplicate", Scope: ScopeLibrary, Default: NewChord("d", ModifiersPrimary)},
		{ID: Rename, Words: "commandRename", Scope: ScopeLibrary, Default: NewChord("r", ModifiersPrimary)},
		{ID: Pin, Words: "commandPin", Scope: ScopeLibrary},
		{ID: Trash, Words: "commandMoveToTrash", Scope: ScopeLibrary, Default: NewChord("backspace", ModifiersPrimary)},
		{ID: EmptyTrash, Words: "commandEmptyTrash", Scope: ScopeLibrary,
			Default: NewChord("backspace", ModifiersPrimary|ModifiersShift)},
		{ID: ViewList, Words: "commandViewAsList", Scope: ScopeLibrary, Default: NewChord("l", ModifiersPrimary)},
		{ID: Back, Words: "commandBack", Scope: ScopeLibrary, Default: NewChord("[", ModifiersPrimary)},
		{ID: Forward, Words: "commandForward", Scope: ScopeLibrary, Default: NewChord("]", ModifiersPrimary)},
		{ID: Home, Words: "commandHome", Scope: ScopeWindow, Default: NewChord("h", ModifiersPrimary|ModifiersShift)},
		{ID: ShowTrash, Words: "commandTrash", Scope: ScopeWindow},
		{ID: Field, Words: "commandShowField", Scope: ScopeWindow, Default: NewChord("0", ModifiersPrimary)},
		{ID: Typewriter, Words: "commandTypewriterMode", Scope: ScopeWindow,
			Default: NewChord("t", ModifiersPrimary|ModifiersAlt)},
		{ID: Motion, Words: "commandMotion", Scope: ScopeWindow, Default: NewChord("m", ModifiersPrimary|ModifiersAlt)},
		{ID: CheckSpelling, Words: "commandCheckSpelling", Scope: ScopeWindow},
		{ID: CheckGrammar, Words: "commandCheckGrammar", Scope: ScopeWindow},
		{ID: CorrectSpelling, Words: "commandCorrectSpelling", Scope: ScopeWindow},
		{ID: PredictSpelling, Words: "commandPredictSpelling", Scope: ScopeWindow},
		{ID: Commands, Words: "commandCommands", Scope: ScopeApp, Default: NewChord("k", ModifiersPrimary)},
		{ID: Settings, Words: "commandSettings", Scope: ScopeApp, Default: NewChord(",", ModifiersPrimary)},
		{ID: Shortcuts, Words: "commandKeyboardShortcuts", Scope: ScopeApp, Default: NewChord("/", ModifiersPrimary)},
		{ID: CheckUpdates, Words: "commandCheckUpdates", Scope: ScopeApp},
	}
}
