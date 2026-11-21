package theme

import "github.com/egoist/mygo/ui"

// palette is one appearance's colors: Starfield at night, Daybreak by day.
// Every value is Cosmofi's (cosmofi/packages/cosmofi/src/index.ts), kept
// here by hand: its inks and hues, and its material, the ladder of
// surfaces and lines they stand on; the states laid over it are Cosmofi's
// too (theme.go). Change them there first. The color budget spends them
// sparingly: neutral surfaces and states, one accent, hues on identity
// alone. A family is one hue in four roles.
type palette struct {
	background ui.Color
	working    ui.Color
	raised     ui.Color
	inset      ui.Color
	hairline      ui.Color
	subtle        ui.Color
	textPrimary   ui.Color
	textEmphasis  ui.Color
	textSecondary ui.Color
	guide         ui.Color
	track         ui.Color
	accents       [len(AllAccents)]Family
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

// starfield is the night: Cosmofi's inks and hues on a ladder of
// near-neutral surfaces in Moonveil's hue (99°, chroma at most 0.005), so
// the ground and the ink are one material. Each step reads at its place:
// the page lifts a little off the chrome, what is raised stands above
// both, and a card's rim stands clear of the page. States are not steps
// of it but the ink laid over whatever they stand on (theme.go). A family's surface, its selection and tag fill,
// takes its hue at chroma 0.028, quiet enough to read text through.
func starfield() palette {
	return palette{
		background:    color(0x171716),
		working:       color(0x1b1b19),
		raised:        color(0x252423),
		inset:         color(0x21201f),
		hairline:      color(0x2c2c2a),
		subtle:        color(0x282826),
		textPrimary:   color(0xc1bfb3), // moonveil foreground
		textEmphasis:  color(0xd9d7cd), // moonveil emphasis
		textSecondary: color(0x92918b), // static foreground
		guide:         color(0x3e3d3a),
		track:         color(0x565552),
		accents: [...]Family{
			Halo:      family(0x2d2c3b, 0x58566f, 0xa19ec2, 0x8a83b0),
			Afterglow: family(0x3b2828, 0x955b5d, 0xdc8789, 0xce7076),
			Remnant:   family(0x39282f, 0x855b70, 0xc88da5, 0xbc728f),
			Unripe:    family(0x2d301f, 0x5b6148, 0xa0a77b, 0x8b965c),
			Life:      family(0x21322a, 0x4b6a5a, 0x7dad96, 0x539579),
			Regolith:  family(0x382b1f, 0x735940, 0xbd9976, 0xb78658),
		},
	}
}

// daybreak is the day: the same inks and hues, the page the lightest
// plane and what is raised lighter than the ground it stands on, so
// nothing meant to stand out sinks into it.
func daybreak() palette {
	return palette{
		background:    color(0xf3f2ef),
		working:       color(0xfcfcfa),
		raised:        color(0xfefefd),
		inset:         color(0xededea),
		hairline:      color(0xdad9d5),
		subtle:        color(0xe7e6e3),
		textPrimary:   color(0x535248), // moonveil foreground
		textEmphasis:  color(0x45433d), // moonveil emphasis
		textSecondary: color(0x6e6d68), // static foreground
		guide:         color(0xbebeb9),
		track:         color(0x999893),
		accents: [...]Family{
			Halo:      family(0xdddbf0, 0xafaec5, 0x696685, 0x726c96),
			Afterglow: family(0xf0d7d7, 0xbe8b8c, 0x9a5255, 0xa5545a),
			Remnant:   family(0xeed7e0, 0xc28ca6, 0x975172, 0xa84b73),
			Unripe:    family(0xdde0cc, 0xa8b08f, 0x6a7038, 0x69752a),
			Life:      family(0xcee4d9, 0x93afa0, 0x4c7663, 0x387562),
			Regolith:  family(0xecdbcb, 0xc6a483, 0x8d6135, 0x9b5f1e),
		},
	}
}
