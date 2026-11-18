package window

import (
	"errors"

	"github.com/caelyreth/neverland/app/storage"
	"github.com/egoist/mygo"
)

// The notices the notebook keeps up to date, each by its ID, so a state
// that changes shows in one toast.
const (
	saveNotice     = "notebook.save"
	backupNotice   = "notebook.backup"
	recoveryNotice = "notebook.recovery"
)

// saving is what the user was last told about saving: the messages of the
// failing write and backup, empty while they go well. The saver's
// goroutine alone reads and writes it.
type saving struct {
	err, backupErr string
}

// saver is what the notices read off the notebook's saver, and how Retry
// Now asks it to write.
type saver interface {
	Err() error
	BackupErr() error
	Flush() error
}

// watchSaving tells the user when saving starts or stops failing, and when
// backups do, as the saver tries.
func (a *AppState) watchSaving(writer *storage.Saver) {
	var told saving
	writer.Watch(func() { a.tellSaving(&told, writer) })
}

// tellSaving tells what changed since told about how writer is doing.
func (a *AppState) tellSaving(told *saving, writer saver) {
	now := saving{err: message(writer.Err()), backupErr: message(writer.BackupErr())}
	switch {
	case now.err == told.err:
	case now.err != "":
		a.Notices.Post(saveFailure(writer, writer.Err(), a.Notices))
	default:
		a.Notices.Post(Notice{ID: saveNotice, Kind: NoticeSuccess, Title: "All changes saved"})
	}
	switch {
	case now.backupErr == told.backupErr:
	case now.backupErr != "":
		a.Notices.Post(Notice{
			ID: backupNotice, Kind: NoticeWarning, Title: "Couldn't back up the notebook",
			Description: describeNotice(writer.BackupErr()),
		})
	default:
		a.Notices.Dismiss(backupNotice)
	}
	*told = now
}

// saveFailure says that changes are not on disk yet, until they are: the
// saver goes on trying, and Retry Now tries at once.
func saveFailure(writer saver, err error, center *NoticeCenter) Notice {
	return Notice{
		ID: saveNotice, Kind: NoticeError, Sticky: true,
		Title: "Changes aren't saved yet", Description: describeNotice(err),
		Action: "Retry Now",
		OnAction: func() {
			go func() {
				// A failure the same as before tells the watcher nothing new,
				// so the notice the action closed comes back from here.
				if err := writer.Flush(); err != nil && !errors.Is(err, storage.ErrClosed) {
					center.Post(saveFailure(writer, err, center))
				}
			}()
		},
	}
}

// recovered tells what opening the notebook repaired.
func recovered(store *storage.Store) (Notice, bool) {
	notice := Notice{ID: recoveryNotice, Kind: NoticeWarning, Sticky: true}
	recovery := store.Recovery
	switch {
	case recovery.Repaired() && recovery.Restored != "":
		notice.Title = "The notebook was restored from its latest backup"
		notice.Description = "It was damaged; the damaged copy is kept."
	case recovery.Repaired():
		notice.Title = "The notebook was damaged"
		notice.Description = "No backup could be read, so it starts empty; the damaged copy is kept."
	default:
		return Notice{}, false
	}
	notice.Action = "Show in Finder"
	notice.OnAction = func() { mygo.Shell.ShowItemInFolder(recovery.Damaged) }
	return notice, true
}

func message(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
