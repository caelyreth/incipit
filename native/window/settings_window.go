package window

import (
	"fmt"
	"time"

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
	a.Settings.SetWindowActive(true)
	semantic, _ := a.colors.At(a.Shell.Settings, time.Now())
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:                "Neverland — Settings",
		Width:                980,
		Height:               740,
		MinWidth:             640,
		MinHeight:            440,
		Parent:               a.mainWindow,
		TitleBarStyle:        mygo.TitleBarHidden,
		TitleBarHeight:       40,
		TrafficLightPosition: trafficLights(),
		StateKey:             "neverland.settings",
		Content:              ui.View(a.Settings.View),
		BackgroundColor:      colorHex(semantic.Surface.Background),
	})
	window.OnFocus(func() {
		a.Settings.SetWindowActive(true)
	})
	window.OnBlur(func() {
		// A shortcut is recorded in this window: leaving it ends the
		// recording, which gives the menu its keys back.
		a.Settings.StopRecording()
		a.Settings.SetWindowActive(false)
	})
	window.OnClosed(func() {
		a.Settings.StopRecording()
		a.settingsWindow = nil
	})
	a.settingsWindow = window
	window.Focus()
}

func colorHex(color ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
}
