package shell

import (
	"time"

	"github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Command is one of the app's commands: what it is called, the group it
// is listed in, and the caps of its key, if it has one.
type Command struct {
	ID, Label, Group string
	Binding          string
	Keys             []string
}

// Page is an open sheet.
type Page struct {
	ID    string
	Title string
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

// FormatState is one formatting action's state: whether it applies at the
// selection or caret, and whether the engine preview says it can.
type FormatState struct {
	On      bool
	Enabled bool
}

// FormattingState is the formatting the floating toolbar shows from the
// open page's previews.
type FormattingState struct {
	Strong, Emphasis, Code, Strike FormatState
}

// ToolbarState is the writing modes and formatting the floating toolbar
// switches, and their current shortcuts.
type ToolbarState struct {
	Typewriter        bool
	Zen               bool
	TypewriterEnabled bool
	ZenEnabled        bool
	Formatting        FormattingState
	Bindings          ToolbarBindings
}

// ToolbarBindings carries the current shortcut text for action tooltips.
type ToolbarBindings struct {
	Typewriter string
	Zen        string
	Strong     string
	Emphasis   string
	Code       string
	Strike     string
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
	// Open are the open sheets, in their order on the desk; ActivePage is
	// the one in front.
	Open       []Page
	ActivePage string
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
	WindowActive  bool

	// Sheet draws an open sheet's page, as frame sets it.
	Sheet        func(c *ui.Context, id string, frame SheetFrame)
	SelectPage   func(id string)
	ClosePageID  func(id string)
	OpenSettings func()
	// Commands are the app's commands, as the pill lists them written
	// after ">"; RunCommand runs one.
	Commands   func() []Command
	RunCommand func(id string)
	// Find runs a sheet's find on the page in front; FindStep goes to the
	// next match or the one before; EndFind ends it.
	Find       func(query string)
	FindStep   func(forward bool)
	EndFind    func()
	Typewriter func(bool)
	Zen        func(bool)
	Strong     func()
	Emphasis   func()
	Code       func()
	Strike     func()

	lastChrome float32
	// armedKey & armedAt govern two-click destructive confirmation.
	armedKey string
	armedAt  time.Time
	// escapedAt is when Escape was last pressed on the desk (scene.go).
	escapedAt time.Time
	// desk is the desk's state (desk.go).
	desk deskState
	// dockAt is the groups the writing actions were last built for, and
	// dockMoving until when they go to where those put them.
	dockAt     string
	dockMoving time.Time
	// field is the field's state (field.go); onDesk whether the window
	// looks at the desk, and view how far it has gone there (scene.go).
	field  fieldState
	lane   laneState
	onDesk bool
	view   float32
}

func New() *State {
	return &State{
		Appearance:   settings.DefaultPresentation(),
		Settings:     theme.DefaultSettings(),
		WindowActive: true,
		Toolbar: ToolbarState{
			TypewriterEnabled: true,
			ZenEnabled:        true,
		},
	}
}

func (s *State) SetWindowActive(active bool) {
	if s != nil {
		s.WindowActive = active
	}
}

func (s *State) animate(c *ui.Context) bool {
	return s != nil && components.MotionEnabled(c, s.Appearance.Motion, s.WindowActive)
}
