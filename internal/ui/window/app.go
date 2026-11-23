package window

import (
	"fmt"
	"log"
	"os"

	"github.com/caelyreth/neverland/internal/app/fonts"
	"github.com/caelyreth/neverland/internal/app/storage"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run() error {
	// Neverland runs once: launched again, it brings the running window
	// forward. A notebook moved elsewhere (NEVERLAND_HOME) runs apart, as
	// the notebook's own lock keeps each to one process.
	if os.Getenv(storage.HomeVariable) == "" && !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	if err := theme.RegisterFonts(); err != nil {
		return err
	}
	root, err := storage.Root()
	if err != nil {
		return err
	}
	store, err := storage.Open(root)
	if err != nil {
		return fmt.Errorf("opening the notebook in %s: %w", root, err)
	}
	app, err := OpenAppState(store)
	if err != nil {
		_ = store.Close()
		return err
	}
	if dir, err := fonts.Dir(); err == nil {
		app.useFonts(openFonts(dir))
	}
	// Whatever is pending is written before the app goes, and once more
	// should the loop stop some other way.
	defer func() {
		if err := app.Close(); err != nil {
			log.Printf("saving the notebook: %v", err)
		}
	}()
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if err := app.Close(); err != nil {
			log.Printf("saving the notebook: %v", err)
		}
	})
	mygo.Theme.SetSource(themeSource(app.Shell.Settings))

	mygo.App.WhenReady(func() {
		app.language = mygo.App.ChosenLocale()
		app.refreshSettingsData()
		mygo.App.OnLocaleChanged(func(string) { app.localeChanged() })
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
			Content:              ui.View(app.View),
			BackgroundColor:      colorHex(app.Shell.Settings.Semantic().Surface.Background),
		})
		window.OnFocus(func() {
			app.refreshShell()
		})
		window.OnBlur(func() {
			// A shortcut is recorded in the field: leaving the window ends
			// the recording, which gives the menu its keys back.
			app.Settings.StopRecording()
			window.Invalidate()
		})
		app.mainWindow = window
		mygo.App.OnSecondInstance(func([]string, string) {
			window.Restore()
			window.Focus()
		})
		app.Notices.SetWake(window.Invalidate)
		if app.finder != nil {
			app.finder.SetWake(window.Invalidate)
		}
	})
	return mygo.App.Run()
}

// trafficLights places the close button's leading edge on the chrome gutter
// and centers the controls vertically in the title bar.
func trafficLights() *mygo.Point {
	return &mygo.Point{X: int(theme.DefaultLayout.ChromeGutter), Y: 13}
}

func MustRun() {
	if err := Run(); err != nil {
		log.Fatal(err)
	}
}

// colorHex is color as a window's background takes it.
func colorHex(color ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
}
