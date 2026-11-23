//go:build windows

package storage

// Windows has no portable directory fsync. Atomic replacement uses
// MoveFileEx with WRITE_THROUGH instead.
func syncDir(string) error { return nil }
