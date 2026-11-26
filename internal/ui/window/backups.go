package window

import (
	"path/filepath"
	"runtime"
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

// syncBackupsToSettings copies the cached verified backups into Settings on the
// UI/caller thread.
func (a *AppState) syncBackupsToSettings() {
	if a == nil {
		return
	}
	a.backupsMu.Lock()
	cached := a.backupsCached
	a.backupsMu.Unlock()
	a.Settings.Backups = cached
}

// refreshBackups synchronously scans verified backups under root and updates
// the cached Settings backup list.
func (a *AppState) refreshBackups(root string) {
	if a == nil || root == "" {
		return
	}
	opts := backupOptions(root)
	a.backupsMu.Lock()
	a.backupsSeq++
	a.backupsRoot = root
	a.backupsCached = opts
	a.backupsMu.Unlock()
	a.Settings.Backups = opts
}

// refreshBackupsAsync scans verified backups off the UI thread so opening
// Settings or toggling preferences never stalls on SQLite integrity checks.
func (a *AppState) refreshBackupsAsync(root string) {
	if a == nil || root == "" || a.closing.Load() {
		return
	}
	a.backupsMu.Lock()
	a.backupsSeq++
	seq := a.backupsSeq
	a.backupsMu.Unlock()
	a.backupsWg.Add(1)
	go func() {
		defer a.backupsWg.Done()
		opts := backupOptions(root)
		a.backupsMu.Lock()
		changed := a.backupsSeq == seq
		if changed {
			a.backupsRoot = root
			a.backupsCached = opts
		}
		a.backupsMu.Unlock()
		if changed && a.mainWindow != nil {
			a.onUI(func() { a.syncBackupsToSettings() })
		}
	}()
}

// bindNotebookSettings wires notebook location and backup recovery controls
// into Settings.
func (a *AppState) bindNotebookSettings() {
	if a == nil || a.notebook == nil || a.notebook.store == nil {
		return
	}
	root := a.notebook.store.Root()
	a.Settings.NotebookRoot = root
	a.Settings.PendingRoot = a.pendingRoot
	a.backupsMu.Lock()
	cached := a.backupsCached
	needRefresh := a.backupsRoot != root
	a.backupsMu.Unlock()
	a.Settings.Backups = cached
	if needRefresh {
		a.refreshBackupsAsync(root)
	}
	a.Settings.RevealNotebook = func() {
		if a.pendingRoot != "" {
			mygo.Shell.ShowItemInFolder(a.pendingRoot)
			return
		}
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
			a.pendingRoot = next
			a.Settings.PendingRoot = next
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

// performBackupRestore flushes pending edits, closes the open notebook store,
// and restores the chosen backup archive over root. If FlushPending fails,
// the notebook remains open and untouched so the caller can abort cleanly.
func (a *AppState) performBackupRestore(root, backupPath string) error {
	if err := a.FlushPending(); err != nil {
		return err
	}
	a.closing.Store(true)
	a.quitConfirmed = true
	if err := a.Close(); err != nil {
		return err
	}
	return storage.RestoreBackup(root, backupPath)
}

func (a *AppState) restoreNotebookBackup(option settings.BackupOption) {
	if a == nil || a.notebook == nil || a.notebook.store == nil || option.Path == "" {
		return
	}
	if a.exportingBusy() {
		a.warnExportBusy(a.exportWindow)
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
		mygo.RunOnMain(func() {
			if a.exportingBusy() {
				a.warnExportBusy(a.exportWindow)
				return
			}
			restoreErr := a.performBackupRestore(root, option.Path)
			if restoreErr != nil {
				_, _ = mygo.Dialog.Message(mygo.MessageOptions{
					Parent:  win,
					Type:    mygo.MessageError,
					Title:   locales.CouldntRestoreBackup(),
					Message: describeNotice(restoreErr),
				})
				if !a.closing.Load() {
					return
				}
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
				if runtime.GOOS != "darwin" && win != nil && !win.IsDestroyed() {
					win.Close()
				}
			}
		})
	}()
}
