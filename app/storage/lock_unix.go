//go:build !windows

package storage

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes the file's exclusive advisory lock without waiting. The
// system drops it when the process ends, however it ends, so a crash never
// leaves the notebook locked.
func tryLock(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return ErrLocked
	}
	return err
}

func unlock(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
