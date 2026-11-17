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

func (el *Element) paintEdges(p *ui.Painter, view ui.Rect) {
	top, bottom := el.edgeFades()
	el.edges.Paint(p, view, el.Theme.Surface.Page, top, bottom)
}
