package stage

// Size is a width and a height, in DIPs.
type Size struct{ W, H float32 }

// Rect is a box, in DIPs.
type Rect struct{ X, Y, W, H float32 }

// Inset is r less d on each side.
func (r Rect) Inset(d float32) Rect {
	return Rect{X: r.X + d, Y: r.Y + d, W: max(0, r.W-2*d), H: max(0, r.H-2*d)}
}

// Scaled is r scaled by k about its centre.
func (r Rect) Scaled(k float32) Rect {
	w, h := r.W*k, r.H*k
	return Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}
}

// Empty reports a box without area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

func lerpRect(a, b Rect, t float32) Rect {
	return Rect{X: lerp(a.X, b.X, t), Y: lerp(a.Y, b.Y, t), W: lerp(a.W, b.W, t), H: lerp(a.H, b.H, t)}
}
