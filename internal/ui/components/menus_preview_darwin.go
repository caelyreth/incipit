//go:build !mygo_nodownload

package components

import (
	"os"

	"github.com/egoist/mygo/ui"
)

// NEVERLAND_MENUS=flyout draws the menus on a Mac as Windows and Linux show
// them, to see them there. A release leaves this out: mygo build tags its
// builds mygo_nodownload.
func init() {
	if os.Getenv("NEVERLAND_MENUS") == "flyout" {
		menuPresentation = ui.FlyoutMenus
	}
}
