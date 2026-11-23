package components

import "github.com/egoist/mygo/ui"

// menuPresentation keeps the system's menus on macOS, which a build for
// development may draw as the other platforms do (menus_preview_darwin.go).
var menuPresentation = ui.SystemMenus
