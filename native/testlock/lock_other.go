//go:build !unix && !windows

package testlock

import "os"

// lockFile is a process-local fallback for platforms without an implemented
// advisory file lock. Native test packages target unix and windows.
func lockFile(*os.File) error {
	return nil
}
