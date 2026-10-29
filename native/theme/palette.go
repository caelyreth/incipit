package theme

import "github.com/egoist/mygo/ui"

// palette is one Cosmofi scheme: Starfield at night, Daybreak by day. The
// values are the design system's tokens (cosmofi/packages/cosmofi), placed
// into Neverland's materials; a family is one hue in four roles.
type palette struct {
	background ui.Color
	page       ui.Color
	raised     ui.Color
	inset      ui.Color
	hover      ui.Color
	selected   ui.Color
	hairline   ui.Color
	subtle     ui.Color
	text       ui.Color
	heading    ui.Color
	secondary  ui.Color
	guide      ui.Color
	track      ui.Color
	accents    [len(AllAccents)]Family
}

func color(hex uint32) ui.Color {
	return ui.RGB(
		uint8(hex>>16),
		uint8(hex>>8),
		uint8(hex),
	)
}

func family(surface, muted, foreground, emphasis uint32) Family {
	return Family{
		Surface:    color(surface),
		Muted:      color(muted),
		Foreground: color(foreground),
		Emphasis:   color(emphasis),
	}
}

// starfield is Cosmofi's dark scheme. Static and Moonveil, the neutral
// families, carry the text; the six hued families are the accents.
func starfield() palette {
	return palette{
		background: color(0x161616),
		page:       color(0x1a1a1a),
		raised:     color(0x242424),
		inset:      color(0x272725), // static surface
		hover:      color(0x333333), // hairline
		selected:   color(0x404040), // guide
		hairline:   color(0x333333),
		subtle:     color(0x242424),
		text:       color(0xc1bfb3), // moonveil foreground
		heading:    color(0xd9d7cd), // moonveil emphasis
		secondary:  color(0x92918b), // static foreground
		guide:      color(0x404040),
		track:      color(0x545450),
		accents: [...]Family{
			Halo:      family(0x292831, 0x58566f, 0xa19ec2, 0x8a83b0),
			Afterglow: family(0x352426, 0x955b5d, 0xdc8789, 0xce7076),
			Remnant:   family(0x382731, 0x855b70, 0xc88da5, 0xbc728f),
			Unripe:    family(0x292b23, 0x5b6148, 0xa0a77b, 0x8b965c),
			Life:      family(0x252e29, 0x4b6a5a, 0x7dad96, 0x539579),
			Regolith:  family(0x302922, 0x735940, 0xbd9976, 0xb78658),
		},
	}
}

// daybreak is Cosmofi's light scheme: Starfield's hues, each role the same
// distance from a gray paper ground.
func daybreak() palette {
	return palette{
		background: color(0xf7f7f6),
		page:       color(0xf2f2f0),
		raised:     color(0xe8e8e6),
		inset:      color(0xe5e5e3), // static surface
		hover:      color(0xd2d2d0), // hairline
		selected:   color(0xc2c2c0), // guide
		hairline:   color(0xd2d2d0),
		subtle:     color(0xe8e8e6),
		text:       color(0x535248), // moonveil foreground
		heading:    color(0x45433d), // moonveil emphasis
		secondary:  color(0x6e6d68), // static foreground
		guide:      color(0xc2c2c0),
		track:      color(0xa0a09c),
		accents: [...]Family{
			Halo:      family(0xe1e1e9, 0xafaec5, 0x696685, 0x726c96),
			Afterglow: family(0xedddde, 0xbe8b8c, 0x9a5255, 0xa5545a),
			Remnant:   family(0xead6e2, 0xc28ca6, 0x975172, 0xa84b73),
			Unripe:    family(0xe0e2da, 0xa8b08f, 0x6a7038, 0x69752a),
			Life:      family(0xd9e1dc, 0x93afa0, 0x4c7663, 0x387562),
			Regolith:  family(0xe7dfd6, 0xc6a483, 0x8d6135, 0x9b5f1e),
		},
	}
}
