package window

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run() error {
	app, err := NewAppState()
	if err != nil {
		return err
	}
	state := app.Shell

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleWindowMenu},
		}))
		window := mygo.NewWindow(mygo.WindowOptions{
			Title:                "Shoreward",
			Width:                1260,
			Height:               850,
			MinWidth:             760,
			MinHeight:            420,
			TitleBarStyle:        mygo.TitleBarHidden,
			TrafficLightPosition: &mygo.Point{X: 9, Y: 13},
			StateKey:             "shoreward.main",
			Content:              ui.View(state.View),
		})
		window.OnFocus(func() {
			app.Error = ""
			app.refreshShell()
		})
	})
	return mygo.App.Run()
}

func MustRun() {
	if err := Run(); err != nil {
		log.Fatal(err)
	}
}
