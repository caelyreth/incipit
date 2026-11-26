package window

import (
	"path/filepath"
	"time"

	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/settings"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
)

const locationNotice = "notebook.location"

// formatBackupTime formats a backup timestamp in the active UI locale.
func formatBackupTime(t time.Time) string {
	return i18n.FormatDate(i18n.Locale(), t.Local(), i18n.DateOptions{
		Date: i18n.MediumStyle,
		Time: i18n.ShortStyle,
	})
}

// backupOptions returns all verified backups under root formatted for UI.
func backupOptions(root string) []settings.BackupOption {
	raw, err := storage.ListBackups(root)
	if err != nil || len(raw) == 0 {
		return nil
	}
	out := make([]settings.BackupOption, 0, len(raw))
	for _, b := range raw {
		out = append(out, settings.BackupOption{
			Path: b.Path,
			Date: formatBackupTime(b.Taken),
			Size: components.Bytes(b.Size),
		})
	}
	return out
}

// bindNotebookSettings wires notebook location and backup recovery controls
// into Settings.
func (a *AppState) bindNotebookSettings() {
	if a == nil || a.notebook == nil || a.notebook.store == nil {
		return
	}
	root := a.notebook.store.Root()
	a.Settings.NotebookRoot = root
	a.Settings.Backups = backupOptions(root)
	a.Settings.RevealNotebook = func() {
		if a.notebook != nil && a.notebook.store != nil {
			mygo.Shell.ShowItemInFolder(a.notebook.store.Root())
		}
	}
	a.Settings.ChangeNotebook = a.changeNotebookFolder
	a.Settings.RestoreBackup = a.restoreNotebookBackup
}

func (a *AppState) changeNotebookFolder() {
	if a == nil || a.notebook == nil || a.notebook.store == nil || a.userDataDir == "" {
		return
	}
	current := a.notebook.store.Root()
	userData := a.userDataDir
	win := a.mainWindow
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            win,
			Title:             locales.OnboardingNotebookChoose(),
			DefaultPath:       filepath.Dir(current),
			Directory:         true,
			CreateDirectories: true,
			ButtonLabel:       locales.ChooseFolder(),
		})
		if err != nil {
			a.fail(locales.CouldntOpenDialog(), err)
			return
		}
		if len(paths) == 0 {
			return
		}
		next := filepath.Clean(paths[0])
		if next == current {
			return
		}
		if err := storage.ChooseRoot(userData, next); err != nil {
			a.fail(locales.NotebookLocation(), err)
			return
		}
		a.onUI(func() {
			a.Settings.NotebookRoot = next
			a.Notices.Post(Notice{
				ID:          locationNotice,
				Kind:        NoticeInfo,
				Sticky:      true,
				Title:       locales.NotebookChangedTitle(),
				Description: locales.NotebookChangedDescription(),
				Action:      locales.UpdatesButtonRestart(),
				OnAction:    func() { mygo.App.Relaunch() },
			})
		})
	}()
}

func (a *AppState) restoreNotebookBackup(option settings.BackupOption) {
	if a == nil || a.notebook == nil || a.notebook.store == nil || option.Path == "" {
		return
	}
	root := a.notebook.store.Root()
	win := a.mainWindow
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             win,
			Type:               mygo.MessageWarning,
			Title:              locales.RestoreBackupConfirmTitle(),
			Message:            locales.RestoreBackupConfirmMessage(option.Date, option.Size),
			Buttons:            []string{locales.Cancel(), locales.RestoreBackupConfirmAction()},
			DefaultButton:      0,
			CancelButton:       0,
			DestructiveButtons: []int{1},
		})
		if err != nil {
			a.fail(locales.CouldntOpenDialog(), err)
			return
		}
		if res.Button != 1 {
			return
		}
		a.onUI(func() {
			a.quitConfirmed = true
			if err := a.Close(); err != nil {
				_, _ = mygo.Dialog.Message(mygo.MessageOptions{
					Parent:  win,
					Type:    mygo.MessageError,
					Title:   locales.CouldntRestoreBackup(),
					Message: describeNotice(err),
				})
				mygo.App.Relaunch()
				return
			}
			if err := storage.RestoreBackup(root, option.Path); err != nil {
				_, _ = mygo.Dialog.Message(mygo.MessageOptions{
					Parent:  win,
					Type:    mygo.MessageError,
					Title:   locales.CouldntRestoreBackup(),
					Message: describeNotice(err),
				})
			}
			mygo.App.Relaunch()
		})
	}()
}

// confirmQuitWithUnsaved asks whether to quit without saving when flushing
// pending edits fails during application quit.
func (a *AppState) confirmQuitWithUnsaved(err error) {
	if a == nil || a.quitAsking {
		return
	}
	a.quitAsking = true
	win := a.mainWindow
	reason := describeNotice(err)
	go func() {
		res, dialogErr := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             win,
			Type:               mygo.MessageWarning,
			Title:              locales.UnsavedOnQuitTitle(),
			Message:            locales.UnsavedOnQuitMessage(reason),
			Buttons:            []string{locales.KeepOpen(), locales.QuitWithoutSaving()},
			DefaultButton:      0,
			CancelButton:       0,
			DestructiveButtons: []int{1},
		})
		a.onUI(func() {
			a.quitAsking = false
			if dialogErr == nil && res.Button == 1 {
				a.quitConfirmed = true
				mygo.App.Quit()
			}
		})
	}()
}
