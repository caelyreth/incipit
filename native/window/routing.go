package window

import (
	"errors"
	"fmt"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ErrUnknownCommand reports a command the window does not answer.
var ErrUnknownCommand = errors.New("window: unknown command")

type hostHandler func(*AppState)

// hostHandlers runs each command through the same shell callback a visible
// control uses.
var hostHandlers = map[keymap.CommandID]hostHandler{
	keymap.NewPage: func(a *AppState) {
		if a.Shell.NewPage != nil {
			a.Shell.NewPage()
		}
	},
	keymap.NextPage:     func(a *AppState) { a.navigatePage(true) },
	keymap.PreviousPage: func(a *AppState) { a.navigatePage(false) },
	keymap.ClosePage: func(a *AppState) {
		if a.Shell.ClosePage != nil {
			a.Shell.ClosePage()
		}
	},
	keymap.FindPage: func(a *AppState) {
		if a.Shell.FindPage != nil {
			a.Shell.FindPage()
		}
	},
	keymap.Zen: func(a *AppState) {
		a.togglePresentation(appsettings.PresentationZen, a.Shell.Zen)
	},
	keymap.Typewriter: func(a *AppState) {
		a.togglePresentation(appsettings.PresentationTypewriter, a.Shell.Typewriter)
	},
	keymap.Motion: func(a *AppState) {
		a.setPresentation(appsettings.PresentationMotion, !a.Product.Presentation.Motion)
	},
	keymap.Sidebar:     func(a *AppState) { a.toggleSidebar() },
	keymap.SheetList:   func(a *AppState) { a.toggleSheetList() },
	keymap.LibraryView: func(a *AppState) { a.showPanes(true, true) },
	keymap.SheetsView:  func(a *AppState) { a.showPanes(false, true) },
	keymap.PageView:    func(a *AppState) { a.showPanes(false, false) },
	keymap.Commands: func(a *AppState) {
		if a.Shell.OpenCommands != nil {
			a.Shell.OpenCommands()
		}
	},
	keymap.Settings: func(a *AppState) {
		if a.Shell.OpenSettings != nil {
			a.Shell.OpenSettings()
		}
	},
}

// routeWindowCommand is the common menu, palette and keyboard entry point.
// An auxiliary window answers only Close Page, which closes it, and never
// reaches back into the main window.
func (a *AppState) routeWindowCommand(window *mygo.Window, command keymap.CommandID) error {
	if a == nil {
		return fmt.Errorf("window: app state is unavailable")
	}
	handler, ok := hostHandlers[command]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
	if window != nil && a.mainWindow != nil && window != a.mainWindow {
		if command != keymap.ClosePage {
			return fmt.Errorf("%w: %s in an auxiliary window", ErrUnknownCommand, command)
		}
		if a.closeWindow != nil {
			a.closeWindow(window)
		}
		return nil
	}
	handler(a)
	return nil
}

// pressedCommands runs the commands whose keys were pressed in the main
// window. A command a menu item carries takes its key through the menu, so
// it is not heard twice.
func (a *AppState) pressedCommands(c *ui.Context) {
	for _, chord := range a.Product.Keymap.Chords() {
		command, _ := a.Product.Keymap.CommandFor(chord)
		if menuCommands[command] {
			continue
		}
		modifiers, key, ok := keymap.Native(chord, keymap.PlatformCurrent)
		if ok && c.Shortcut(modifiers, key) {
			if err := a.routeWindowCommand(a.mainWindow, command); err != nil {
				a.fail("Couldn't run the command", err)
			}
		}
	}
}
