package components

import (
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
