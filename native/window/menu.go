package window

import "github.com/egoist/mygo"

// MainMenu builds the application menu around the editor's stable command
// boundary. Edit and format roles remain MyGo's native commands; app actions
// use the same state transitions as the in-window controls.
func (a *AppState) MainMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Role: mygo.RoleAppMenu,
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				mygo.Separator(),
				{
					ID:    "settings",
					Label: "Settings...",
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.OpenSettingsWindow()
					},
				},
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
				{
					ID:          "new-page",
					Label:       "New Page",
					Accelerator: "CmdOrCtrl+N",
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.newPage()
					},
				},
				{
					ID:          "close-page",
					Label:       "Close Page",
					Accelerator: "CmdOrCtrl+W",
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.closePage()
					},
				},
			},
		},
		{Role: mygo.RoleEditMenu},
		{
			Label: "Format",
			Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleBold},
				{Role: mygo.RoleItalic},
			},
		},
		{
			Label: "View",
			Submenu: []*mygo.MenuItem{
				{
					ID:          "toggle-sidebar",
					Label:       "Toggle Pages",
					Accelerator: "CmdOrCtrl+Shift+B",
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.toggleSidebar()
					},
				},
				{
					ID:          "commands",
					Label:       "Command Palette",
					Accelerator: "CmdOrCtrl+Shift+P",
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.openCommands()
					},
				},
				mygo.Separator(),
				{Role: mygo.RoleToggleFullScreen},
			},
		},
		{Role: mygo.RoleWindowMenu},
	})
}

func (a *AppState) newPage() {
	if a == nil || a.Workspace == nil {
		return
	}
	a.parkScroll()
	if _, err := a.Workspace.NewPage(); err != nil {
		a.Error = err.Error()
		a.refreshShell()
		return
	}
	a.selectMounted()
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
		a.Shell.ToggleSidebar()
	}
}

func (a *AppState) openCommands() {
	if a == nil || a.Palette == nil {
		return
	}
	a.Palette.Open = true
	a.Palette.Reset(a.commands())
}
