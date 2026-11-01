package window

import (
	"fmt"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo"
)

var commandAliases = map[kernel.CommandID]kernel.CommandID{
	"workspace.new": editor.SurfaceNewPageCommandID,
}

type hostHandler func(*AppState) error

// hostHandlers keeps host command behavior identified by command, not by the
// surface group or a shared raw-ID switch. Each handler still uses the same
// shell callback a visible control uses.
var hostHandlers = map[kernel.CommandID]hostHandler{
	editor.SurfaceNewPageCommandID: func(a *AppState) error {
		if a.Shell.NewPage != nil {
			a.Shell.NewPage()
		} else {
			a.newPage()
		}
		return nil
	},
	editor.SurfaceNextPageCommandID: func(a *AppState) error {
		a.navigatePage(true)
		return nil
	},
	editor.SurfacePreviousPageCommandID: func(a *AppState) error {
		a.navigatePage(false)
		return nil
	},
	editor.SurfaceClosePageCommandID: func(a *AppState) error {
		if a.Shell.ClosePage != nil {
			a.Shell.ClosePage()
		} else {
			a.closePage()
		}
		return nil
	},
	editor.SurfaceZenCommandID: func(a *AppState) error {
		a.togglePresentation(appsettings.PresentationZen, a.Shell.Zen)
		return nil
	},
	editor.SurfaceTypewriterCommandID: func(a *AppState) error {
		a.togglePresentation(appsettings.PresentationTypewriter, a.Shell.Typewriter)
		return nil
	},
	editor.SurfaceMotionCommandID: func(a *AppState) error {
		a.setPresentation(
			appsettings.PresentationMotion,
			!a.Product.Presentation.Motion,
		)
		return nil
	},
	editor.SurfaceCJKCommandID: func(a *AppState) error {
		configuration := a.Session.Document().Runtime().PolicyConfiguration()
		on := !configuration.IsEnabled(editor.CJKPolicyID)
		if a.Shell.ToggleCJK != nil {
			a.Shell.ToggleCJK(on)
		} else {
			a.setPolicy(editor.CJKPolicyID, on)
		}
		return nil
	},
	editor.SurfaceInsertImageCommandID: func(a *AppState) error {
		a.chooseImages()
		return nil
	},
	editor.SurfaceCommandsCommandID: func(a *AppState) error {
		if a.Shell.OpenCommands != nil {
			a.Shell.OpenCommands()
		} else {
			a.openCommands()
		}
		return nil
	},
	editor.SurfaceFindPageCommandID: func(a *AppState) error {
		if a.Shell.FindPage != nil {
			a.Shell.FindPage()
		} else {
			a.findPage()
		}
		return nil
	},
	editor.SurfaceSidebarCommandID: func(a *AppState) error {
		a.toggleSidebar()
		return nil
	},
	editor.SurfaceSheetListCommandID: func(a *AppState) error {
		a.toggleSheetList()
		return nil
	},
	editor.SurfaceLibraryViewCommandID: func(a *AppState) error {
		a.showPanes(true, true)
		return nil
	},
	editor.SurfaceSheetsViewCommandID: func(a *AppState) error {
		a.showPanes(false, true)
		return nil
	},
	editor.SurfaceEditorViewCommandID: func(a *AppState) error {
		a.showPanes(false, false)
		return nil
	},
	editor.SurfaceSettingsCommandID: func(a *AppState) error {
		if a.Shell.OpenSettings != nil {
			a.Shell.OpenSettings()
		} else {
			a.OpenSettingsWindow()
		}
		return nil
	},
}

func canonicalCommandID(command kernel.CommandID) kernel.CommandID {
	if canonical, ok := commandAliases[command]; ok {
		return canonical
	}
	return command
}

func (a *AppState) resolveCommand(command kernel.CommandID) (keymap.Action, error) {
	action, ok := keymap.ResolveCommand(canonicalCommandID(command))
	if !ok {
		return keymap.Action{}, fmt.Errorf("%w: %s", keymap.ErrUnsupportedCommand, command)
	}
	return action, nil
}

// commandEnabled joins policy enablement with the command's contextual
// requirements. Selection is deliberately not part of this decision.
func (a *AppState) commandEnabled(action keymap.Action) bool {
	if a == nil || a.Store == nil {
		return false
	}
	return a.commandEnabledWith(a.Store.PreparedConfiguration(), action)
}

func (a *AppState) commandEnabledWith(prepared *kernel.PreparedPolicyConfiguration, action keymap.Action) bool {
	if prepared == nil || !prepared.IsCommandEnabled(action.ID) {
		return false
	}
	document := a.Session != nil && a.Session.Document() != nil
	table := document && a.Session.Document().Session().InTable()
	return action.Requirements.Satisfied(document, table)
}

// routeWindowCommand is the common menu, palette, toolbar and keyboard bridge
// entry point. Auxiliary windows retain their own close behavior and never
// reach back into the main document.
func (a *AppState) routeWindowCommand(window *mygo.Window, command kernel.CommandID) error {
	if a == nil {
		return fmt.Errorf("window: app state is unavailable")
	}
	action, err := a.resolveCommand(command)
	if err != nil {
		return err
	}
	if window != nil && a.mainWindow != nil && window != a.mainWindow {
		if action.ID != editor.SurfaceClosePageCommandID {
			return fmt.Errorf("%w: %s in an auxiliary window", keymap.ErrUnsupportedCommand, command)
		}
		if a.closeWindow != nil {
			a.closeWindow(window)
		}
		return nil
	}
	if !a.commandEnabled(action) {
		return fmt.Errorf("%w: %s", keymap.ErrCommandUnavailable, command)
	}
	switch action.Owner {
	case editor.CommandOwnerEditor:
		if a.Bridge == nil {
			return fmt.Errorf("%w: %s without a mounted document", keymap.ErrCommandUnavailable, command)
		}
		return a.Bridge.Execute(action)
	case editor.CommandOwnerHost:
		handler, ok := hostHandlers[action.ID]
		if !ok {
			return fmt.Errorf("%w: %s", keymap.ErrUnsupportedCommand, command)
		}
		return handler(a)
	default:
		return fmt.Errorf("%w: %s has no owner", keymap.ErrUnsupportedCommand, command)
	}
}

// executeHostCommand runs only host-owned actions, as documentview's external
// surface callback expects. Unknown and editor-owned identities are errors.
func (a *AppState) executeHostCommand(command kernel.CommandID) error {
	action, err := a.resolveCommand(command)
	if err != nil {
		return err
	}
	if action.Owner != editor.CommandOwnerHost {
		return fmt.Errorf("%w: %s is not host-owned", keymap.ErrUnsupportedCommand, command)
	}
	if !a.commandEnabled(action) {
		return fmt.Errorf("%w: %s", keymap.ErrCommandUnavailable, command)
	}
	handler, ok := hostHandlers[action.ID]
	if !ok {
		return fmt.Errorf("%w: %s", keymap.ErrUnsupportedCommand, command)
	}
	return handler(a)
}
