package shell

// stageSize is a width and a height, in DIPs.
type stageSize struct{ W, H float32 }

// stageRect is a box, in DIPs.
type stageRect struct{ X, Y, W, H float32 }

// Inset is r less d on each side.
func (r stageRect) Inset(d float32) stageRect {
	return stageRect{X: r.X + d, Y: r.Y + d, W: max(0, r.W-2*d), H: max(0, r.H-2*d)}
}

// Scaled is r scaled by k about its centre.
func (r stageRect) Scaled(k float32) stageRect {
	w, h := r.W*k, r.H*k
	return stageRect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}
}

// Empty reports a box without area.
func (r stageRect) Empty() bool { return r.W <= 0 || r.H <= 0 }

func stageLerp(a, b, t float32) float32 { return a + (b-a)*t }

func stageLerpRect(a, b stageRect, t float32) stageRect {
	return stageRect{X: stageLerp(a.X, b.X, t), Y: stageLerp(a.Y, b.Y, t), W: stageLerp(a.W, b.W, t), H: stageLerp(a.H, b.H, t)}
}
