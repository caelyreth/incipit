package components

import (
	"image"
	"image/color"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// menuIcons keeps every icon a menu has shown: the menus hold them weakly.
var menuIcons sync.Map

// MenuIcon is the icon as a menu's item shows it beside its words: a
// template, tinted as the words are, drawn once and kept. It is nil where
// it cannot be drawn, and the item then shows none.
func (n IconName) MenuIcon() ui.MenuIcon {
	if icon, ok := menuIcons.Load(n); ok {
		return icon.(ui.MenuIcon)
	}
	icon, err := mygo.NewMenuIconSVG(n.SVG(), true)
	if err != nil {
		return nil
	}
	menuIcons.Store(n, ui.MenuIcon(icon))
	return icon
}

// MenuItem gives it the icon n, none where n is empty.
func MenuItem(it *ui.MenuItem, n IconName) *ui.MenuItem {
	if n == "" {
		return it
	}
	return it.Icon(n.MenuIcon())
}

// tintedIcon keys an icon drawn in a color of its own.
type tintedIcon struct {
	name IconName
	ink  ui.Color
}

// TintedMenuIcon is the icon as a menu's item shows it in ink, whatever
// the words' color, as a group's icon wears its tint: drawn once for each
// ink and kept. It is nil where it cannot be drawn.
func (n IconName) TintedMenuIcon(ink ui.Color) ui.MenuIcon {
	key := tintedIcon{n, ink}
	if icon, ok := menuIcons.Load(key); ok {
		return icon.(ui.MenuIcon)
	}
	icon, err := mygo.NewMenuIconSVG(inked{n.SVG(), ink}, false)
	if err != nil {
		return nil
	}
	menuIcons.Store(key, ui.MenuIcon(icon))
	return icon
}

// inked draws an SVG as an icon is drawn, its shapes' coverage alone, in
// ink.
type inked struct {
	*ui.SVG
	ink ui.Color
}

func (s inked) Image(w, h int, current color.Color) *image.RGBA {
	img := s.SVG.Image(w, h, current)
	for i := 0; i < len(img.Pix); i += 4 {
		// Premultiplied: the ink at the shapes' coverage, and its own alpha.
		a := uint32(img.Pix[i+3]) * uint32(s.ink.A) / 255
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(uint32(s.ink.R)*a/255), uint8(uint32(s.ink.G)*a/255), uint8(uint32(s.ink.B)*a/255), uint8(a)
	}
	return img
}
