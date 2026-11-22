// Package fonts keeps the prose faces for Chinese and Japanese, which the
// app does not ship: each is Google Fonts' Noto Serif for a language, its
// static builds at the weights the page draws, downloaded only when the
// person asks from Google Fonts' static hosts, verified against the
// SHA-256 pinned in the catalog (scripts/fontcatalog), and kept on this
// machine, not in the notebook. One of them, chosen, draws the text the
// Latin face does not; with none chosen, the system's fonts draw it.
package fonts

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// Face is one language's downloadable face.
type Face struct {
	// ID names it in the settings.
	ID string
	// Language is what it is drawn for, as the settings say, and Sample a
	// line in it, shown in the face.
	Language, Sample string
	// Family is its name, the family its files are registered under and
	// the prose family list names.
	Family string
	Files  []File
}

// File is one of a face's font files: its path on each of Hosts, the name
// it is kept under, its weight, its size and its SHA-256.
type File struct {
	Path   string
	Name   string
	Weight int
	Size   int64
	SHA256 string
}

// Hosts are where a file's path is asked of, in turn as one fails: Google
// Fonts' static host, then its mirror for mainland China, which serves the
// same files.
var Hosts = []string{"https://fonts.gstatic.com", "https://fonts.gstatic.cn"}

// Size is how much a face downloads.
func (f Face) Size() int64 {
	var n int64
	for _, file := range f.Files {
		n += file.Size
	}
	return n
}

// Lookup is the catalog's face id.
func Lookup(id string) (Face, bool) {
	i := slices.IndexFunc(Catalog, func(f Face) bool { return f.ID == id })
	if i < 0 {
		return Face{}, false
	}
	return Catalog[i], true
}

// Family is the prose family list: base, the Latin face the app ships,
// then the face chosen, when there is one.
func Family(base string, chosen *Face) string {
	if chosen == nil {
		return base
	}
	return base + ", " + chosen.Family
}

// HomeVariable is the variable that moves the notebook, and with it the
// fonts, for development and tests (storage.HomeVariable).
const HomeVariable = "NEVERLAND_HOME"

// Dir is where downloaded faces are kept: a Fonts folder in
// NEVERLAND_HOME when it is set, or in the app's folder of the user's
// application support, which no sync carries, as the notebook's may be.
func Dir() (string, error) {
	if home := os.Getenv(HomeVariable); home != "" {
		return filepath.Abs(filepath.Join(home, "Fonts"))
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if config == "" {
		return "", errors.New("fonts: no folder for application support")
	}
	return filepath.Join(config, "Neverland", "Fonts"), nil
}
