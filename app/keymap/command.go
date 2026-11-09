package keymap

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

// Editing commands are the editor's: their identities are the commands
// its policies declare, which the page resolves to intents through the
// program (§9.1). Undo, redo and select all are the Edit menu's roles,
// which keep their system keys.
const (
	MoveLeft           CommandID = "core.left"
	MoveRight          CommandID = "core.right"
	WordLeft           CommandID = "core.wordLeft"
	WordRight          CommandID = "core.wordRight"
	LineStart          CommandID = "core.lineStart"
	LineEnd            CommandID = "core.lineEnd"
	MoveUp             CommandID = "core.up"
	MoveDown           CommandID = "core.down"
	ParagraphUp        CommandID = "core.paragraphUp"
	ParagraphDown      CommandID = "core.paragraphDown"
	PageStart          CommandID = "core.pageStart"
	PageEnd            CommandID = "core.pageEnd"
	ScreenUp           CommandID = "core.pageUp"
	ScreenDown         CommandID = "core.pageDown"
	DeleteBackward     CommandID = "core.deleteBackward"
	DeleteForward      CommandID = "core.deleteForward"
	DeleteWordBackward CommandID = "core.deleteWordBackward"
	DeleteWordForward  CommandID = "core.deleteWordForward"
	DeleteToStart      CommandID = "core.deleteToStart"
	DeleteToEnd        CommandID = "core.deleteToEnd"
	Break              CommandID = "core.break"
	SoftBreak          CommandID = "core.softBreak"
	Nest               CommandID = "core.nest"
	Outdent            CommandID = "core.outdent"
	Leave              CommandID = "core.leave"
	Strong             CommandID = "core.strong"
	Emphasis           CommandID = "core.emphasis"
	Code               CommandID = "core.code"
)

// Command is one declared command.
type Command struct {
	ID    CommandID
	Label string
	Scope Scope
	// Default is the key the command starts with; a zero chord is none.
	Default Chord
}

var commands = [...]Command{
	{ID: MoveLeft, Label: "Move left", Scope: ScopeEditing, Default: NewChord("left", ModifiersNone)},
	{ID: MoveRight, Label: "Move right", Scope: ScopeEditing, Default: NewChord("right", ModifiersNone)},
	{ID: WordLeft, Label: "Previous word", Scope: ScopeEditing, Default: NewChord("left", ModifiersAlt)},
	{ID: WordRight, Label: "Next word", Scope: ScopeEditing, Default: NewChord("right", ModifiersAlt)},
	{ID: LineStart, Label: "Line start", Scope: ScopeEditing, Default: NewChord("left", ModifiersPrimary)},
	{ID: LineEnd, Label: "Line end", Scope: ScopeEditing, Default: NewChord("right", ModifiersPrimary)},
	{ID: MoveUp, Label: "Line up", Scope: ScopeEditing, Default: NewChord("up", ModifiersNone)},
	{ID: MoveDown, Label: "Line down", Scope: ScopeEditing, Default: NewChord("down", ModifiersNone)},
	{ID: ParagraphUp, Label: "Previous paragraph", Scope: ScopeEditing, Default: NewChord("up", ModifiersAlt)},
	{ID: ParagraphDown, Label: "Next paragraph", Scope: ScopeEditing, Default: NewChord("down", ModifiersAlt)},
	{ID: PageStart, Label: "Start of page", Scope: ScopeEditing, Default: NewChord("up", ModifiersPrimary)},
	{ID: PageEnd, Label: "End of page", Scope: ScopeEditing, Default: NewChord("down", ModifiersPrimary)},
	{ID: ScreenUp, Label: "Screen up", Scope: ScopeEditing, Default: NewChord("pageup", ModifiersNone)},
	{ID: ScreenDown, Label: "Screen down", Scope: ScopeEditing, Default: NewChord("pagedown", ModifiersNone)},
	{ID: DeleteBackward, Label: "Delete backward", Scope: ScopeEditing, Default: NewChord("backspace", ModifiersNone)},
	{ID: DeleteForward, Label: "Delete forward", Scope: ScopeEditing, Default: NewChord("delete", ModifiersNone)},
	{ID: DeleteWordBackward, Label: "Delete word backward", Scope: ScopeEditing,
		Default: NewChord("backspace", ModifiersAlt)},
	{ID: DeleteWordForward, Label: "Delete word forward", Scope: ScopeEditing,
		Default: NewChord("delete", ModifiersAlt)},
	{ID: DeleteToStart, Label: "Delete to line start", Scope: ScopeEditing,
		Default: NewChord("backspace", ModifiersPrimary)},
	{ID: DeleteToEnd, Label: "Delete to line end", Scope: ScopeEditing,
		Default: NewChord("delete", ModifiersPrimary)},
	{ID: Break, Label: "New block", Scope: ScopeEditing, Default: NewChord("enter", ModifiersNone)},
	{ID: SoftBreak, Label: "Line break", Scope: ScopeEditing, Default: NewChord("enter", ModifiersShift)},
	{ID: Nest, Label: "Nest", Scope: ScopeEditing, Default: NewChord("tab", ModifiersNone)},
	{ID: Outdent, Label: "Outdent", Scope: ScopeEditing, Default: NewChord("tab", ModifiersShift)},
	{ID: Leave, Label: "Leave", Scope: ScopeEditing, Default: NewChord("escape", ModifiersNone)},
	{ID: Strong, Label: "Strong", Scope: ScopeEditing, Default: NewChord("b", ModifiersPrimary)},
	{ID: Emphasis, Label: "Emphasis", Scope: ScopeEditing, Default: NewChord("i", ModifiersPrimary)},
	{ID: Code, Label: "Code", Scope: ScopeEditing, Default: NewChord("e", ModifiersPrimary)},
	{ID: NewPage, Label: "New page", Scope: ScopePages, Default: NewChord("n", ModifiersPrimary)},
	{ID: NextPage, Label: "Next tab", Scope: ScopePages,
		Default: NewChord("]", ModifiersPrimary|ModifiersShift)},
	{ID: PreviousPage, Label: "Previous tab", Scope: ScopePages,
		Default: NewChord("[", ModifiersPrimary|ModifiersShift)},
	{ID: ClosePage, Label: "Close tab", Scope: ScopePages, Default: NewChord("w", ModifiersPrimary)},
	{ID: FindPage, Label: "Find a page", Scope: ScopePages, Default: NewChord("p", ModifiersPrimary)},
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

// AllCommands returns every command in declaration order, which is display
// order within a scope.
func AllCommands() []Command {
	return append([]Command(nil), commands[:]...)
}

// CommandFor returns one command's declaration.
func CommandFor(id CommandID) (Command, bool) {
	for _, command := range commands {
		if command.ID == id {
			return command, true
		}
	}
	return Command{}, false
}
