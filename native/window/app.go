package window

import (
	"log"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run() error {
	if err := theme.RegisterFonts(); err != nil {
		return err
	}
	app, err := NewAppState()
	if err != nil {
		return err
	}
	state := app.Shell

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(app.MainMenu())
		window := mygo.NewWindow(mygo.WindowOptions{
			Title:                "Neverland",
			Width:                1260,
			Height:               850,
			MinWidth:             760,
			MinHeight:            420,
			TitleBarStyle:        mygo.TitleBarHidden,
			TrafficLightPosition: trafficLights(),
			StateKey:             "neverland.main",
			Content:              ui.View(state.View),
		})
		window.OnFocus(func() {
			app.Shell.SetWindowActive(true)
			app.Error = ""
			app.refreshShell()
		})
		window.OnBlur(func() {
			app.Shell.SetWindowActive(false)
		})
		app.mainWindow = window
	})
	return mygo.App.Run()
}

// trafficLights places the close button's leading edge on the chrome gutter,
// aligned with the sidebar and settings search fields below it, and centers
// the controls vertically in the title bar.
func trafficLights() *mygo.Point {
	return &mygo.Point{X: int(theme.DefaultLayout.ChromeGutter), Y: 13}
}

func MustRun() {
	if err := Run(); err != nil {
		log.Fatal(err)
	}
}
