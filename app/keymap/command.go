package keymap

// CommandID is a command's stable identity: how menus, the palette, the
// keymap and the person's remaps name it.
type CommandID string

// Scope is what a command acts on, which is how the Keymap groups it.
type Scope uint8

const (
	ScopePages Scope = iota + 1
	ScopeWindow
	ScopeApp
)

// Title names a group of the Keymap.
func (s Scope) Title() string {
	switch s {
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

// Command is one declared command.
type Command struct {
	ID    CommandID
	Label string
	Scope Scope
	// Default is the key the command starts with; a zero chord is none.
	Default Chord
}

var commands = [...]Command{
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
