package settings

import (
	"math"

	"github.com/caelyreth/neverland/app/fonts"
)

// ThemeChoice selects one of Neverland's coherent built-in palettes.
type ThemeChoice uint8

const (
	Starfield ThemeChoice = iota + 1
	Daybreak
)

func (a ThemeChoice) Valid() bool {
	return a == Starfield || a == Daybreak
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
	// HanFace is the downloaded face that draws Han characters, which the
	// faces for Chinese, Japanese and Korean each draw in their own way,
	// when more than one is kept; empty, the first in the catalog's order
	// (app/fonts).
	HanFace string
}

// ThemeNumber returns the stable one-based module representation used by the
// declarative settings view.
func (s AppearanceSettings) ThemeNumber() uint8 {
	return uint8(s.Appearance)
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
	if !hanFace(s.HanFace) {
		s.HanFace = ""
	}
	return s
}

// hanFace is whether id is a face that draws Han characters.
func hanFace(id string) bool {
	f, ok := fonts.Lookup(id)
	return ok && f.Han
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
	case SetHanFace:
		if !hanFace(change.Face) {
			return false
		}
		s.HanFace = change.Face
	default:
		return false
	}
	return true
}

func clampFloat(value, low, high float32) float32 {
	return max(low, min(value, high))
}
