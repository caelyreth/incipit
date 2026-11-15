package page

import "github.com/egoist/mygo/ui"

// The page's text fades out at the top and bottom of its view, so a line
// scrolled out leaves softly and the view reads as going on. The fades are
// drawn over its opaque background: they stand still over the view, and
// the text moves through them. Each lies within
// the room the page leaves past its text, so a page at its top or its end
// shows its first or last line whole and the fade comes in only as text
// scrolls into it. At the foot the fade reaches up as far as the dock
// floating there, so the dock stands over text already fading. A revealed
// caret's line stays clear of both.

// edgeFades are how deep the fades at the view's top and bottom are: half
// the room above the first line, and the dock's clearance, within the
// room below the last.
func (el *Element) edgeFades() (top, bottom float32) {
	return el.Top / 2, min(el.Dock, el.Bottom)
}

type edgeFade struct {
	ink         ui.Color
	top, bottom ui.LinearGradient
}

func (el *Element) paintEdges(p *ui.Painter, view ui.Rect) {
	top, bottom := el.edgeFades()
	if view.H <= 0 || top+bottom <= 0 || top+bottom >= view.H {
		return
	}
	fade := &el.edges
	if ink := el.Theme.Surface.Page; fade.ink != ink {
		fade.ink = ink
		fade.top = edgeGradient(ink, true)
		fade.bottom = edgeGradient(ink, false)
	}
	if top > 0 {
		p.FillGradient(ui.Rect{X: view.X, Y: view.Y, W: view.W, H: top}, fade.top, 0)
	}
	if bottom > 0 {
		p.FillGradient(ui.Rect{X: view.X, Y: view.Y + view.H - bottom, W: view.W, H: bottom}, fade.bottom, 0)
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
