//go:build !darwin

package storage

// keepLocal has nothing to do where Documents is not synced implicitly.
func keepLocal(string) {}
