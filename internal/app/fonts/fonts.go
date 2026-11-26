// Package fonts keeps downloadable prose faces for Latin and for Chinese and
// Japanese: Google Fonts' Noto Serif and Noto Sans, verified against the
// SHA-256 pinned in the catalog (tools/fonts) and stored locally on this
// machine. When none are chosen, the system's own fonts (or a user-selected
// installed font) draw prose.
package fonts

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// FormsLatin identifies the downloadable Latin Noto face in each style.
const FormsLatin = "latin"

// Style is the prose's style: Serif or Sans, controlling the system generic
// fallback ("serif" / "sans-serif") and pairing downloadable Noto faces.
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

// Face is one downloadable face in a style.
type Face struct {
	// ID names it, by its style and its forms.
	ID    string
	Style Style
	// Forms names what it draws: "latin", "sc", "tc", "hk" or "jp".
	Forms string
	// Family is its name, the family its files are registered under and
	// the prose family list names.
	Family string
	Files  []File
}

// IsLatin reports whether f is a downloadable Latin prose face.
func (f Face) IsLatin() bool { return f.Forms == FormsLatin }

// File is one of a face's font files: its path on each of Hosts, the name
// it is kept under, its weight, whether it is italic, its size and its SHA-256.
type File struct {
	Path   string
	Name   string
	Weight int
	Italic bool
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

// LookupFamily returns the catalog face matching family, case-insensitively.
func LookupFamily(family string) (Face, bool) {
	i := slices.IndexFunc(Catalog, func(f Face) bool { return strings.EqualFold(f.Family, family) })
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

// Latin returns the downloadable Latin Noto face for style.
func Latin(style Style) (Face, bool) {
	return Find(style, FormsLatin)
}

// Languages returns the CJK glyph-form faces the catalog offers, each once,
// in catalog order.
func Languages() []Face {
	var out []Face
	for _, f := range Catalog {
		if f.Style == Serif && !f.IsLatin() {
			out = append(out, f)
		}
	}
	return out
}

// ResolveFamily returns the prose family list: primary (when non-empty),
// then the system fallback base ("serif" or "sans-serif"), followed by any
// optional fallback faces (e.g. a downloaded CJK face) without duplicates.
// Placing base before optional CJK fallbacks prevents CJK faces from taking
// over Latin text if primary is unavailable on another machine.
func ResolveFamily(base, primary string, fallbacks ...*Face) string {
	var parts []string
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		parts = append(parts, name)
	}
	primary = strings.TrimSpace(primary)
	if primary != "" && !strings.EqualFold(primary, base) {
		add(primary)
	}
	add(base)
	for _, fb := range fallbacks {
		if fb != nil {
			add(fb.Family)
		}
	}
	return strings.Join(parts, ", ")
}

// HomeVariable is the variable that moves the notebook, and with it the
// fonts, for development and tests (storage.HomeVariable).
const HomeVariable = "NEVERLAND_HOME"

// Dir is where downloaded faces are kept: a Fonts folder in
// NEVERLAND_HOME when it is set, or in userData, the app's folder of the
// user's application support, which no sync carries, as the notebook's may
// be.
func Dir(userData string) (string, error) {
	if home := os.Getenv(HomeVariable); home != "" {
		return filepath.Abs(filepath.Join(home, "Fonts"))
	}
	if userData == "" {
		return "", errors.New("fonts: no folder for application support")
	}
	return filepath.Join(userData, "Fonts"), nil
}
