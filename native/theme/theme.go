// Package theme maps Neverland's semantic materials and independent prose
// metrics into values MyGo paints. It is the only package that translates the
// product palette into MyGo's control theme.
package theme

import "github.com/egoist/mygo/ui"

type Appearance uint8

const (
	Starfield Appearance = iota + 1
	Daybreak
)

// Accent is one of Cosmofi's six hued families. Static and Moonveil, the
// first and last of its palette, are the neutrals text is made of.
type Accent uint8

const (
	Halo Accent = iota
	Afterglow
	Remnant
	Unripe
	Life
	Regolith
)

var AllAccents = [...]Accent{Halo, Afterglow, Remnant, Unripe, Life, Regolith}

func (a Accent) Label() string {
	switch a {
	case Halo:
		return "Halo"
	case Remnant:
		return "Remnant"
	case Unripe:
		return "Unripe"
	case Life:
		return "Life"
	case Regolith:
		return "Regolith"
	default:
		return "Afterglow"
	}
}

// Family is one hue in Cosmofi's four roles.
type Family struct {
	Surface    ui.Color
	Muted      ui.Color
	Foreground ui.Color
	Emphasis   ui.Color
}

type Surfaces struct {
	Background ui.Color
	Panel      ui.Color
	Page       ui.Color
	Inset      ui.Color
	// Chosen is a chosen row's fill in a list, dimmer than Inset so what
	// a row holds on a family's surface, as a tag, still stands out.
	Chosen   ui.Color
	Backdrop ui.Color
	Elevated ui.Color
}

type TextColors struct {
	Default ui.Color
	Heading ui.Color
	Muted   ui.Color
}

type IconColors struct {
	Default ui.Color
	Muted   ui.Color
	Accent  ui.Color
}

type BorderColors struct {
	Default  ui.Color
	Variant  ui.Color
	Focused  ui.Color
	Selected ui.Color
}

type ElementColors struct {
	Background ui.Color
	Hover      ui.Color
	Selected   ui.Color
}

type AccentColors struct {
	Foreground ui.Color
	Muted      ui.Color
	OnAccent   ui.Color
}

type ScrollbarColors struct {
	Thumb      ui.Color
	ThumbHover ui.Color
}

type StatusColor struct {
	Foreground ui.Color
	Background ui.Color
	Border     ui.Color
}

type StatusColors struct {
	Info    StatusColor
	Warning StatusColor
	Success StatusColor
	Danger  StatusColor
}

type ProseColors struct {
	Text      ui.Color
	Strong    ui.Color
	Notation  ui.Color
	Subtle    ui.Color
	Selection ui.Color
}

// Theme is the complete semantic product theme, independent of the subset of
// fields MyGo exposes in ui.Theme.
type Theme struct {
	Appearance   Appearance
	AccentFamily Accent
	Surface      Surfaces
	Text         TextColors
	Icon         IconColors
	Border       BorderColors
	Element      ElementColors
	GhostElement ElementColors
	Accent       AccentColors
	Status       StatusColors
	Scrollbar    ScrollbarColors
	Prose        ProseColors
	// Families are the six hues, by Accent, for what wears a color of its
	// own, as a group.
	Families [len(AllAccents)]Family
}

type Settings struct {
	// Scheme is the palettes the appearances take; nil is Cosmofi's.
	Scheme        *Scheme
	Appearance    Appearance
	Accent        Accent
	UIFontSize    float32
	ProseFontSize float32
	LineSpacing   float32
}

const (
	MinUIFontSize     float32 = 12
	MaxUIFontSize     float32 = 24
	DefaultUIFontSize float32 = 16
	// DefaultProseFontSize is a reading size for a page: larger than the
	// interface, as a book's text is, but not a display size.
	DefaultProseFontSize float32 = 17
)

func DefaultSettings() Settings {
	return Settings{
		Appearance:    Starfield,
		Accent:        Halo,
		UIFontSize:    DefaultUIFontSize,
		ProseFontSize: DefaultProseFontSize,
		LineSpacing:   DefaultLineSpacing,
	}
}

// Prose is document text's style under these settings.
func (s Settings) Prose() ProseStyle {
	return NewProseStyle(s.ProseFontSize).WithSpacing(s.LineSpacing)
}

func (s Settings) Normalized() Settings {
	s.UIFontSize = clamp(s.UIFontSize, MinUIFontSize, MaxUIFontSize)
	prose := s.Prose()
	s.ProseFontSize, s.LineSpacing = prose.FontSize, prose.Spacing
	return s
}

