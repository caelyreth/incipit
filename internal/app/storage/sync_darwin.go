package storage

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// noSyncName marks a folder iCloud Drive leaves alone.
const noSyncName = ".nosync"

// fileProviderIgnore is the attribute that tells the File Provider, which
// syncs Desktop & Documents to iCloud, to skip a folder; the "#P" keeps it
// with the folder when it is copied or moved.
const fileProviderIgnore = "com.apple.fileprovider.ignore#P"

// keepLocal keeps the notebook out of iCloud's Desktop & Documents sync,
// which can upload a database mid-write or evict it to save space. Syncing
// a notebook deliberately is for native code to do, not the folder's luck.
// It is best effort: a folder that cannot be marked still works.
func keepLocal(root string) {
	marker := filepath.Join(root, noSyncName)
	if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
		if file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
			_ = file.Close()
		}
	}
	_ = unix.Setxattr(root, fileProviderIgnore, []byte{1}, 0)
}
