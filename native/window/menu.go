package window

import (
	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/egoist/mygo"
)

// MainMenu builds the application menu around the editor's stable command
// boundary. Edit and format items keep MyGo's roles, so text fields anywhere
// still cut, copy and paste; app items run the same host commands as their
// shortcuts. Every item that performs a keymap action takes its key from the
// keymap (syncMenu), so the menu shows, and a key press reaches, the action
// the keymap gives that key.
func (a *AppState) MainMenu() *mygo.Menu {
	command := func(id kernel.CommandID) func(*mygo.MenuItem, *mygo.Window) {
		// MyGo runs a click without a frame after it, so the window is
		// redrawn here, or the change would wait for the next event.
		return func(_ *mygo.MenuItem, window *mygo.Window) {
			_ = a.executeWindowCommand(window, id)
			for _, w := range []*mygo.Window{window, a.mainWindow, a.settingsWindow} {
				if w != nil && !w.IsDestroyed() {
					w.Invalidate()
				}
			}
		}
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{
			Role: mygo.RoleAppMenu,
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				mygo.Separator(),
				{ID: "settings", Label: "Settings...", Click: command(editor.SurfaceSettingsCommandID)},
				mygo.Separator(),
				{Role: mygo.RoleServices},
				mygo.Separator(),
				{Role: mygo.RoleHide},
				{Role: mygo.RoleHideOthers},
				{Role: mygo.RoleUnhide},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			},
		},
		{
			Label: "File",
			Submenu: []*mygo.MenuItem{
				{ID: "new-page", Label: "New Page", Click: command(editor.SurfaceNewPageCommandID)},
				{ID: "close-page", Label: "Close Page", Click: command(editor.SurfaceClosePageCommandID)},
			},
		},
		{
			Label: "Edit",
			Submenu: []*mygo.MenuItem{
				{ID: "undo", Role: mygo.RoleUndo},
				{ID: "redo", Role: mygo.RoleRedo},
				mygo.Separator(),
				{ID: "cut", Role: mygo.RoleCut},
				{ID: "copy", Role: mygo.RoleCopy},
				{ID: "paste", Role: mygo.RolePaste},
				{Role: mygo.RolePasteAndMatchStyle},
				{Role: mygo.RoleDelete},
				{ID: "select-all", Role: mygo.RoleSelectAll},
			},
		},
		{
			Label: "Format",
			Submenu: []*mygo.MenuItem{
				{ID: "bold", Role: mygo.RoleBold},
				{ID: "italic", Role: mygo.RoleItalic},
			},
		},
		{
			Label: "View",
			Submenu: []*mygo.MenuItem{
				{ID: "view-library", Label: "Library", Click: command(editor.SurfaceLibraryViewCommandID)},
				{ID: "view-sheets", Label: "Sheet List", Click: command(editor.SurfaceSheetsViewCommandID)},
				{ID: "view-editor", Label: "Editor Only", Click: command(editor.SurfaceEditorViewCommandID)},
				mygo.Separator(),
				{ID: "toggle-sidebar", Label: "Toggle Library", Click: command(editor.SurfaceSidebarCommandID)},
				{ID: "toggle-sheet-list", Label: "Toggle Sheet List", Click: command(editor.SurfaceSheetListCommandID)},
				mygo.Separator(),
				{ID: "commands", Label: "Command Palette", Click: command(editor.SurfaceCommandsCommandID)},
				mygo.Separator(),
				{Role: mygo.RoleToggleFullScreen},
			},
		},
		{Role: mygo.RoleWindowMenu},
	})
	a.menu = menu
	// The system's items keep the keys their roles gave them; they stand
	// down while recording, and come back from here.
	keyed := make(map[string]bool, len(keymapMenu))
	for _, entry := range keymapMenu {
		keyed[entry.id] = true
	}
	a.systemKeys = make(map[*mygo.MenuItem]string)
	eachMenuItem(menu, func(item *mygo.MenuItem) {
		if !keyed[item.ID] && item.Accelerator != "" {
			a.systemKeys[item] = item.Accelerator
		}
	})
	a.syncMenu()
	return menu
}

// executeWindowCommand applies a menu command in the window that received
// it. Editor and workspace commands belong to the main window; an auxiliary
// window closes itself instead of reaching back into the editor.
func (a *AppState) executeWindowCommand(window *mygo.Window, command kernel.CommandID) error {
	if a == nil {
		return nil
	}
	if window == nil || a.mainWindow == nil || window == a.mainWindow {
		return a.executeExternalCommand(command)
	}
	switch command {
	case editor.SurfaceClosePageCommandID:
		if a.closeWindow != nil {
			a.closeWindow(window)
		}
	}
	return nil
}

