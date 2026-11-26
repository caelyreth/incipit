package settings

import (
	"math"
	"strings"

	"github.com/caelyreth/neverland/internal/app/fonts"
)

// ThemeChoice selects one of Neverland's coherent built-in palettes.
type ThemeChoice uint8

// System follows the system's appearance, a fixed pair: Starfield while
// it is dark, Daybreak while it is light.
const (
	Starfield ThemeChoice = iota + 1
	Daybreak
	System
)

func (a ThemeChoice) Valid() bool {
	return a == Starfield || a == Daybreak || a == System
}

// Resolved is the palette the choice shows while the system is dark or
// not.
func (a ThemeChoice) Resolved(dark bool) ThemeChoice {
	if a != System {
		return a
	}
	if dark {
		return Starfield
	}
	return Daybreak
}

const (
	MinUIFontSize     float32 = 12
	MaxUIFontSize     float32 = 24
	DefaultUIFontSize float32 = 16
	MinProseFontSize  float32 = 15
	MaxProseFontSize  float32 = 26
	DefaultProseSize  float32 = 17
	// Line spacing is a body line's height over its text's size, from
	// compact to loose. The page derives every other room from it: the
	// leading of headings, lists and code, and the gaps between blocks.
	// Below 1.5 a line would be shorter than the prose face's own height,
	// which no row goes below.
	MinLineSpacing     float32 = 1.5
	MaxLineSpacing     float32 = 2.2
	DefaultLineSpacing float32 = 1.8
	LineSpacingStep    float32 = 0.1
)

// AppearanceSettings is app-wide product display state, independent of MyGo.
type AppearanceSettings struct {
	Appearance    ThemeChoice
	UIFontSize    float32
	ProseFontSize float32
	LineSpacing   float32
	// Typeface is the prose's style, Serif or Sans: sets the system generic
	// fallback and pairs downloadable Noto faces.
	Typeface fonts.Style
	// ProseFont is the user-chosen primary prose font family (any installed
	// system font or downloaded face); empty uses the system generic family
	// matching Typeface ("serif" or "sans-serif").
	ProseFont string
	// CJKForms is the language (internal/app/fonts) whose glyph forms Chinese and
	// Japanese are drawn in, by its face in Typeface once kept; empty, the
	// system's fonts draw them.
	CJKForms string
	// DrawnMenus has the app draw its popup menus itself on macOS, as it
	// always does on Windows and Linux; off, the default, macOS draws them.
	DrawnMenus bool
}

// ThemeNumber returns the stable one-based number of the palette shown
// while the system is dark or not, for the declarative settings view.
func (s AppearanceSettings) ThemeNumber(dark bool) uint8 {
	return uint8(s.Appearance.Resolved(dark))
}

func DefaultAppearance() AppearanceSettings {
	return AppearanceSettings{
		Appearance:    Starfield,
		UIFontSize:    DefaultUIFontSize,
		ProseFontSize: DefaultProseSize,
		LineSpacing:   DefaultLineSpacing,
	}
}

func (s AppearanceSettings) Normalized() AppearanceSettings {
	if !s.Appearance.Valid() {
		s.Appearance = Starfield
	}
	s.UIFontSize = clampFloat(s.UIFontSize, MinUIFontSize, MaxUIFontSize)
	s.ProseFontSize = clampFloat(s.ProseFontSize, MinProseFontSize, MaxProseFontSize)
	s.LineSpacing = clampFloat(s.LineSpacing, MinLineSpacing, MaxLineSpacing)
	if !s.Typeface.Valid() {
		s.Typeface = fonts.Serif
	}
	s.ProseFont = cleanFontName(s.ProseFont)
	if _, ok := fonts.Find(fonts.Serif, s.CJKForms); !ok || s.CJKForms == fonts.FormsLatin {
		s.CJKForms = ""
	}
	return s
}

// Apply mutates a candidate and reports whether the request was valid.
func (s *AppearanceSettings) Apply(change AppearanceChange) bool {
	if s == nil {
		return false
	}
	switch change.Kind {
	case SetTheme:
		appearance := ThemeChoice(change.Theme)
		if !appearance.Valid() {
			return false
		}
		s.Appearance = appearance
	case SetUIFontSize:
		if change.FontSize != change.FontSize {
			return false
		}
		s.UIFontSize = clampFloat(change.FontSize, MinUIFontSize, MaxUIFontSize)
	case SetProseFontSize:
		if change.FontSize != change.FontSize {
			return false
		}
		s.ProseFontSize = clampFloat(change.FontSize, MinProseFontSize, MaxProseFontSize)
	case SetLineSpacing:
		if change.Spacing != change.Spacing {
			return false
		}
		// Spacing moves in steps, kept exact as they add up.
		steps := float32(math.Round(float64(change.Spacing / LineSpacingStep)))
		s.LineSpacing = clampFloat(steps*LineSpacingStep, MinLineSpacing, MaxLineSpacing)
	case SetTypeface:
		if !change.Typeface.Valid() {
			return false
		}
		s.Typeface = change.Typeface
	case SetProseFont:
		font, ok := parseFontName(change.Font)
		if !ok {
			return false
		}
		s.ProseFont = font
	case SetCJKForms:
		if change.Forms == fonts.FormsLatin {
			return false
		}
		if _, ok := fonts.Find(fonts.Serif, change.Forms); !ok && change.Forms != "" {
			return false
		}
		s.CJKForms = change.Forms
	case SetDrawnMenus:
		s.DrawnMenus = change.Drawn
	default:
		return false
	}
	return true
}

func clampFloat(value, low, high float32) float32 {
	return max(low, min(value, high))
}

func parseFontName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, ",\r\n") {
		return "", false
	}
	return name, true
}

func cleanFontName(name string) string {
	cleaned, ok := parseFontName(name)
	if !ok {
		return ""
	}
	return cleaned
}
