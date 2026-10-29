package testlock

import (
	"os"
	"path/filepath"
	"sync"
)

var once struct {
	sync.Once
	file *os.File
	err  error
}

// Acquire serializes test binaries across packages that use MyGo Tester.
// It is intentionally idempotent within one binary.
func Acquire() error {
	once.Do(func() {
		path := filepath.Join(os.TempDir(), "neverland-mygo-tester.lock")
		once.file, once.err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if once.err != nil {
			return
		}
		once.err = lockFile(once.file)
	})
	return once.err
}
