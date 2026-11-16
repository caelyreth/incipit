package window

import (
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo"
)

// MainMenu builds the application menu. Edit items keep MyGo's roles, so
// text fields anywhere still cut, copy and paste; app items run the same
// commands as their shortcuts. Every item that runs a keymap command takes
// its key from the keymap (syncMenu), so the menu shows, and a key press
// reaches, the command the keymap gives that key.
func (a *AppState) MainMenu() *mygo.Menu {
	command := func(id keymap.CommandID) func(*mygo.MenuItem, *mygo.Window) {
		// MyGo runs a click without a frame after it, so the window is
		// redrawn here, or the change would wait for the next event.
		return func(_ *mygo.MenuItem, window *mygo.Window) {
			if err := a.routeWindowCommand(window, id); err != nil {
				a.fail("Couldn't run the command", err)
			}
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
				{ID: "settings", Label: "Settings...", Click: command(keymap.Settings)},
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
				{ID: "new-page", Label: "New Page", Click: command(keymap.NewPage)},
				{ID: "close-page", Label: "Close Sheet", Click: command(keymap.ClosePage)},
			},
		},
		{
			Label: "Edit",
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleUndo},
				{Role: mygo.RoleRedo},
				mygo.Separator(),
				{Role: mygo.RoleCut},
				{Role: mygo.RoleCopy},
				{Role: mygo.RolePaste},
				{Role: mygo.RolePasteAndMatchStyle},
				{Role: mygo.RoleDelete},
				{Role: mygo.RoleSelectAll},
				mygo.Separator(),
				{ID: "insert-picture", Label: "Insert Picture…", Click: func(*mygo.MenuItem, *mygo.Window) {
					if el := a.pageElement(); el != nil {
						el.Editor.Do(vocab.Command{ID: "figures.insert"}, time.Now())
					}
				}},
			},
		},
		{
			Label: "View",
			Submenu: []*mygo.MenuItem{
				{ID: "view-library", Label: "Library", Click: command(keymap.LibraryView)},
				{ID: "view-sheets", Label: "Sheet List", Click: command(keymap.SheetsView)},
				{ID: "view-page", Label: "Page Only", Click: command(keymap.PageView)},
				{ID: "view-field", Label: "Toggle Field", Click: command(keymap.Field)},
				{ID: "view-glance", Label: "Glance at Field", Click: command(keymap.Glance)},
				mygo.Separator(),
				{ID: "toggle-sidebar", Label: "Toggle Library", Click: command(keymap.Sidebar)},
				{ID: "toggle-sheet-list", Label: "Toggle Sheet List", Click: command(keymap.SheetList)},
				mygo.Separator(),
				{ID: "commands", Label: "Command Palette", Click: command(keymap.Commands)},
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

// keymapMenu pairs each menu item that runs a keymap command with it.
var keymapMenu = [...]struct {
	id      string
	command keymap.CommandID
}{
	{"settings", keymap.Settings},
	{"new-page", keymap.NewPage},
	{"close-page", keymap.ClosePage},
	{"view-library", keymap.LibraryView},
	{"view-sheets", keymap.SheetsView},
	{"view-page", keymap.PageView},
	{"view-field", keymap.Field},
	{"view-glance", keymap.Glance},
	{"toggle-sidebar", keymap.Sidebar},
	{"toggle-sheet-list", keymap.SheetList},
	{"commands", keymap.Commands},
}

// menuCommands are the commands a menu item carries, whose keys the menu
// takes.
var menuCommands = func() map[keymap.CommandID]bool {
	commands := make(map[keymap.CommandID]bool, len(keymapMenu))
	for _, entry := range keymapMenu {
		commands[entry.command] = true
	}
	return commands
}()

// syncMenu gives each keymap item its command's key, or none. While a
// shortcut is being recorded every item lets
// go of its key, the system's too, so the recorder hears each chord as keys:
// a disabled item would not do, as macOS validates the edit items enabled
// and still lets them take their keys.
func (a *AppState) syncMenu() {
	if a == nil || a.menu == nil {
		return
	}
	for _, entry := range keymapMenu {
		item := a.menu.ItemByID(entry.id)
		if item == nil {
			continue
		}
		accelerator := ""
		if chord, ok := a.Product.Keymap.ChordFor(entry.command); ok && !a.recording {
			accelerator, _ = keymap.Accelerator(chord, keymap.PlatformCurrent)
		}
		setAccelerator(item, accelerator)
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

// reservedKey names the menu item that keeps chord for itself: a key of the
// system's items, as Quit's, which no keymap command can be given, as the
// menu would always take it first.
func (a *AppState) reservedKey(chord keymap.Chord) (string, bool) {
	if a == nil {
		return "", false
	}
	for item, accelerator := range a.systemKeys {
		if held, ok := keymap.ParseAccelerator(accelerator, keymap.PlatformCurrent); ok && held == chord {
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
