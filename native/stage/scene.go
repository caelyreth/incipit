package stage

// The field is where the window opens and comes back to; the desk rests
// beside it, small, in its lane at the window's right edge, and comes up
// out of it to fill the window to be written at, the field going aside to
// the left. The window looks at the field or at the desk, or anywhere on
// the way between (at, from 0 on the field to 1 on the desk).

// Look is how a layer stands in the window: scaled by Scale about its
// centre, its centre moved by Offset, and as bright as Shown.
type Look struct {
	Scale, Shown float32
	Offset       Point
}

// Point is a place, in DIPs.
type Point struct{ X, Y float32 }

// Scene is how the field and the desk stand in a window.
type Scene struct {
	Field, Desk Look
	// Upright is how far the desk has come up to be written at, from 0
	// where it rests to 1 filling the window.
	Upright float32
}

// Aside is how far the field goes left, in parts of the window's width,
// and how much darker, as the desk comes up over it.
type Aside struct{ Shift, Dim float32 }

// Stage is the scene in a window w by h looked at from at, the desk
// resting at rest, in the window.
func Stage(w, h, at float32, rest Rect, a Aside) Scene {
	at = max(0, min(at, 1))
	away := 1 - at
	k := lerp(1, rest.W/max(w, 1), away)
	return Scene{
		Field: Look{Scale: 1, Shown: 1 - at*a.Dim, Offset: Point{X: -at * a.Shift * w}},
		Desk: Look{
			Scale:  k,
			Shown:  1,
			Offset: Point{X: away * (rest.X + rest.W/2 - w/2), Y: away * (rest.Y + rest.H/2 - h/2)},
		},
		Upright: at,
	}
}
