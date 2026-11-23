package page

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/egoist/mygo/ui"
)

// paintText uses the app's stable raster origin for caller-owned layouts.
// Its logical origin stays shared with decorations and input geometry.
func paintText(p *ui.Painter, layout *ui.TextLayout, x, y float32, ink ui.Color) {
	components.WithTextOrigin(p, ui.Point{X: x, Y: y}, func(origin ui.Point) {
		p.TextLayout(layout, origin.X, origin.Y, ink)
	})
}
