package window

import (
	"errors"
	"fmt"
	"slices"

	"github.com/caelyreth/incipit/internal/app/keymap"
	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	"github.com/caelyreth/incipit/internal/ui/shell"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/ui"
)

// ErrUnknownCommand reports a command the window does not answer.
var ErrUnknownCommand = errors.New("window: unknown command")

type hostHandler func(*AppState)

// hostHandlers runs each command the window answers itself, as a visible
// control for it does. Each acts on the scene in front: a key never
// changes what the person cannot see (commandState says when it can).
var hostHandlers = map[keymap.CommandID]hostHandler{
	keymap.NewPage: func(a *AppState) {
		a.newSheet()
		a.Shell.ShowDesk()
	},
	keymap.NextPage:     func(a *AppState) { a.navigatePage(true) },
	keymap.PreviousPage: func(a *AppState) { a.navigatePage(false) },
	keymap.ClosePage:    func(a *AppState) { a.closeActive() },
	keymap.CloseOthers:  func(a *AppState) { a.Shell.CloseOthers() },
	keymap.SplitPage:    func(a *AppState) { a.splitActive() },
	keymap.FormatTools: func(a *AppState) {
		if el := a.pageElement(); el != nil && a.Shell.OnDesk() {
			el.ToggleSelectionTools()
		}
	},
	keymap.FindPage: func(a *AppState) { a.findPage() },
	keymap.Export: func(a *AppState) {
		if id, ok := a.Shell.ExportSubject(); ok {
			a.Export(id)
		}
	},
	keymap.FindInSheet: func(a *AppState) {
		if !a.Shell.OnDesk() {
			a.findPage()
			return
		}
		a.findInSheet()
	},
	keymap.Typewriter: func(a *AppState) {
		a.togglePresentation(appsettings.PresentationTypewriter, a.Shell.Typewriter)
	},
	keymap.Motion: func(a *AppState) {
		a.setPresentation(appsettings.PresentationMotion, !a.Product.Presentation.Motion)
	},
	keymap.Field:     func(a *AppState) { a.Shell.ToggleField() },
	keymap.Commands:     func(a *AppState) { a.Shell.SearchCommands() },
	keymap.Settings:     func(a *AppState) { a.Shell.ShowSettings() },
	keymap.Shortcuts:    func(a *AppState) { a.Shell.ShowShortcuts() },
	keymap.CheckUpdates: func(a *AppState) { a.checkForUpdates() },

	keymap.CheckSpelling:   func(a *AppState) { a.toggleOption(checkSpellingKey) },
	keymap.CheckGrammar:    func(a *AppState) { a.toggleOption(checkGrammarKey) },
	keymap.CorrectSpelling: func(a *AppState) { a.toggleOption(correctSpellingKey) },
	keymap.PredictSpelling: func(a *AppState) { a.toggleOption(predictSpellingKey) },
}

func init() {
	// The field answers its own verbs (shell.Verb).
	for _, id := range verbCommands {
		hostHandlers[id] = func(a *AppState) { a.Shell.DoVerb(string(id)) }
	}
}

// verbCommands are the commands the field answers.
var verbCommands = [...]keymap.CommandID{
	keymap.NewGroup, keymap.Duplicate, keymap.Rename, keymap.Pin, keymap.Trash, keymap.EmptyTrash,
	keymap.ViewList, keymap.Back, keymap.Forward, keymap.Home, keymap.ShowTrash,
}

// commandState is how a command stands now, as the menu bar shows it:
// what it is called, whether it can be done in the scene in front, and
// whether it is on. The keymap's label stands where the command says
// nothing of its own.
func (a *AppState) commandState(id keymap.CommandID, label string) shell.VerbState {
	sh := a.Shell
	desk := sh.OnDesk()
	open := len(sh.Open)
	state := shell.VerbState{Label: label, Enabled: true}
	declared, _ := a.Product.Keymap.Command(id)
	switch {
	case slices.Contains(verbCommands[:], id):
		state = sh.Verb(string(id))
		if declared.Scope == keymap.ScopeLibrary && desk {
			state.Enabled = false
		}
	case id == keymap.ClosePage, id == keymap.SplitPage, id == keymap.FormatTools:
		state.Enabled = desk
	case id == keymap.CloseOthers, id == keymap.NextPage, id == keymap.PreviousPage:
		state.Enabled = desk && open > 1
	case id == keymap.Field:
		state.Label = map[bool]string{true: locales.CommandShowField(), false: locales.ShowDesk()}[desk]
		state.Enabled = desk || open > 0
	case id == keymap.Export:
		_, state.Enabled = sh.ExportSubject()
		state.Enabled = state.Enabled && a.exporting == nil
	case id == keymap.Typewriter:
		state.Checked = a.Product.Presentation.Typewriter
	case id == keymap.Motion:
		state.Checked = a.Product.Presentation.Motion
	case id == keymap.CheckSpelling:
		state.Checked = a.optionOn(checkSpellingKey)
	case id == keymap.CheckGrammar:
		state.Checked = a.optionOn(checkGrammarKey)
		state.Enabled = a.optionOn(checkSpellingKey)
	case id == keymap.CorrectSpelling:
		state.Checked = a.optionOn(correctSpellingKey)
		state.Enabled = a.optionOn(checkSpellingKey)
	case id == keymap.PredictSpelling:
		state.Checked = a.optionOn(predictSpellingKey)
		state.Enabled = a.optionOn(checkSpellingKey)
	}
	if a.exportWindow != nil {
		state.Enabled = false
	}
	return state
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
		for _, command := range km.CommandsFor(chord) {
			if declared, _ := km.Command(command); a.menuItems[command] != nil || declared.Scope == keymap.ScopeEditing {
				continue
			}
			modifiers, key, ok := keymap.Native(chord, keymap.PlatformCurrent)
			if ok {
				a.shortcuts = append(a.shortcuts, globalShortcut{modifiers: modifiers, key: key, command: command})
			}
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
		if !a.commandState(sc.command, "").Enabled {
			continue
		}
		if c.Shortcut(sc.modifiers, sc.key) {
			if err := a.routeCommand(sc.command); err != nil {
				a.fail(locales.CouldntRunCommand(), err)
			}
		}
	}
}
