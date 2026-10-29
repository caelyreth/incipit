package settings

// ThemeChoice selects one of Neverland's coherent built-in palettes.
type ThemeChoice uint8

const (
	Ink ThemeChoice = iota + 1
	Paper
)

func (a ThemeChoice) Valid() bool {
	return a == Ink || a == Paper
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
)

// AppearanceSettings is app-wide product display state, independent of MyGo.
type AppearanceSettings struct {
	Appearance    ThemeChoice
	Accent        Accent
	UIFontSize    float32
	ProseFontSize float32
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
		Appearance:    Ink,
		Accent:        Afterglow,
		UIFontSize:    DefaultUIFontSize,
		ProseFontSize: DefaultProseSize,
	}
}

func (s AppearanceSettings) Normalized() AppearanceSettings {
	if !s.Appearance.Valid() {
		s.Appearance = Ink
	}
	if !s.Accent.Valid() {
		s.Accent = Afterglow
	}
	s.UIFontSize = clampFloat(s.UIFontSize, MinUIFontSize, MaxUIFontSize)
	s.ProseFontSize = clampFloat(s.ProseFontSize, MinProseFontSize, MaxProseFontSize)
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
	default:
		return false
	}
	return true
}

func clampFloat(value, low, high float32) float32 {
	return max(low, min(value, high))
}
