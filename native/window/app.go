package window

import (
	"fmt"
	"log"

	"github.com/caelyreth/neverland/app/storage"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Run() error {
	root, err := storage.Root()
	if err != nil {
		return err
	}
	return run(root, nil, nil)
}

// run opens the notebook at root and its main window. prepare, if any,
// takes the notebook before any window opens; ready, if any, the app as
// its main window has.
func run(root string, prepare func(*AppState) error, ready func(*AppState)) error {
	if err := theme.RegisterFonts(); err != nil {
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
	if prepare != nil {
		if err := prepare(app); err != nil {
			_ = app.Close()
			return err
		}
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
			app.Shell.SetWindowActive(true)
			app.refreshShell()
		})
		window.OnBlur(func() {
			app.Shell.SetWindowActive(false)
		})
		window.OnClosed(func() {
			// Settings, and the lab, belong to the editor: they close with
			// it, and the app quits when no window is left.
			for _, other := range []*mygo.Window{app.settingsWindow, app.labWindow} {
				if other != nil && !other.IsDestroyed() {
					other.Close()
				}
			}
		})
		app.mainWindow = window
		app.Notices.SetWake(window.Invalidate)
		if app.finder != nil {
			app.finder.SetWake(window.Invalidate)
		}
		if ready != nil {
			ready(app)
		}
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
