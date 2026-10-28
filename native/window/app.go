package window

import (
	"log"

	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run() error {
	state := shell.New()
	state.Content = func(c *ui.Context) {
		// The document controller replaces this transitional content with the
		// custom editor surface once its session is mounted.
		ui.Box(c).Height(theme.NewProseStyle(state.Settings.ProseFontSize).LineHeight() * 8)
	}

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleWindowMenu},
		}))
		mygo.NewWindow(mygo.WindowOptions{
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
	})
	return mygo.App.Run()
}

func MustRun() {
	if err := Run(); err != nil {
		log.Fatal(err)
	}
}
