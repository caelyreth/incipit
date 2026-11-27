package shell

import (
	"time"

	"github.com/caelyreth/incipit/internal/app/settings"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Command is one of the app's commands: what it is called, the group it
// is listed in, and the caps of its key, if it has one.
type Command struct {
	ID, Label, Group string
	Binding          string
	Keys             []string
}

// Page is an open sheet in one of the desk's columns: its sheet, and the
// column, one of several where the sheet is split.
type Page struct {
	ID     string
	Column string
	Title  string
}

// SheetFrame is how a sheet's page is set in its column on the desk.
type SheetFrame struct {
	// Prose and UIFontSize are the document and interface type sizes.
	Prose      theme.ProseStyle
	UIFontSize float32
	// Top and Bottom are the reading space above its first line and below
	// its last, scrolled with it; Dock is the room the floating writing
	// actions take over its foot.
	Top, Bottom, Dock float32
	// LayoutWidth is the editor's viewport, and Inset its text's left
	// inset, centring the reading measure in it.
	LayoutWidth, Inset float32
	// Focus lets the page take the keyboard while nothing else holds it,
	// and Claim gives it the keyboard now, from wherever it is.
	Focus, Claim bool
}

// ToolbarState is the writing modes the dock switches, and their
// current shortcuts.
type ToolbarState struct {
	Typewriter        bool
	TypewriterEnabled bool
	Bindings          ToolbarBindings
}

// ToolbarBindings carries the current shortcut text for action tooltips.
type ToolbarBindings struct {
	Typewriter string
}

type State struct {
	// root is the shell's outermost element, for where the focus is.
	root ui.Handle
	// dockHandle is the writing actions, which float over the window
	// outside root (components.Floating).
	dockHandle ui.Handle

	Appearance settings.PresentationState
	Settings   theme.Settings
	// Colors carries theme changes; nil shows Settings as they are.
	Colors *theme.Transition
	// Open are the desk's columns, in their order; ActiveColumn is the one
	// in front, and ActivePage its sheet.
	Open         []Page
	ActivePage   string
	ActiveColumn string
	// Library is the notebook's library shown in the field; nil shows none.
	Library    Library
	Statistics string
	// FindQuery is a sheet's find as written after its fixed $sheet (the
	// query it runs is FindText), FindRequested asks its field
	// for the focus, and FindCount is what it found, as the window counts
	// it.
	FindQuery     string
	FindRequested bool
	FindCount     FindCount
	finder        findBar
	Toolbar       ToolbarState

	// Sheet draws an open column's page, as frame sets it.
	Sheet func(c *ui.Context, page Page, frame SheetFrame)
	// Thumbnail is the small picture a card shows of a sheet's first
	// figure, by its media id, as far as it has come: nil while it is
	// read, and false where it cannot show. Nil shows each one's place.
	Thumbnail func(media string, now time.Time) (*ui.Bitmap, bool)
	// SelectColumn brings a column in front, CloseColumnID closes one,
	// and SplitColumnID opens its sheet again beside it.
	SelectColumn  func(column string)
	CloseColumnID func(column string)
	SplitColumnID func(column string)
	// AppSettings are the app's settings, which the field shows as cards.
	AppSettings AppSettings
	// Commands are the app's commands, as the pill lists them written
	// after ">"; RunCommand runs one.
	Commands   func() []Command
	RunCommand func(id string)
	// Wrote is when the last input came to a sheet's page, any sheet's:
	// the desk's heads keep out of the way while it is recent.
	Wrote func() time.Time
	// KeyOf is the key a command runs on, for the menus to show, and
	// Binding the same key spelled, for a hint.
	KeyOf   KeyOf
	Binding func(id string) string
	// Find runs a sheet's find on the page in front; FindStep goes to the
	// next match or the one before; EndFind ends it.
	Find       func(query string)
	FindStep   func(forward bool)
	EndFind    func()
	Typewriter func(bool)
	// Tools are the page's in front for the dock, what is inserted and what
	// the selection is changed into, as it stands; RunTool runs one.
	Tools   func() []components.ToolGroup
	RunTool func(id string)

	// armedKey & armedAt govern two-click destructive confirmation.
	armedKey string
	armedAt  time.Time
	// escapedAt is when Escape was last pressed on the desk (scene.go).
	escapedAt time.Time
	// desk is the desk's state (desk.go).
	desk deskState
	// actionDock is the dock's tool choice and what holds it.
	actionDock dockState
	// field is the field's state (field.go); onDesk whether the window
	// looks at the desk, and view how far it has gone there (scene.go).
	field       fieldState
	lane        laneState
	onDesk      bool
	view        float32
	textFocused bool
}

func New() *State {
	return &State{
		Appearance: settings.DefaultPresentation(),
		Settings:   theme.DefaultSettings(),
		Toolbar: ToolbarState{
			TypewriterEnabled: true,
		},
	}
}

func (s *State) animate(c *ui.Context) bool {
	return s != nil && components.MotionEnabled(c, s.Appearance.Motion)
}
