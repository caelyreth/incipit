package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Elevation names the two raised surfaces used by the app. Popovers keep a
// stronger, fixed-DIP shadow; dialogs and notices scale with the UI.
type Elevation uint8

const (
	PanelElevation Elevation = iota
	PopoverElevation
)

// ElevatedSurface gives an element the shared raised material while leaving
// its contents, interaction, and transition to its owner.
func ElevatedSurface(element ui.Element, semantic theme.Theme, scale float32, elevation Elevation) ui.Element {
	y, blur, alpha := 4*scale/theme.DefaultUIFontSize, 12*scale/theme.DefaultUIFontSize, float32(0.25)
	if elevation == PopoverElevation {
		y, blur, alpha = 8, 24, 0.28
	}
	return element.Radius(theme.DefaultControls.Scaled(scale).PanelRadius).
		Border(1, semantic.Border.Default).
		Background(semantic.Surface.Elevated).
		Shadow(0, y, blur, 0, ui.RGBA(0, 0, 0, alpha))
}
