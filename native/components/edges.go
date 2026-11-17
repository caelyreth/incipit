package components

import "github.com/egoist/mygo/ui"

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
