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

// What a thing is doing shows as the ink laid over whatever it stands
// on, never as a gray of its own: the same step on the chrome, the page,
// what is raised or what is set in, and never darker on one than on
// another. Under the pointer, pressed, chosen: each louder than the last.
const (
	StateHover   float32 = 0.05
	StatePressed float32 = 0.09
	StateChosen  float32 = 0.13
)

// The accent comes in as few steps, one meaning each: a hint under the
// pointer on what is the accent's, a tint on what something is dropped
// on or is taken, its surface on what is chosen in its hue, and strong
// for a ring around what has the keys.
const (
	TintHint   float32 = 0.08
	TintFill   float32 = 0.16
	TintStrong float32 = 0.85
)

type Surfaces struct {
	Background ui.Color
	Panel      ui.Color
	Page       ui.Color
	Inset      ui.Color
	// Chosen is a chosen row's fill in a list: neutral, laid over its
	// ground, louder than a hover, and marked with the accent at its edge
	// by the list.
	Chosen   ui.Color
	Backdrop ui.Color
	Elevated ui.Color
	// Inverse is what is chosen among a few, as which of the lane's
	// places the field shows: moonveil's ink as its ground, OnInverse its
	// words.
	Inverse ui.Color
}

type TextColors struct {
	Default   ui.Color
	Heading   ui.Color
	Muted     ui.Color
	OnInverse ui.Color
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
	// Hint, Tint and Strong are the accent's ink laid over its ground
	// (TintHint, TintFill, TintStrong); Muted is its opaque surface.
	Hint     ui.Color
	Tint     ui.Color
	Muted    ui.Color
	Strong   ui.Color
	OnAccent ui.Color
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

// TagColors are a tag's, in the page as in a list: quiet, neutral, so a
// sheet's tags never add a hue to the window.
type TagColors struct {
	Background ui.Color
	Foreground ui.Color
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
	Tag          TagColors
	Prose        ProseColors
	// Families are the six hues, by Accent, for what wears a color of its
	// own, as a group.
	Families [len(AllAccents)]Family
}

type Settings struct {
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
	palette := paletteFor(s.Appearance)
	accent := familyFor(palette, s.Accent)
	ink := palette.textPrimary
	return Theme{
		Appearance:   s.Appearance,
		AccentFamily: s.Accent,
		Surface: Surfaces{
			Background: palette.background,
			Panel:      palette.background,
			Page:       palette.working,
			Inset:      palette.inset,
			Chosen:     ink.Alpha(StateChosen),
			Backdrop:   ui.RGBA(0, 0, 0, 0.38),
			Elevated:   palette.raised,
			Inverse:    palette.textEmphasis,
		},
		Text: TextColors{
			Default:   palette.textPrimary,
			Heading:   palette.textEmphasis,
			Muted:     palette.textSecondary,
			OnInverse: palette.background,
		},
		Icon: IconColors{
			Default: palette.textPrimary,
			Muted:   palette.textSecondary,
			Accent:  accent.Foreground,
		},
		Border: BorderColors{
			Default:  palette.hairline,
			Variant:  palette.subtle,
			Focused:  accent.Foreground,
			Selected: accent.Foreground,
		},
		Element: ElementColors{
			Background: palette.inset,
			Hover:      ink.Alpha(StateHover).Over(palette.inset),
			Selected:   ink.Alpha(StatePressed).Over(palette.inset),
		},
		GhostElement: ElementColors{
			Background: ui.Transparent,
			Hover:      ink.Alpha(StateHover),
			Selected:   ink.Alpha(StatePressed),
		},
		Accent: AccentColors{
			Foreground: accent.Foreground,
			Hint:       accent.Foreground.Alpha(TintHint),
			Tint:       accent.Foreground.Alpha(TintFill),
			Muted:      accent.Surface,
			Strong:     accent.Foreground.Alpha(TintStrong),
			OnAccent:   onAccent(s.Appearance),
		},
		Status: StatusColors{
			Info:    statusColor(palette.accents[Halo]),
			Warning: statusColor(palette.accents[Regolith]),
			Success: statusColor(palette.accents[Life]),
			Danger:  statusColor(palette.accents[Afterglow]),
		},
		Scrollbar: ScrollbarColors{
			Thumb:      palette.guide,
			ThumbHover: palette.track,
		},
		Tag: TagColors{
			// Between a hover and a press, laid over its ground: a tag
			// stays a step off the row it stands in, hovered or not.
			Background: ink.Alpha((StateHover + StatePressed) / 2),
			Foreground: palette.textSecondary,
		},
		Prose: ProseColors{
			Text:     palette.textPrimary,
			Strong:   palette.textEmphasis,
			Notation: palette.textSecondary,
			Subtle:   palette.hairline,
			// Selection is the accent family's surface: the hue, quiet
			// enough to read text through.
			Selection: accent.Surface,
		},
		Families: palette.accents,
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

func paletteFor(appearance Appearance) palette {
	if appearance == Daybreak {
		return daybreak()
	}
	return starfield()
}

func statusColor(family Family) StatusColor {
	return StatusColor{
		Foreground: family.Foreground,
		Background: family.Surface,
		Border:     family.Muted,
	}
}

func familyFor(palette palette, accent Accent) Family {
	if int(accent) >= len(palette.accents) {
		accent = Halo
	}
	return palette.accents[accent]
}

func onAccent(appearance Appearance) ui.Color {
	return paletteFor(appearance).background
}

func clamp(value, low, high float32) float32 {
	return max(low, min(value, high))
}
