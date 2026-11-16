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

// ElevatedSurface gives an element the shared flat raised material—crisp
// border and elevated surface fill without drop shadows—while leaving its
// contents, interaction, and transition to its owner.
func ElevatedSurface(element ui.Element, semantic theme.Theme, scale float32, _ Elevation) ui.Element {
	return element.Radius(theme.DefaultControls.Scaled(scale).PanelRadius).
		Border(1, semantic.Border.Default).
		Background(semantic.Surface.Elevated)
}

// OutlinedSurface is the raised material without a shadow, set apart by
// its border alone: a popover that opens over the text it serves, as the
// page's tag picker, and should read as part of the page.
func OutlinedSurface(element ui.Element, semantic theme.Theme, scale float32) ui.Element {
	return element.Radius(theme.DefaultControls.Scaled(scale).PanelRadius).
		Border(1, semantic.Border.Default).
		Background(semantic.Surface.Elevated)
}