// keymapMenu pairs each menu item that performs a keymap action with it.
var keymapMenu = [...]struct {
	id      string
	command kernel.CommandID
}{
	{"settings", editor.SurfaceSettingsCommandID},
	{"new-page", editor.SurfaceNewPageCommandID},
	{"close-page", editor.SurfaceClosePageCommandID},
	{"undo", editor.HistoryUndo.ID()},
	{"redo", editor.HistoryRedo.ID()},
	{"cut", editor.SurfaceCutCommandID},
	{"copy", editor.SurfaceCopyCommandID},
	{"paste", editor.SurfacePasteCommandID},
	{"select-all", editor.TextSelectAll.ID()},
	{"bold", editor.MarkupStrong.ID()},
	{"italic", editor.MarkupEmphasis.ID()},
	{"view-library", editor.SurfaceLibraryViewCommandID},
	{"view-sheets", editor.SurfaceSheetsViewCommandID},
	{"view-editor", editor.SurfaceEditorViewCommandID},
	{"toggle-sidebar", editor.SurfaceSidebarCommandID},
	{"toggle-sheet-list", editor.SurfaceSheetListCommandID},
	{"commands", editor.SurfaceCommandsCommandID},
}

// syncMenu gives each keymap item its action's first key, or none, and the
// action's availability. While a shortcut is being recorded every item lets
// go of its key, the system's too, so the recorder hears each chord as keys:
// a disabled item would not do, as macOS validates the edit items enabled
// and still lets them take their keys.
func (a *AppState) syncMenu() {
	if a == nil || a.menu == nil || a.Store == nil {
		return
	}
	prepared := a.Store.PreparedConfiguration()
	for _, entry := range keymapMenu {
		item := a.menu.ItemByID(entry.id)
		if item == nil {
			continue
		}
		accelerator, enabled := "", false
		if prepared != nil {
			enabled = prepared.IsCommandEnabled(entry.command)
			if bindings := prepared.BindingsFor(entry.command); len(bindings) > 0 && !a.recording {
				accelerator, _ = keymap.Accelerator(bindings[0], keymap.PlatformCurrent)
			}
		}
		setAccelerator(item, accelerator)
		if item.IsEnabled() != enabled {
			item.SetEnabled(enabled)
		}
	}
	for item, accelerator := range a.systemKeys {
		if a.recording {
			accelerator = ""
		}
		setAccelerator(item, accelerator)
	}
}

func setAccelerator(item *mygo.MenuItem, accelerator string) {
	if item.Accelerator != accelerator {
		item.SetAccelerator(accelerator)
	}
}

// setRecording stands the menu's keys down while a shortcut is recorded.
func (a *AppState) setRecording(recording bool) {
	if a.recording != recording {
		a.recording = recording
		a.syncMenu()
	}
}

// reservedKey names the menu item that keeps target for itself: a key of the
// system's items, as Quit's, which no keymap action can be given, as the menu
// would always take it first.
func (a *AppState) reservedKey(target kernel.BindingTarget) (string, bool) {
	if a == nil {
		return "", false
	}
	for item, accelerator := range a.systemKeys {
		if held, ok := keymap.ParseAccelerator(accelerator, keymap.PlatformCurrent); ok && held == target {
			return item.Label, true
		}
	}
	return "", false
}

func eachMenuItem(menu *mygo.Menu, visit func(*mygo.MenuItem)) {
	eachItem(menu.Items(), visit)
}

func eachItem(items []*mygo.MenuItem, visit func(*mygo.MenuItem)) {
	for _, item := range items {
		visit(item)
		eachItem(item.Submenu, visit)
	}
}

func (a *AppState) newPage() {
	if a == nil || a.Workspace == nil {
		return
	}
	a.NewSheet(a.Shell.CurrentGroup(), shell.Last)
}

func (a *AppState) closePage() {
	if a == nil || a.Workspace == nil {
		return
	}
	a.parkScroll()
	id, ok := a.Workspace.Active()
	if !ok {
		return
	}
	if err := a.Workspace.Close(id); err != nil {
		a.Error = err.Error()
		a.refreshShell()
		return
	}
	a.selectMounted()
}

func (a *AppState) toggleSidebar() {
	if a != nil && a.Shell != nil {
		a.setPresentation(appsettings.PresentationZen, false)
		a.Shell.ToggleSidebar()
	}
}

func (a *AppState) toggleSheetList() {
	if a != nil && a.Shell != nil {
		a.setPresentation(appsettings.PresentationZen, false)
		a.Shell.ToggleSheetList()
	}
}

// showPanes shows the library and the sheet list as asked, leaving Zen
// for either.
func (a *AppState) showPanes(library, sheets bool) {
	if a == nil || a.Shell == nil {
		return
	}
	if library || sheets {
		a.setPresentation(appsettings.PresentationZen, false)
	}
	a.Shell.ShowPanes(library, sheets)
}

func (a *AppState) openCommands() {
	if a == nil || a.Palette == nil {
		return
	}
	a.Palette.Open = true
	a.Palette.Reset(a.commands())
}
