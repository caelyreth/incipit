package stage

// Point is a position in DIPs.
type Point struct{ X, Y float32 }

// Size is a width and height in DIPs.
type Size struct{ W, H float32 }

// Rect is an axis-aligned rectangle in DIPs.
type Rect struct{ X, Y, W, H float32 }

// Center returns the midpoint of r.
func (r Rect) Center() Point { return Point{X: r.X + r.W/2, Y: r.Y + r.H/2} }

// Empty reports whether r has zero or negative area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Intersects reports whether r and other overlap in area.
func (r Rect) Intersects(other Rect) bool {
	if r.Empty() || other.Empty() {
		return false
	}
	return r.X < other.X+other.W && r.X+r.W > other.X &&
		r.Y < other.Y+other.H && r.Y+r.H > other.Y
}

// Inset shrinks r by dx on each horizontal side and dy on each vertical side.
func (r Rect) Inset(dx, dy float32) Rect {
	return Rect{
		X: r.X + dx,
		Y: r.Y + dy,
		W: max(0, r.W-2*dx),
		H: max(0, r.H-2*dy),
	}
}

// Kind identifies what kind of top-level or field object a Placement positions.
type Kind uint8

const (
	KindDesk Kind = iota + 1
	KindField
	KindControl
	KindCard
)

// CardKind identifies what a card in the field stands for.
type CardKind uint8

const (
	CardGroup CardKind = iota + 1
	CardSheet
	CardNew
)

// SizeClass is a card's discrete cell footprint on the field's unit grid.
type SizeClass uint8

const (
	Size1x1 SizeClass = iota
	Size2x1
	Size2x2
)

// Cells returns the horizontal and vertical grid cell counts of s.
func (s SizeClass) Cells() (w, h int) {
	switch s {
	case Size2x1:
		return 2, 1
	case Size2x2:
		return 2, 2
	default:
		return 1, 1
	}
}

// Placement is the computed layout and visual state of one object.
// Views draw objects where their placements say without making layout choices.
type Placement struct {
	ID          string
	Kind        Kind
	Rect        Rect
	Scale       float32
	Rotate      float32
	Offset      Point
	Depth       float32
	Opacity     float32
	Brightness  float32
	Interactive bool
}

// BlendPlacement interpolates from toward to by t (typically 0..1).
func BlendPlacement(from, to Placement, t float32) Placement {
	id := to.ID
	if id == "" {
		id = from.ID
	}
	kind := to.Kind
	if kind == 0 {
		kind = from.Kind
	}
	opacity := clamp01(lerp(from.Opacity, to.Opacity, t))
	interactive := to.Interactive
	if t < 0.5 {
		interactive = from.Interactive
	}
	if opacity <= 0.01 {
		interactive = false
	}
	return Placement{
		ID:   id,
		Kind: kind,
		Rect: Rect{
			X: lerp(from.Rect.X, to.Rect.X, t),
			Y: lerp(from.Rect.Y, to.Rect.Y, t),
			W: max(0, lerp(from.Rect.W, to.Rect.W, t)),
			H: max(0, lerp(from.Rect.H, to.Rect.H, t)),
		},
		Scale:       max(0, lerp(from.Scale, to.Scale, t)),
		Rotate:      lerp(from.Rotate, to.Rotate, t),
		Offset:      Point{X: lerp(from.Offset.X, to.Offset.X, t), Y: lerp(from.Offset.Y, to.Offset.Y, t)},
		Depth:       lerp(from.Depth, to.Depth, t),
		Opacity:     opacity,
		Brightness:  max(0, lerp(from.Brightness, to.Brightness, t)),
		Interactive: interactive,
	}
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

func clamp01(v float32) float32 { return max(0, min(v, 1)) }
