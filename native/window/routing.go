package window

import (
	"errors"
	"fmt"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

// ErrUnknownCommand reports a command the window does not answer.
var ErrUnknownCommand = errors.New("window: unknown command")

type hostHandler func(*AppState)

// hostHandlers runs each command the window answers itself, as a visible
// control for it does.
var hostHandlers = map[keymap.CommandID]hostHandler{
	keymap.NewPage:      func(a *AppState) { a.newSheet() },
	keymap.NextPage:     func(a *AppState) { a.navigatePage(true) },
	keymap.PreviousPage: func(a *AppState) { a.navigatePage(false) },
	keymap.ClosePage:    func(a *AppState) { a.closeActive() },
	keymap.FindPage:     func(a *AppState) { a.findPage() },
	keymap.FindInSheet:  func(a *AppState) { a.findInSheet() },
	keymap.Zen: func(a *AppState) {
		a.togglePresentation(appsettings.PresentationZen, a.Shell.Zen)
	},
	keymap.Typewriter: func(a *AppState) {
		a.togglePresentation(appsettings.PresentationTypewriter, a.Shell.Typewriter)
	},
	keymap.Motion: func(a *AppState) {
		a.setPresentation(appsettings.PresentationMotion, !a.Product.Presentation.Motion)
	},
	keymap.Field:    func(a *AppState) { a.Shell.ToggleField() },
	keymap.Commands: func(a *AppState) { a.Shell.SearchCommands() },
	keymap.Settings: func(a *AppState) { a.Shell.ShowSettings() },
}

// routeCommand is the common menu, pill and keyboard entry point.
func (a *AppState) routeCommand(command keymap.CommandID) error {
	if a == nil {
		return fmt.Errorf("window: app state is unavailable")
	}
	handler, ok := hostHandlers[command]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
	handler(a)
	return nil
}

type globalShortcut struct {
	modifiers ui.Modifiers
	key       ui.Key
	command   keymap.CommandID
}

func (a *AppState) windowShortcuts() []globalShortcut {
	km := a.Product.Keymap
	if km != nil && km == a.shortcutMap {
		return a.shortcuts
	}
	a.shortcutMap = km
	a.shortcuts = a.shortcuts[:0]
	if km == nil {
		return nil
	}
	for _, chord := range km.Chords() {
		command, _ := km.CommandFor(chord)
		if declared, _ := km.Command(command); menuCommands[command] || declared.Scope == keymap.ScopeEditing {
			continue
		}
		modifiers, key, ok := keymap.Native(chord, keymap.PlatformCurrent)
		if ok {
			a.shortcuts = append(a.shortcuts, globalShortcut{modifiers: modifiers, key: key, command: command})
		}
	}
	return a.shortcuts
}

// pressedCommands runs the commands whose keys were pressed in the main
// window. A command a menu item carries takes its key through the menu, so
// it is not heard twice; an editing command is the page's, which hears
// its key itself.
func (a *AppState) pressedCommands(c *ui.Context) {
	for _, sc := range a.windowShortcuts() {
		if c.Shortcut(sc.modifiers, sc.key) {
			if err := a.routeCommand(sc.command); err != nil {
				a.fail("Couldn't run the command", err)
			}
		}
	}
}
