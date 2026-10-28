package theme

import "github.com/egoist/mygo/ui"

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
	clay       Family
	iris       Family
	sage       Family
	sand       Family
	slate      Family
	danger     Family
}

func color(hex uint32) ui.Color {
	return ui.RGB(
		uint8(hex>>16),
		uint8(hex>>8),
		uint8(hex),
	)
}

func family(surface, muted, foreground uint32) Family {
	return Family{
		Surface:    color(surface),
		Muted:      color(muted),
		Foreground: color(foreground),
	}
}

func inkPalette() palette {
	return palette{
		background: color(0x151515),
		page:       color(0x191919),
		raised:     color(0x242422),
		inset:      color(0x292927),
		hover:      color(0x343431),
		selected:   color(0x42423d),
		hairline:   color(0x30302c),
		subtle:     color(0x242422),
		text:       color(0xdad8d2),
		heading:    color(0xf1efe9),
		secondary:  color(0xa5a39b),
		guide:      color(0x575751),
		track:      color(0x77776d),
		clay:       family(0x422d25, 0x976047, 0xe49b7b),
		iris:       family(0x302d43, 0x736998, 0xb5a7df),
		sage:       family(0x263b31, 0x567c63, 0x91b99d),
		sand:       family(0x3e3423, 0x8e7544, 0xcead70),
		slate:      family(0x293742, 0x577b98, 0x9bbbd2),
		danger:     family(0x41292b, 0x9b565b, 0xe89a9f),
	}
}

func paperPalette() palette {
	return palette{
		background: color(0xf2f1ec),
		page:       color(0xfcfcfb),
		raised:     color(0xffffff),
		inset:      color(0xf6f5f1),
		hover:      color(0xe8e7e1),
		selected:   color(0xdedcd3),
		hairline:   color(0xd4d2c9),
		subtle:     color(0xe7e6df),
		text:       color(0x35352f),
		heading:    color(0x24241f),
		secondary:  color(0x626259),
		guide:      color(0xb9b7ac),
		track:      color(0x918f83),
		clay:       family(0xf1ded2, 0xc58e70, 0x985032),
		iris:       family(0xe8e2f3, 0xa99ac6, 0x6d548e),
		sage:       family(0xe0eade, 0x8eac91, 0x466c50),
		sand:       family(0xeee5cd, 0xb5a170, 0x7b6128),
		slate:      family(0xdfe8ee, 0x90aabb, 0x4c6b81),
		danger:     family(0xf3dfe0, 0xc58b8f, 0x9a3f48),
	}
}
