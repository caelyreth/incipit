package window

import (
	"fmt"
	"log"
	"os"
	"runtime"

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
	documents, _ := mygo.App.Path(mygo.PathDocuments)
	root, err := storage.Root(documents)
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
	userData, _ := mygo.App.Path(mygo.PathUserData)
	if dir, err := fonts.Dir(userData); err == nil {
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
		app.Notices.SetWake(app.wake)
		if app.finder != nil {
			app.finder.SetWake(app.wake)
		}
		app.showMain()
		mygo.App.OnSecondInstance(func([]string, string) { app.showMain() })
		// On macOS Neverland keeps running with its window closed, as Mac
		// apps do, and opens it again from the Dock; elsewhere closing it
		// quits.
		if runtime.GOOS == "darwin" {
			mygo.App.OnWindowAllClosed(func() {})
			mygo.App.OnActivate(func(visible bool) {
				if !visible {
					app.showMain()
				}
			})
		}
	})
	return mygo.App.Run()
}

// showMain brings the main window forward, opening it again when it was
// closed.
func (a *AppState) showMain() {
	if w := a.mainWindow; w != nil && !w.IsDestroyed() {
		w.Restore()
		w.Focus()
		return
	}
	window := mygo.NewWindow(mygo.WindowOptions{
		Title:                "Neverland",
		Width:                1260,
		Height:               850,
		MinWidth:             760,
		MinHeight:            420,
		TitleBarStyle:        mygo.TitleBarHidden,
		TrafficLightPosition: trafficLights(),
		StateKey:             "neverland.main",
		Content:              ui.View(a.View),
		BackgroundColor:      colorHex(a.Shell.Settings.Semantic().Surface.Background),
	})
	window.OnFocus(func() {
		a.refreshShell()
	})
	window.OnBlur(func() {
		// A shortcut is recorded in the field: leaving the window ends
		// the recording, which gives the menu its keys back.
		a.Settings.StopRecording()
		window.Invalidate()
	})
	a.mainWindow = window
}

// redraw asks the app's open windows for a frame built anew. Main thread
// only: wake asks it from anywhere.
func (a *AppState) redraw() {
	for _, w := range []*mygo.Window{a.mainWindow, a.exportWindow} {
		if w != nil && !w.IsDestroyed() {
			w.Invalidate()
		}
	}
}

func (a *AppState) wake() { mygo.RunOnMain(a.redraw) }

// onUI runs f on the UI thread and redraws the windows open then, at once
// without a window, as in tests.
func (a *AppState) onUI(f func()) {
	if a.mainWindow == nil {
		f()
		return
	}
	mygo.RunOnMain(func() {
		f()
		a.redraw()
	})
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
