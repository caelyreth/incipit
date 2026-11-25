package components

import "github.com/egoist/mygo/ui"

// menuPresentation keeps the system's menus on macOS, unless the settings
// choose the app's own.
var menuPresentation = ui.SystemMenus

// menuGlass is how much of the raised surface a menu's panel lays over
// the system's material of menus behind its flyout: enough to keep the
// app's tone, little enough for the frosted material to show through.
const menuGlass = 0.72
