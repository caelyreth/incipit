package shell

// The field is where the window opens and comes back to; the desk rests
// beside it, small, in its lane at the window's right edge, and comes up
// out of it to fill the window to be written at, the field going aside to
// the left. The window looks at the field or at the desk, or anywhere on
// the way between (at, from 0 on the field to 1 on the desk).

// stageLook is how a layer stands in the window: scaled by Scale about its
// centre, its centre moved by Offset, and as bright as Shown.
type stageLook struct {
	Scale, Shown float32
	Offset       stagePoint
}

// stagePoint is a place, in DIPs.
type stagePoint struct{ X, Y float32 }

// stageScene is how the field and the desk stand in a window.
type stageScene struct {
	Field, Desk stageLook
	// Upright is how far the desk has come up to be written at, from 0
	// where it rests to 1 filling the window.
	Upright float32
}

// stageAside is how far the field goes left, in parts of the window's width,
// and how much darker, as the desk comes up over it.
type stageAside struct{ Shift, Dim float32 }

// stageSized is the share of the way at either end over which the desk
// keeps the size it ends at: it comes to its size a little before its
// place. On the way it is one picture (leanDesk), drawn again at each step
// of scale it passes and, once its scale rests, at that scale itself; so
// its scale comes to rest while it still moves, the picture it settles in
// the lane as is the one it last moved as, and it never changes as the
// desk stops.
const stageSized = 0.03

// buildStage is the scene in a window w by h looked at from at, the desk
// resting at rest, in the window.
func buildStage(w, h, at float32, rest stageRect, a stageAside) stageScene {
	at = max(0, min(at, 1))
	away := 1 - at
	sized := max(0, min(1, (away-stageSized)/(1-2*stageSized)))
	k := stageLerp(1, rest.W/max(w, 1), sized)
	return stageScene{
		Field: stageLook{Scale: 1, Shown: 1 - at*a.Dim, Offset: stagePoint{X: -at * a.Shift * w}},
		Desk: stageLook{
			Scale:  k,
			Shown:  1,
			Offset: stagePoint{X: away * (rest.X + rest.W/2 - w/2), Y: away * (rest.Y + rest.H/2 - h/2)},
		},
		Upright: at,
	}
}
