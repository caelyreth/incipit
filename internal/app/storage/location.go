package storage

import (
	"errors"
	"os"
	"path/filepath"
)

// FolderName is the folder in the user's Documents that holds the notebook.
const FolderName = "Neverland"

// HomeVariable names the environment variable that moves the notebook
// elsewhere, for development and tests.
const HomeVariable = "NEVERLAND_HOME"

// Root is the folder that holds the notebook: NEVERLAND_HOME when set, or
// a Neverland folder in documents, the user's Documents.
func Root(documents string) (string, error) {
	if home := os.Getenv(HomeVariable); home != "" {
		return filepath.Abs(home)
	}
	if documents == "" {
		return "", errors.New("storage: the Documents folder could not be found")
	}
	return filepath.Join(documents, FolderName), nil
}
