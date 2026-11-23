package storage

import "golang.org/x/sys/windows"

// documentsDir is the Documents known folder, wherever the user or their
// organization moved it.
func documentsDir() (string, error) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DEFAULT)
	if err != nil || path == "" {
		return homeDocuments()
	}
	return path, nil
}
