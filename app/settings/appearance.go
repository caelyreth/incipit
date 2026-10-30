package settings

import "math"

// ThemeChoice selects one of Neverland's coherent built-in palettes.
type ThemeChoice uint8

const (
	Starfield ThemeChoice = iota + 1
	Daybreak
)

func (a ThemeChoice) Valid() bool {
	return a == Starfield || a == Daybreak
}

// Accent is one of the design system's six hued families.
type Accent uint8

const (
	Halo Accent = iota + 1
	Afterglow
	Remnant
	Unripe
	Life
	Regolith
)

func (a Accent) Valid() bool {
	return a >= Halo && a <= Regolith
}

const (
	MinUIFontSize     float32 = 12
	MaxUIFontSize     float32 = 24
	DefaultUIFontSize float32 = 16
	MinProseFontSize  float32 = 15
	MaxProseFontSize  float32 = 26
	DefaultProseSize  float32 = 19
	// Line spacing is a body line's height over its text's size; the room
	// it leaves between lines is kept between every line, whatever size
	// its text. Below 1.5 a line would be shorter than the prose face's
	// own height, which no row goes below.
	MinLineSpacing     float32 = 1.5
	MaxLineSpacing     float32 = 2.2
	DefaultLineSpacing float32 = 1.8
	LineSpacingStep    float32 = 0.1
)

// AppearanceSettings is app-wide product display state, independent of MyGo.
type AppearanceSettings struct {
	Appearance    ThemeChoice
	Accent        Accent
	UIFontSize    float32
	ProseFontSize float32
	LineSpacing   float32
}

// ThemeNumber returns the stable one-based module representation used by the
// declarative settings view.
func (s AppearanceSettings) ThemeNumber() uint8 {
	return uint8(s.Appearance)
}

// AccentNumber returns the stable one-based accent representation.
func (s AppearanceSettings) AccentNumber() uint8 {
	return uint8(s.Accent)
}

func DefaultAppearance() AppearanceSettings {
	return AppearanceSettings{
		Appearance:    Starfield,
		Accent:        Halo,
		UIFontSize:    DefaultUIFontSize,
		ProseFontSize: DefaultProseSize,
		LineSpacing:   DefaultLineSpacing,
	}
}

func (s AppearanceSettings) Normalized() AppearanceSettings {
	if !s.Appearance.Valid() {
		s.Appearance = Starfield
	}
	if !s.Accent.Valid() {
		s.Accent = Halo
	}
	s.UIFontSize = clampFloat(s.UIFontSize, MinUIFontSize, MaxUIFontSize)
	s.ProseFontSize = clampFloat(s.ProseFontSize, MinProseFontSize, MaxProseFontSize)
	s.LineSpacing = clampFloat(s.LineSpacing, MinLineSpacing, MaxLineSpacing)
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
	case SetAccent:
		accent := Accent(change.Accent)
		if !accent.Valid() {
			return false
		}
		s.Accent = accent
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
	default:
		return false
	}
	return true
}

func clampFloat(value, low, high float32) float32 {
	return max(low, min(value, high))
}
