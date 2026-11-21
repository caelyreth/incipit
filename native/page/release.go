package page

// A long page keeps the shaping of the text near the view only: a unit
// more than keepViews views above or below it lets its surfaces' shaping
// go, keeping their sizes, and whatever reads one again, a caret moved
// there or the page scrolled to it, shapes it again, the same. A unit read
// while it is far, as one a highlight across the page covers, is held
// shaped until it comes near again, so no paint shapes it anew each time.

// keepViews is how many views either side of the view stay shaped.
const keepViews = 2

// releaseFar lets go the shaping of pl's units far from the view, top to
// bottom on the screen, the page's top at oy.
func releaseFar(pl *pageLayout, oy, top, bottom float32) {
	band := keepViews * (bottom - top)
	for _, u := range pl.units {
		if uy := oy + u.y; uy+u.h >= top-band && uy <= bottom+band {
			u.far, u.held = false, false
			continue
		}
		switch {
		case u.held:
		case u.far:
			u.held = shapedAgain(u)
		default:
			u.far = true
			for _, s := range u.surfaces {
				if s.layout != nil {
					s.layout.Release()
				}
			}
		}
	}
}

// shapedAgain is whether a surface of u was shaped again since it was let
// go.
func shapedAgain(u *unitLayout) bool {
	for _, s := range u.surfaces {
		if s.layout != nil && s.layout.Shaped() {
			return true
		}
	}
	return false
}
