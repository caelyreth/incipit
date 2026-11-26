package window

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/caelyreth/neverland/internal/app/fonts"
	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/ui/onboarding"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
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
	useSystemSpelling()
	appFolders(runtime.GOOS)
	userData, _ := mygo.App.Path(mygo.PathUserData)
	logsDir, _ := mygo.App.Path(mygo.PathLogs)
	if logsDir == "" && userData != "" {
		logsDir = filepath.Join(userData, "Logs")
	}
	if path := keepLog(logsDir); path != "" {
		log.Printf("Neverland %s started, logging to %s", mygo.App.Version(), path)
	}
	root, chosen, locErr := notebookRoot(userData)
	// The app opens on the notebook once its folder is chosen: at once on
	// every launch after the first, and on the first as its walkthrough
	// ends. Whatever is pending is written before the app goes, and once
	// more should the loop stop some other way.
	var app *AppState
	closeApp := func() {
		if app == nil {
			return
		}
		app.closing.Store(true)
		if err := app.Close(); err != nil {
			log.Printf("saving the notebook: %v", err)
		}
	}
	defer closeApp()
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if e.SessionEnding || app == nil {
			return
		}
		if !app.flushForQuit() {
			e.PreventDefault()
		}
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { closeApp() })
	var lost *onboarding.Lost
	switch {
	case locErr != nil:
		if root == "" {
			root = userData
		}
		lost = lostFromLocationError(root, locErr)
	case chosen:
		if app, lost = openChosen(root, userData); app != nil {
			mygo.Theme.SetSource(themeSource(app.Product.Appearance.Appearance))
		}
	}
	mygo.App.WhenReady(func() {
		if app != nil {
			app.start()
			return
		}
		offered := root
		if lost != nil {
			offered, _ = defaultRoot()
		}
		welcome(offered, userData, lost, func(opened *AppState) { app = opened })
	})
	return mygo.App.Run()
}

// appFolders names the app's own folders, its data, cache and logs, as
// storage.FolderName does on goos: on Linux in lowercase, in the XDG
// config and cache folders, where MyGo would take the app's name as it is
// written. Elsewhere MyGo's own stand.
func appFolders(goos string) {
	if goos != "linux" {
		return
	}
	name := storage.FolderName(goos)
	if dir, err := os.UserConfigDir(); err == nil {
		data := filepath.Join(dir, name)
		if err := os.MkdirAll(data, 0o755); err != nil {
			log.Printf("making the app's data folder: %v", err)
		}
		mygo.App.SetPath(mygo.PathUserData, data)
		mygo.App.SetPath(mygo.PathLogs, filepath.Join(data, "logs"))
	}
	if dir, err := os.UserCacheDir(); err == nil {
		mygo.App.SetPath(mygo.PathCache, filepath.Join(dir, name))
	}
}

// notebookRoot is the notebook's folder and whether it has been chosen:
// NEVERLAND_HOME's, or the one recorded on the first launch; else the
// folder the walkthrough offers, not chosen yet. A corrupt or unreadable
// location record reports an error so an existing user is not silently
// dropped into fresh onboarding.
func notebookRoot(userData string) (string, bool, error) {
	if os.Getenv(storage.HomeVariable) != "" {
		root, err := storage.Root("", runtime.GOOS)
		return root, true, err
	}
	root, ok, err := storage.ChosenRoot(userData)
	if err != nil {
		log.Printf("reading where the notebook is: %v", err)
		fallback, _ := defaultRoot()
		return fallback, false, err
	}
	if ok {
		return root, true, nil
	}
	root, err = defaultRoot()
	return root, false, err
}

// defaultRoot is the folder the walkthrough offers for a notebook.
func defaultRoot() (string, error) {
	base, _ := notebookBase(runtime.GOOS)
	return storage.Root(base, runtime.GOOS)
}

func lostFromLocationError(defaultFolder string, locErr error) *onboarding.Lost {
	return &onboarding.Lost{
		Root:   defaultFolder,
		Reason: describeNotice(locErr),
	}
}

