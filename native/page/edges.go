package page

import "github.com/egoist/mygo/ui"

// The page's text fades out at the top and bottom of its view, so a line
// scrolled out leaves softly and the view reads as going on. The fades are
// a mask on what the page draws, set as the view is built: they stand
// still over the view, and the text moves through them. Each lies within
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

// edgeMask is a view's mask, kept while its fades and height stand, as
// the view is built at every scroll.
type edgeMask struct {
	top, bottom, h float32
	g              ui.LinearGradient
	ok             bool
}

// edgeMask is the mask the fades make over a view h tall, false where
// there is none.
func (el *Element) edgeMask(h float32) (ui.LinearGradient, bool) {
	top, bottom := el.edgeFades()
	if m := &el.mask; m.top != top || m.bottom != bottom || m.h != h {
		m.g, m.ok = fadeMask(top, bottom, h)
		m.top, m.bottom, m.h = top, bottom, h
	}
	return el.mask.g, el.mask.ok
}

// fadeMask is the mask of fades top and bottom deep over a view h tall.
func fadeMask(top, bottom, h float32) (ui.LinearGradient, bool) {
	if h <= 0 || top+bottom <= 0 || top+bottom >= h {
		return ui.LinearGradient{}, false
	}
	var stops []ui.GradientStop
	// Each fade eases in and out (smoothstep), so neither its ends nor
	// where the text stands whole show an edge.
	ease := [...]float32{0, 0.156, 0.5, 0.844, 1}
	for k, a := range ease {
		stops = append(stops, ui.GradientStop{Color: ui.RGB(0, 0, 0).Alpha(a), At: top / h * float32(k) / 4})
	}
	for k := len(ease) - 1; k >= 0; k-- {
		stops = append(stops, ui.GradientStop{Color: ui.RGB(0, 0, 0).Alpha(ease[k]), At: 1 - bottom/h*float32(k)/4})
	}
	return ui.LinearGradient{Angle: 180, Stops: stops}, true
}
