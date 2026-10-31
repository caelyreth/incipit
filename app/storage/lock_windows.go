package storage

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock takes the file's exclusive lock without waiting. The system drops
// it when the process ends, however it ends.
func tryLock(file *os.File) error {
	overlapped := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, overlapped,
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrLocked
	}
	return err
}

func unlock(file *os.File) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, new(windows.Overlapped))
}