// openChosen opens the notebook recorded in root, or says why it is lost:
// a folder that holds no notebook is never made one unasked, as one moved,
// deleted or on a drive not connected would be, and one that will not open
// keeps the app from opening on nothing. NEVERLAND_HOME's folder is made
// as it is named, for development and tests.
func openChosen(root, userData string) (*AppState, *onboarding.Lost) {
	if os.Getenv(storage.HomeVariable) == "" && !storage.Holds(root) {
		return nil, &onboarding.Lost{Root: root}
	}
	app, err := openApp(root, userData)
	if err != nil {
		log.Printf("opening the notebook in %s: %v", root, err)
		return nil, &onboarding.Lost{Root: root, Reason: err.Error()}
	}
	return app, nil
}

// openApp opens the notebook in root, with the fonts kept in userData.
func openApp(root, userData string) (*AppState, error) {
	store, err := storage.Open(root)
	if err != nil {
		return nil, fmt.Errorf("opening the notebook in %s: %w", root, err)
	}
	app, err := OpenAppState(store)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	app.userDataDir = userData
	if dir, err := fonts.Dir(userData); err == nil {
		app.useFonts(openFonts(dir))
	} else {
		log.Printf("fonts folder: %v", err)
		if app.Notices != nil {
			app.Notices.Post(Notice{
				ID:          "fonts.dir",
				Kind:        NoticeWarning,
				Title:       locales.FontsFolderUnavailable(),
				Description: err.Error(),
			})
		}
	}
	app.useUpdater(userData)
	return app, nil
}

// start shows the main window, once the app is ready, and follows the
// system from then on.
func (a *AppState) start() {
	a.language = mygo.App.ChosenLocale()
	mygo.Theme.SetSource(themeSource(a.Product.Appearance.Appearance))
	a.followSystem()
	mygo.App.OnLocaleChanged(func(string) { a.localeChanged() })
	mygo.App.SetMenu(a.MainMenu())
	a.Notices.SetWake(a.wake)
	if a.finder != nil {
		a.finder.SetWake(a.wake)
	}
	if a.updater != nil {
		a.updater.Start()
	}
	a.showMain()
	mygo.App.OnSecondInstance(func([]string, string) { a.showMain() })
	// On macOS Neverland keeps running with its window closed, as Mac
	// apps do, and opens it again from the Dock; elsewhere closing it
	// quits.
	if runtime.GOOS == "darwin" {
		mygo.App.OnWindowAllClosed(func() {})
		mygo.App.OnActivate(func(visible bool) {
			if !visible {
				a.showMain()
			}
		})
	}
}

// notebookBase returns the parent directory holding Neverland's notebook: the
// XDG data folder on Linux, and the user's Documents directory elsewhere.
func notebookBase(goos string) (string, error) {
	if goos == "linux" {
		return storage.XDGDataHome()
	}
	return mygo.App.Path(mygo.PathDocuments)
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
	window.OnClose(func(e *mygo.CloseEvent) {
		if a.closing.Load() || a.quitConfirmed {
			return
		}
		if a.exportingBusy() {
			e.PreventDefault()
			a.warnExportBusy(a.exportWindow)
			return
		}
		if runtime.GOOS != "darwin" && !a.flushForQuit() {
			e.PreventDefault()
		}
	})
	a.mainWindow = window
}

// flushForQuit attempts to persist pending edits and guard against active
// exports before quitting or closing the main window on non-macOS systems.
func (a *AppState) flushForQuit() bool {
	if a == nil || a.quitConfirmed {
		return true
	}
	if a.exportingBusy() {
		a.warnExportBusy(a.exportWindow)
		return false
	}
	if err := a.FlushPending(); err != nil {
		if a.Notices != nil && a.notebook != nil {
			a.Notices.Post(saveFailure(a.notebook.saver, err, a.Notices))
		}
		a.confirmQuitWithUnsaved(err)
		return false
	}
	return true
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
// without a window, as in tests. Skips scheduling once shutdown has begun
// so background workers closing on quit cannot deadlock against OnWillQuit.
func (a *AppState) onUI(f func()) {
	if a == nil || a.closing.Load() {
		return
	}
	if a.mainWindow == nil {
		f()
		return
	}
	mygo.RunOnMain(func() {
		if a.closing.Load() {
			return
		}
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
