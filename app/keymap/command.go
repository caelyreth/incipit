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
	ScopeWindow
	ScopeApp
)

// Title names a group of the Keymap.
func (s Scope) Title() string {
	switch s {
	case ScopeEditing:
		return "Editing"
	case ScopePages:
		return "Pages"
	case ScopeWindow:
		return "Window"
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
		return "Open, find, switch and close pages."
	case ScopeWindow:
		return "Change what the window shows around your page."
	default:
		return "Open Neverland's own tools."
	}
}

// Command identities.
const (
	NewPage      CommandID = "page.new"
	NextPage     CommandID = "page.next"
	PreviousPage CommandID = "page.previous"
	ClosePage    CommandID = "page.close"
	FindPage     CommandID = "page.find"
	FindInSheet  CommandID = "page.find-in-sheet"
	Zen          CommandID = "window.zen"
	Typewriter   CommandID = "window.typewriter"
	Motion       CommandID = "window.motion"
	Sidebar      CommandID = "window.sidebar"
	SheetList    CommandID = "window.sheet-list"
	LibraryView  CommandID = "window.view-library"
	SheetsView   CommandID = "window.view-sheets"
	PageView     CommandID = "window.view-page"
	Commands     CommandID = "app.commands"
	Settings     CommandID = "app.settings"
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
	{ID: NewPage, Label: "New page", Scope: ScopePages, Default: NewChord("n", ModifiersPrimary)},
	{ID: NextPage, Label: "Next tab", Scope: ScopePages,
		Default: NewChord("]", ModifiersPrimary|ModifiersShift)},
	{ID: PreviousPage, Label: "Previous tab", Scope: ScopePages,
		Default: NewChord("[", ModifiersPrimary|ModifiersShift)},
	{ID: ClosePage, Label: "Close tab", Scope: ScopePages, Default: NewChord("w", ModifiersPrimary)},
	{ID: FindPage, Label: "Find a page", Scope: ScopePages, Default: NewChord("p", ModifiersPrimary)},
	{ID: FindInSheet, Label: "Find in sheet", Scope: ScopePages, Default: NewChord("f", ModifiersPrimary)},
	{ID: Zen, Label: "Toggle Zen mode", Scope: ScopeWindow, Default: NewChord("f", ModifiersPrimary|ModifiersShift)},
	{ID: Typewriter, Label: "Toggle typewriter mode", Scope: ScopeWindow,
		Default: NewChord("t", ModifiersPrimary|ModifiersAlt)},
	{ID: Motion, Label: "Toggle motion", Scope: ScopeWindow, Default: NewChord("m", ModifiersPrimary|ModifiersAlt)},
	{ID: Sidebar, Label: "Toggle library", Scope: ScopeWindow, Default: NewChord("b", ModifiersPrimary|ModifiersAlt)},
	{ID: SheetList, Label: "Toggle sheet list", Scope: ScopeWindow,
		Default: NewChord("l", ModifiersPrimary|ModifiersAlt)},
	{ID: LibraryView, Label: "Show library and sheets", Scope: ScopeWindow, Default: NewChord("1", ModifiersPrimary)},
	{ID: SheetsView, Label: "Show sheet list only", Scope: ScopeWindow, Default: NewChord("2", ModifiersPrimary)},
	{ID: PageView, Label: "Show page only", Scope: ScopeWindow, Default: NewChord("3", ModifiersPrimary)},
	{ID: Commands, Label: "Search commands", Scope: ScopeApp, Default: NewChord("k", ModifiersPrimary)},
	{ID: Settings, Label: "Open settings", Scope: ScopeApp, Default: NewChord(",", ModifiersPrimary)},
}
