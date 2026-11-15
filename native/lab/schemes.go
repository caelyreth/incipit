package lab

import "github.com/caelyreth/neverland/native/theme"

// Schemes are the schemes the lab sets beside each other: Cosmofi's, as
// the app paints today, first.
func Schemes() []*theme.Scheme { return []*theme.Scheme{theme.Cosmofi(), &quiet} }

// quiet mends only the ladder Cosmofi's derivation leaves inverted, at
// its lightness, with near-neutral surfaces in Moonveil's hue (99°, chroma
// at most 0.005) so the ground and the ink are one material:
//
//   - chosen stands above hover, both neutral (Starfield ΔL 6.4 over 2.7,
//     Daybreak 4.9 over 2.1);
//   - the page is the lightest plane by day and lifts a little by night;
//   - what is raised stands above the ground by day as by night.
//
// The families keep Cosmofi's inks; only their surfaces, a selection and a
// tag's fill, take their hue at chroma 0.028, quiet enough to read through.
// The values come from references/ui-remake-audit-2026-11-14/probes.
var quiet = theme.Scheme{
	Name: "Quiet",
	Starfield: theme.Palette{
		Background:    theme.RGB(0x171716),
		Page:          theme.RGB(0x1b1b19),
		Raised:        theme.RGB(0x252423),
		Inset:         theme.RGB(0x21201f),
		Chosen:        theme.RGB(0x262624),
		Hover:         theme.RGB(0x1e1d1c),
		Selected:      theme.RGB(0x2c2c2a),
		Hairline:      theme.RGB(0x2c2c2a),
		Subtle:        theme.RGB(0x222220),
		TextPrimary:   theme.RGB(0xc1bfb3),
		TextEmphasis:  theme.RGB(0xd9d7cd),
		TextSecondary: theme.RGB(0x92918b),
		Guide:         theme.RGB(0x3e3d3a),
		Track:         theme.RGB(0x565552),
		Accents: [...]theme.Family{
			theme.Halo:      theme.NewFamily(0x2d2c3b, 0x58566f, 0xa19ec2, 0x8a83b0),
			theme.Afterglow: theme.NewFamily(0x3b2828, 0x955b5d, 0xdc8789, 0xce7076),
			theme.Remnant:   theme.NewFamily(0x39282f, 0x855b70, 0xc88da5, 0xbc728f),
			theme.Unripe:    theme.NewFamily(0x2d301f, 0x5b6148, 0xa0a77b, 0x8b965c),
			theme.Life:      theme.NewFamily(0x21322a, 0x4b6a5a, 0x7dad96, 0x539579),
			theme.Regolith:  theme.NewFamily(0x382b1f, 0x735940, 0xbd9976, 0xb78658),
		},
	},
	Daybreak: theme.Palette{
		Background:    theme.RGB(0xf3f2ef),
		Page:          theme.RGB(0xfcfcfa),
		Raised:        theme.RGB(0xfefefd),
		Inset:         theme.RGB(0xededea),
		Chosen:        theme.RGB(0xe3e2de),
		Hover:         theme.RGB(0xecebe8),
		Selected:      theme.RGB(0xdbdbd7),
		Hairline:      theme.RGB(0xdad9d5),
		Subtle:        theme.RGB(0xe7e6e3),
		TextPrimary:   theme.RGB(0x535248),
		TextEmphasis:  theme.RGB(0x45433d),
		TextSecondary: theme.RGB(0x6e6d68),
		Guide:         theme.RGB(0xbebeb9),
		Track:         theme.RGB(0x999893),
		Accents: [...]theme.Family{
			theme.Halo:      theme.NewFamily(0xdddbf0, 0xafaec5, 0x696685, 0x726c96),
			theme.Afterglow: theme.NewFamily(0xf0d7d7, 0xbe8b8c, 0x9a5255, 0xa5545a),
			theme.Remnant:   theme.NewFamily(0xeed7e0, 0xc28ca6, 0x975172, 0xa84b73),
			theme.Unripe:    theme.NewFamily(0xdde0cc, 0xa8b08f, 0x6a7038, 0x69752a),
			theme.Life:      theme.NewFamily(0xcee4d9, 0x93afa0, 0x4c7663, 0x387562),
			theme.Regolith:  theme.NewFamily(0xecdbcb, 0xc6a483, 0x8d6135, 0x9b5f1e),
		},
	},
}
