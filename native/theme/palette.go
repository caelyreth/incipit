package theme

import "github.com/egoist/mygo/ui"

// Palette is the colors of one appearance: the surfaces, lines and inks
// of the materials, and the six hued families. A Scheme pairs one for the
// night with one for the day; Cosmofi's is Neverland's own, and the
// design lab sets others beside it to be judged in the running app.
type Palette struct {
	Background ui.Color
	// Page is the working surface a sheet is written on.
	Page   ui.Color
	Raised ui.Color
	Inset  ui.Color
	// Chosen is a chosen row's fill: midway, by contrast, between the
	// panel and the families' surfaces, so a row stands out from the
	// panel by as much as what wears a surface stands out from the row.
	Chosen        ui.Color
	Hover         ui.Color
	Selected      ui.Color
	Hairline      ui.Color
	Subtle        ui.Color
	TextPrimary   ui.Color
	TextEmphasis  ui.Color
	TextSecondary ui.Color
	Guide         ui.Color
	Track         ui.Color
	Accents       [len(AllAccents)]Family
}

// Scheme is a Palette for each appearance, under a name.
type Scheme struct {
	Name      string
	Starfield Palette
	Daybreak  Palette
}

// For is the scheme's palette for an appearance.
func (s *Scheme) For(appearance Appearance) Palette {
	if appearance == Daybreak {
		return s.Daybreak
	}
	return s.Starfield
}

// Cosmofi is Neverland's scheme: the design system's tokens
// (cosmofi/packages/cosmofi), placed into Neverland's materials, a family
// being one hue in four roles.
func Cosmofi() *Scheme { return &cosmofi }

var cosmofi = Scheme{Name: "Cosmofi", Starfield: starfield(), Daybreak: daybreak()}

// RGB is the color of a 0xRRGGBB value.
func RGB(hex uint32) ui.Color { return color(hex) }

func color(hex uint32) ui.Color {
	return ui.RGB(
		uint8(hex>>16),
		uint8(hex>>8),
		uint8(hex),
	)
}

// NewFamily is a family of 0xRRGGBB values, in its four roles.
func NewFamily(surface, muted, foreground, emphasis uint32) Family {
	return Family{
		Surface:    color(surface),
		Muted:      color(muted),
		Foreground: color(foreground),
		Emphasis:   color(emphasis),
	}
}

// starfield is Cosmofi's dark scheme. Static and Moonveil, the neutral
// families, carry the text; the six hued families are the accents.
func starfield() Palette {
	return Palette{
		Background:    color(0x161616),
		Page:          color(0x1a1a1a),
		Raised:        color(0x242424),
		Inset:         color(0x272725), // static surface
		Chosen:        color(0x202020),
		Hover:         color(0x333333), // hairline
		Selected:      color(0x404040), // guide
		Hairline:      color(0x333333),
		Subtle:        color(0x242424),
		TextPrimary:   color(0xc1bfb3), // moonveil foreground
		TextEmphasis:  color(0xd9d7cd), // moonveil emphasis
		TextSecondary: color(0x92918b), // static foreground
		Guide:         color(0x404040),
		Track:         color(0x545450),
		Accents: [...]Family{
			Halo:      NewFamily(0x292831, 0x58566f, 0xa19ec2, 0x8a83b0),
			Afterglow: NewFamily(0x352426, 0x955b5d, 0xdc8789, 0xce7076),
			Remnant:   NewFamily(0x382731, 0x855b70, 0xc88da5, 0xbc728f),
			Unripe:    NewFamily(0x292b23, 0x5b6148, 0xa0a77b, 0x8b965c),
			Life:      NewFamily(0x252e29, 0x4b6a5a, 0x7dad96, 0x539579),
			Regolith:  NewFamily(0x302922, 0x735940, 0xbd9976, 0xb78658),
		},
	}
}

// daybreak is Cosmofi's light scheme: Starfield's hues, each role the same
// distance from a gray paper ground.
func daybreak() Palette {
	return Palette{
		Background:    color(0xf7f7f6),
		Page:          color(0xf2f2f0),
		Raised:        color(0xe8e8e6),
		Inset:         color(0xe5e5e3), // static surface
		Chosen:        color(0xeeeeec),
		Hover:         color(0xd2d2d0), // hairline
		Selected:      color(0xc2c2c0), // guide
		Hairline:      color(0xd2d2d0),
		Subtle:        color(0xe8e8e6),
		TextPrimary:   color(0x535248), // moonveil foreground
		TextEmphasis:  color(0x45433d), // moonveil emphasis
		TextSecondary: color(0x6e6d68), // static foreground
		Guide:         color(0xc2c2c0),
		Track:         color(0xa0a09c),
		Accents: [...]Family{
			Halo:      NewFamily(0xe1e1e9, 0xafaec5, 0x696685, 0x726c96),
			Afterglow: NewFamily(0xedddde, 0xbe8b8c, 0x9a5255, 0xa5545a),
			Remnant:   NewFamily(0xead6e2, 0xc28ca6, 0x975172, 0xa84b73),
			Unripe:    NewFamily(0xe0e2da, 0xa8b08f, 0x6a7038, 0x69752a),
			Life:      NewFamily(0xd9e1dc, 0x93afa0, 0x4c7663, 0x387562),
			Regolith:  NewFamily(0xe7dfd6, 0xc6a483, 0x8d6135, 0x9b5f1e),
		},
	}
}
