package window

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// OpenSettingsWindow shows one app-owned settings window. Reopening while it
// exists brings the same window forward rather than rebuilding its state.
func (a *AppState) OpenSettingsWindow() {
	if a == nil {
		return
	}
	if a.settingsWindow != nil && !a.settingsWindow.IsDestroyed() {
		a.settingsWindow.Show()
		a.settingsWindow.Focus()
		return
	}
	a.refreshSettingsData()
	a.Settings.Close = func() {
		if a.settingsWindow != nil && !a.settingsWindow.IsDestroyed() {
			a.settingsWindow.Close()
		}
	}
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:           "Shoreward Settings",
		Width:           900,
		Height:          650,
		MinWidth:        720,
		MinHeight:       480,
		Parent:          a.mainWindow,
		StateKey:        "shoreward.settings",
		Content:         ui.View(a.Settings.View),
		BackgroundColor: "#f5f1e8",
	})
	window.OnClosed(func() {
		a.settingsWindow = nil
		a.Settings.Close = nil
	})
	a.settingsWindow = window
	window.Focus()
}
