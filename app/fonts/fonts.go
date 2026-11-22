// Package fonts keeps the prose faces for Chinese and Japanese, which the
// app does not ship: Google Fonts' Noto Serif and Noto Sans for each
// language's glyph forms, their static builds at the weights the page
// draws, downloaded only when the person chooses one, from Google Fonts'
// static hosts, verified against the SHA-256 pinned in the catalog
// (scripts/fontcatalog), and kept on this machine, not in the notebook.
// The one chosen draws what the Latin face, Noto Serif or Noto Sans as the
// style is, does not; with none, the system's fonts draw it.
package fonts

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// Style is the prose's style: Serif or Sans, for the Latin face the app
// ships and the face of each language alike.
type Style uint8

const (
	Serif Style = iota
	Sans
)

// Valid reports whether s is a style.
func (s Style) Valid() bool { return s <= Sans }

// String is the style as the settings keep and name it.
func (s Style) String() string {
	if s == Sans {
		return "Sans"
	}
	return "Serif"
}

// Face is one language's downloadable face in a style.
type Face struct {
	// ID names it, by its style and its forms.
	ID    string
	Style Style
	// Forms names the language whose glyph forms it draws: sc, tc, hk or
	// jp.
	Forms string
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

// Find is the catalog's face of forms in style.
func Find(style Style, forms string) (Face, bool) {
	i := slices.IndexFunc(Catalog, func(f Face) bool { return f.Style == style && f.Forms == forms })
	if i < 0 {
		return Face{}, false
	}
	return Catalog[i], true
}

// Languages are the glyph forms the catalog has faces for, each once, in
// its order, as the faces of one style name them.
func Languages() []Face {
	var out []Face
	for _, f := range Catalog {
		if f.Style == Serif {
			out = append(out, f)
		}
	}
	return out
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
