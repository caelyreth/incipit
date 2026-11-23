//go:build !darwin

package components

import "github.com/egoist/mygo/ui"

// menuPresentation draws the menus in flyouts on Windows and Linux, where
// the system's would stand apart from the app's look.
var menuPresentation = ui.FlyoutMenus
