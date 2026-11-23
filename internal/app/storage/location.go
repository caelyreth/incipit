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
// a Neverland folder in the user's Documents.
func Root() (string, error) {
	if home := os.Getenv(HomeVariable); home != "" {
		return filepath.Abs(home)
	}
	documents, err := documentsDir()
	if err != nil {
		return "", err
	}
	if documents == "" {
		return "", errors.New("storage: the Documents folder could not be found")
	}
	return filepath.Join(documents, FolderName), nil
}

// homeDocuments is the conventional Documents folder in the home folder.
func homeDocuments() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents"), nil
}