func (s Settings) Semantic() Theme {
	palette := s.Palette()
	accent := familyFor(palette, s.Accent)
	return Theme{
		Appearance:   s.Appearance,
		AccentFamily: s.Accent,
		Surface: Surfaces{
			Background: palette.Background,
			Panel:      palette.Background,
			Page:       palette.Page,
			Inset:      palette.Inset,
			Chosen:     palette.Chosen,
			Backdrop:   ui.RGBA(0, 0, 0, 0.38),
			Elevated:   palette.Raised,
		},
		Text: TextColors{
			Default: palette.TextPrimary,
			Heading: palette.TextEmphasis,
			Muted:   palette.TextSecondary,
		},
		Icon: IconColors{
			Default: palette.TextPrimary,
			Muted:   palette.TextSecondary,
			Accent:  accent.Foreground,
		},
		Border: BorderColors{
			Default:  palette.Hairline,
			Variant:  palette.Subtle,
			Focused:  accent.Foreground,
			Selected: accent.Foreground,
		},
		Element: ElementColors{
			Background: palette.Inset,
			Hover:      palette.Hover,
			Selected:   palette.Selected,
		},
		GhostElement: ElementColors{
			Background: ui.Transparent,
			Hover:      palette.Hover,
			Selected:   palette.Selected,
		},
		Accent: AccentColors{
			Foreground: accent.Foreground,
			Muted:      accent.Surface,
			OnAccent:   palette.Background,
		},
		Status: StatusColors{
			Info:    statusColor(palette.Accents[Halo]),
			Warning: statusColor(palette.Accents[Regolith]),
			Success: statusColor(palette.Accents[Life]),
			Danger:  statusColor(palette.Accents[Afterglow]),
		},
		Scrollbar: ScrollbarColors{
			Thumb:      palette.Guide,
			ThumbHover: palette.Track,
		},
		Prose: ProseColors{
			Text:     palette.TextPrimary,
			Strong:   palette.TextEmphasis,
			Notation: palette.TextSecondary,
			Subtle:   palette.Hairline,
			// Selection is the accent family's surface: the hue, quiet
			// enough to read text through.
			Selection: accent.Surface,
		},
		Families: palette.Accents,
	}
}

// MyGoTheme overlays the Neverland semantic colors onto MyGo's control theme.
func (s Settings) MyGoTheme() *ui.Theme {
	return s.MyGoThemeFor(s.Normalized().Semantic())
}

// MyGoThemeFor is MyGoTheme painted in semantic, such as a theme partway
// through a Transition, at these settings' scale.
func (s Settings) MyGoThemeFor(semantic Theme) *ui.Theme {
	normalized := s.Normalized()
	result := ui.DarkTheme()
	if normalized.Appearance == Daybreak {
		result = ui.LightTheme()
	}
	result.Background = semantic.Surface.Background
	result.Surface = semantic.Element.Background
	result.SurfaceHover = semantic.Element.Hover
	result.SurfacePressed = semantic.Element.Selected
	result.Border = semantic.Border.Default
	result.Text = semantic.Text.Default
	result.TextMuted = semantic.Text.Muted
	result.Accent = semantic.Accent.Foreground
	result.AccentHover = semantic.Accent.Foreground
	result.AccentPressed = semantic.Accent.Foreground
	result.AccentText = semantic.Accent.OnAccent
	result.Danger = semantic.Status.Danger.Foreground
	result.Warning = semantic.Status.Warning.Foreground
	result.Success = semantic.Status.Success.Foreground
	result.Selection = semantic.Prose.Selection
	result.Focus = semantic.Border.Focused
	result.Scrollbar = semantic.Scrollbar.Thumb
	scale := normalized.UIFontSize / DefaultUIFontSize
	result.ScrollbarWidth = 6 * scale
	result.Radius = 8 * scale
	result.Spacing = 4 * scale
	result.FontSize = normalized.UIFontSize
	result.Font = UIFontFamily
	return result
}

// Palette is the palette of the settings' scheme for their appearance.
func (s Settings) Palette() Palette {
	scheme := s.Scheme
	if scheme == nil {
		scheme = Cosmofi()
	}
	return scheme.For(s.Appearance)
}

func statusColor(family Family) StatusColor {
	return StatusColor{
		Foreground: family.Foreground,
		Background: family.Surface,
		Border:     family.Muted,
	}
}

func familyFor(palette Palette, accent Accent) Family {
	if int(accent) >= len(palette.Accents) {
		accent = Halo
	}
	return palette.Accents[accent]
}

func clamp(value, low, high float32) float32 {
	return max(low, min(value, high))
}
