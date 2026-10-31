//go:build !darwin && !windows

package storage

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// documentsDir is the XDG documents folder: XDG_DOCUMENTS_DIR, else the one
// user-dirs.dirs names, else ~/Documents.
func documentsDir() (string, error) {
	if dir := os.Getenv("XDG_DOCUMENTS_DIR"); dir != "" {
		return expandHome(dir)
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		config = filepath.Join(home, ".config")
	}
	if dir, ok := userDirsDocuments(filepath.Join(config, "user-dirs.dirs")); ok {
		return expandHome(dir)
	}
	return homeDocuments()
}

// userDirsDocuments reads XDG_DOCUMENTS_DIR from a user-dirs.dirs file,
// which xdg-user-dirs writes as shell assignments.
func userDirsDocuments(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		value, ok := strings.CutPrefix(line, "XDG_DOCUMENTS_DIR=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		if value != "" {
			return value, true
		}
	}
	return "", false
}

func expandHome(dir string) (string, error) {
	if !strings.HasPrefix(dir, "$HOME") && !strings.HasPrefix(dir, "~") {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(dir, "$HOME"), "~")
	return filepath.Join(home, rest), nil
}
