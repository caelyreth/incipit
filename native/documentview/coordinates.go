package documentview

import "github.com/egoist/mygo/ui"

// pageCoordinates maps between the stationary viewport and the unscrolled
// layout. Horizontal table scrolling remains part of the layout itself.
type pageCoordinates struct {
	top, scroll float32
}

func (c pageCoordinates) contentY(y float32) float32  { return y + c.scroll - c.top }
func (c pageCoordinates) viewportY(y float32) float32 { return y + c.top - c.scroll }

func (c pageCoordinates) contentPoint(point ui.Point) ui.Point {
	point.Y = c.contentY(point.Y)
	return point
}

func (e *Element) contentPoint(x, y float32) ui.Point {
	return (pageCoordinates{top: e.TopInset, scroll: e.visibleScroll()}).contentPoint(ui.Point{X: x, Y: y})
}
