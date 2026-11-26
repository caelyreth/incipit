package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// FolderName is the name of Neverland's own folders on goos: the
// notebook's inside the base directory (Documents on macOS and Windows, the
// XDG data folder on Linux), and the app's data, cache and logs folders.
// Linux names them in lowercase, as its folders are named.
func FolderName(goos string) string {
	if goos == "linux" {
		return "neverland"
	}
	return "Neverland"
}

// HomeVariable names the environment variable that moves the notebook
// elsewhere, for development and tests.
const HomeVariable = "NEVERLAND_HOME"

// XDGDataHome returns the user's XDG data directory on Linux:
// XDG_DATA_HOME when set to an absolute path, else ~/.local/share.
func XDGDataHome() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return filepath.Clean(dir), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("storage: the home folder could not be found")
	}
	return filepath.Join(home, ".local", "share"), nil
}

// Root is the folder that holds the notebook: NEVERLAND_HOME when set, or
// a folder named FolderName(goos) inside base (the user's Documents on
// macOS and Windows, or the XDG data folder on Linux).
func Root(base, goos string) (string, error) {
	if home := os.Getenv(HomeVariable); home != "" {
		return filepath.Abs(home)
	}
	if base == "" {
		return "", errors.New("storage: the Documents or data folder could not be found")
	}
	return filepath.Join(base, FolderName(goos)), nil
}

// locationName is the record, in the app's own data folder, of the folder
// the person chose for the notebook. It lives apart from the notebook, as
// it is what finds it; until it is written, the app has not been set up.
const locationName = "notebook-location.json"

// locationVersion is the record's format.
const locationVersion = 1

type location struct {
	Version int    `json:"version"`
	Root    string `json:"root"`
}

// ChosenRoot returns the notebook folder recorded in userData, and false
// when none has been chosen yet: the first launch.
func ChosenRoot(userData string) (string, bool, error) {
	data, err := os.ReadFile(filepath.Join(userData, locationName))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var record location
	if err := json.Unmarshal(data, &record); err != nil {
		return "", false, fmt.Errorf("storage: notebook location: %w", err)
	}
	if record.Version != locationVersion || !filepath.IsAbs(record.Root) {
		return "", false, fmt.Errorf("storage: notebook location %q of version %d is not one this build reads", record.Root, record.Version)
	}
	return record.Root, true, nil
}

// ChooseRoot records root, an absolute folder, as the notebook's in
// userData, replacing the record whole.
func ChooseRoot(userData, root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("storage: notebook location %q is not absolute", root)
	}
	data, err := json.Marshal(location{Version: locationVersion, Root: filepath.Clean(root)})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(userData, 0o755); err != nil {
		return err
	}
	path := filepath.Join(userData, locationName)
	return writePreparedFile(path, func(w io.Writer) error {
		_, writeErr := w.Write(data)
		return writeErr
	}, nil)
}

// Holds reports whether root already holds a non-empty notebook or verified
// backups that opening can recover.
func Holds(root string) bool {
	info, err := os.Stat(filepath.Join(root, databaseName))
	if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return true
	}
	backups, err := ListBackups(root)
	return err == nil && len(backups) > 0
}
