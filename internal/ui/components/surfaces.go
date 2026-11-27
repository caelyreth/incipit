package components

import (
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Rim is the hairline a framed box is drawn in. It is laid out inside the
// box and takes room from what the box holds, and an absolute child is
// placed from inside it: sized from the box's outer edge, its children
// would stand the rim's width off-centre and press into it on one side.
const Rim = float32(1)

// Within is the room inside a box outer across, framed in a Rim and pad
// in from it on either side: what its children are laid out in.
func Within(outer, pad float32) float32 { return outer - 2*(Rim+pad) }

// Around is how far across a box framed in a Rim is to hold inner, pad
// in from the rim on either side: the inverse of Within.
func Around(inner, pad float32) float32 { return inner + 2*(Rim+pad) }

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
		Border(Rim, semantic.Border.Default).
		Background(semantic.Surface.Elevated)
}

// OutlinedSurface is the raised material without a shadow, set apart by
// its border alone: a popover that opens over the text it serves, as the
// page's tag picker, and should read as part of the page.
func OutlinedSurface(element ui.Element, semantic theme.Theme, scale float32) ui.Element {
	return element.Radius(theme.DefaultControls.Scaled(scale).PanelRadius).
		Border(Rim, semantic.Border.Default).
		Background(semantic.Surface.Elevated)
}

// EdgeFade caches the top and bottom eased fade gradients for a background
// color so scrolled content softens at the view's edges without rebuilding
// gradient stops on every frame.
type EdgeFade struct {
	ink         ui.Color
	top, bottom ui.LinearGradient
}

// Paint draws the top and bottom edge fades of height top and bottom over
// view in ink.
func (f *EdgeFade) Paint(p *ui.Painter, view ui.Rect, ink ui.Color, top, bottom float32) {
	if view.H <= 0 || top+bottom <= 0 || top+bottom >= view.H {
		return
	}
	if f.ink != ink || len(f.top.Stops) == 0 {
		f.ink = ink
		f.top = edgeGradient(ink, true)
		f.bottom = edgeGradient(ink, false)
	}
	if top > 0 {
		p.FillGradient(ui.Rect{X: view.X, Y: view.Y, W: view.W, H: top}, f.top, 0)
	}
	if bottom > 0 {
		p.FillGradient(ui.Rect{X: view.X, Y: view.Y + view.H - bottom, W: view.W, H: bottom}, f.bottom, 0)
	}
}

func edgeGradient(ink ui.Color, top bool) ui.LinearGradient {
	ease := [...]float32{0, 0.156, 0.5, 0.844, 1}
	stops := make([]ui.GradientStop, len(ease))
	for index, alpha := range ease {
		if top {
			alpha = 1 - alpha
		}
		stops[index] = ui.GradientStop{Color: ink.Alpha(alpha), At: float32(index) / 4}
	}
	return ui.LinearGradient{Angle: 180, Stops: stops}
}

type DividerWeight uint8

const (
	DefaultDivider DividerWeight = iota + 1
	VariantDivider
	FadedDivider
)

func DividerColor(semantic theme.Theme, weight DividerWeight) ui.Color {
	switch weight {
	case VariantDivider:
		return semantic.Border.Variant
	case FadedDivider:
		return semantic.Border.Default.Alpha(0.6)
	default:
		return semantic.Border.Default
	}
}

func HorizontalDivider(
	c *ui.Context,
	semantic theme.Theme,
	weight DividerWeight,
) ui.Element {
	return ui.Box(c).WidthPercent(100).Height(1).Shrink(0).
		Background(DividerColor(semantic, weight))
}

func VerticalDivider(
	c *ui.Context,
	semantic theme.Theme,
) ui.Element {
	return ui.Box(c).Width(1).Height(16 * UIFontSize(c) / theme.DefaultUIFontSize).
		Shrink(0).Background(DividerColor(semantic, DefaultDivider))
}
