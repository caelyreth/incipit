package window

import (
	"path/filepath"
	"runtime"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/onboarding"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The walkthrough's window opens at this size and grows no smaller: each
// page is laid out for at least this much room. It fits a 13-inch
// laptop's screen.
const (
	onboardingWidth  = 1200
	onboardingHeight = 820
)

// welcome opens the first launch's walkthrough in a window of its own,
// the notebook offered in defaultRoot; with lost, a notebook recorded
// that is not there, or would not open, it begins by saying so. Finished, it opens the notebook in
// the folder chosen, records that folder in userData, applies what was
// chosen through the settings as any change in Settings is, hands the app
// to opened and gives way to the main window. Closed before it finishes,
// nothing is kept, and the next launch begins it again: a lost notebook's
// record stands until another folder is chosen.
func welcome(defaultRoot, userData string, lost *onboarding.Lost, opened func(*AppState)) {
	var window *mygo.Window
	// A second launch brings the walkthrough forward while it runs.
	second := func() {}
	state := onboarding.New(defaultRoot, onboarding.Platform{Mac: runtime.GOOS == "darwin"}, onboarding.Host{
		Holds: storage.Holds,
		ChooseFolder: func(current string, chosen func(string)) {
			go func() {
				paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
					Parent: window, Title: locales.OnboardingNotebookChoose(), DefaultPath: filepath.Dir(current),
					Directory: true, CreateDirectories: true, ButtonLabel: locales.ChooseFolder(),
				})
				if err != nil {
					_, _ = mygo.Dialog.Message(mygo.MessageOptions{
						Parent:  window,
						Type:    mygo.MessageError,
						Title:   locales.CouldntOpenDialog(),
						Message: describeNotice(err),
					})
					return
				}
				if len(paths) == 0 {
					return
				}
				mygo.RunOnMain(func() {
					chosen(paths[0])
					window.Invalidate()
				})
			}()
		},
		Backups: func(root string) []onboarding.Backup {
			raw, err := storage.ListBackups(root)
			if err != nil || len(raw) == 0 {
				return nil
			}
			out := make([]onboarding.Backup, 0, len(raw))
			for _, b := range raw {
				out = append(out, onboarding.Backup{
					Path: b.Path,
					Date: formatBackupTime(b.Taken),
					Size: components.Bytes(b.Size),
				})
			}
			return out
		},
		RestoreBackup: func(root string, backup onboarding.Backup, done func(bool, error)) {
			go func() {
				res, err := mygo.Dialog.Message(mygo.MessageOptions{
					Parent:             window,
					Type:               mygo.MessageWarning,
					Title:              locales.RestoreBackupConfirmTitle(),
					Message:            locales.RestoreBackupConfirmMessage(backup.Date, backup.Size),
					Buttons:            []string{locales.Cancel(), locales.RestoreBackupConfirmAction()},
					DefaultButton:      0,
					CancelButton:       0,
					DestructiveButtons: []int{1},
				})
				if err != nil {
					mygo.RunOnMain(func() {
						done(false, err)
						window.Invalidate()
					})
					return
				}
				if res.Button != 1 {
					mygo.RunOnMain(func() {
						done(false, nil)
						window.Invalidate()
					})
					return
				}
				restoreErr := storage.RestoreBackup(root, backup.Path)
				mygo.RunOnMain(func() {
					done(restoreErr == nil, restoreErr)
					window.Invalidate()
				})
			}()
		},
		SystemDark: mygo.Theme.IsDark,
		Appearance: func(choice appsettings.ThemeChoice) {
			mygo.Theme.SetSource(themeSource(choice))
			settings := theme.DefaultSettings()
			settings.Appearance = theme.Appearance(choice.Resolved(mygo.Theme.IsDark()))
			_ = window.SetBackgroundColor(colorHex(settings.Semantic().Surface.Background))
		},
		Finish: func(result onboarding.Result) error {
			app, err := openApp(result.Root, userData)
			if err != nil {
				return err
			}
			if err := storage.ChooseRoot(userData, result.Root); err != nil {
				_ = app.Close()
				return err
			}
			for _, change := range result.Changes {
				app.Settings.Apply(change)
			}
			second()
			opened(app)
			app.start()
			window.Close()
			return nil
		},
	})
	if lost != nil {
		state.Lost(*lost)
	}
	first := appsettings.DefaultAppearance().Appearance
	mygo.Theme.SetSource(themeSource(first))
	settings := theme.DefaultSettings()
	settings.Appearance = theme.Appearance(first)
	window = mygo.NewWindow(mygo.WindowOptions{
		Title:                locales.OnboardingTitle(),
		Width:                onboardingWidth,
		Height:               onboardingHeight,
		MinWidth:             onboardingWidth,
		MinHeight:            onboardingHeight,
		TitleBarStyle:        mygo.TitleBarHidden,
		TrafficLightPosition: trafficLights(),
		Content:              ui.View(state.View),
		BackgroundColor:      colorHex(settings.Semantic().Surface.Background),
	})
	second = mygo.App.OnSecondInstance(func([]string, string) {
		if !window.IsDestroyed() {
			window.Focus()
		}
	})
}
