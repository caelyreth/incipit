package window

import (
	"context"
	"errors"
	"io/fs"
	"syscall"

	"github.com/caelyreth/incipit/internal/app/export"
	"github.com/caelyreth/incipit/internal/app/library"
	"github.com/caelyreth/incipit/internal/app/storage"
	"github.com/caelyreth/incipit/locales"
)

// failures are the failures the app has words for, the first that err is
// telling why it failed under a notice's title.
var failures = []struct {
	err  error
	says func() string
}{
	{fs.ErrPermission, locales.ErrorPermission},
	{fs.ErrNotExist, locales.ErrorNotFound},
	{syscall.ENOSPC, locales.ErrorDiskFull},
	{storage.ErrClosed, locales.ErrorNotebookClosed},
	{storage.ErrLocked, locales.ErrorNotebookLocked},
	{storage.ErrMediaFormat, locales.ErrorMediaFormat},
	{storage.ErrMediaSize, locales.ErrorMediaSize},
	{library.ErrInbox, locales.ErrorInbox},
	{export.ErrTaken, locales.ErrorExportTaken},
	{library.ErrCycle, locales.ErrorCycle},
	{library.ErrTrashed, locales.ErrorTrashed},
	{library.ErrUnknown, locales.ErrorUnknownItem},
	{context.DeadlineExceeded, locales.ErrorTimeout},
}

// describeNotice is why err failed, as a notice says it under its title:
// in the app's words where it has them, else as the system gives it, its
// first letter raised.
func describeNotice(err error) string {
	for _, f := range failures {
		if errors.Is(err, f.err) {
			return f.says()
		}
	}
	text := []rune(err.Error())
	if len(text) > 0 && text[0] >= 'a' && text[0] <= 'z' {
		text[0] -= 'a' - 'A'
	}
	return string(text)
}
