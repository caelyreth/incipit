//go:build !windows

package storage

import "os"

// syncDir makes a rename or directory-entry change durable on systems with
// directory fsync.
func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		_ = dir.Close()
		return err
	}
	return dir.Close()
}
