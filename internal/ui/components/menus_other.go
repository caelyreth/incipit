//go:build !darwin

package components

import "github.com/egoist/mygo/ui"

// menuPresentation draws the menus in flyouts on Windows and Linux, where
// the system's would stand apart from the app's look.
var menuPresentation = ui.FlyoutMenus

// menuGlass is how much of the raised surface a menu's panel lays over
// what is behind its flyout: all of it, as no material of the system's
// is sure to be there.
const menuGlass = 1
