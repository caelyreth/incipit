package stage

import "math"

// Posture names a spatial balance between the desk and the field.
// Postures are points along a continuous allocation rather than isolated modes.
type Posture uint8

const (
	// PostureWriting gives nearly the whole window to the desk while the
	// field rests behind and the pill stays quiet on the title line.
	PostureWriting Posture = iota
	// PostureGlance leans the desk aside to leave roughly half the window
	// for quick navigation and additions in the field.
	PostureGlance
	// PostureArrive gives most of the window to the field while the desk
	// leans far into the right/bottom-right corner.
	PostureArrive
)

// Scene is the top-level placement of the desk, the field, the field's
// unobstructed room, and the pill for a viewport and posture.
type Scene struct {
	Posture   Posture
	Desk      Placement
	Field     Placement
	FieldRoom Rect
	Pill      Placement
	// PillAnchor is the center point around which field cards pack, in
	// FieldRoom coordinates.
	PillAnchor Point
}

// TitleHeight is the fallback title-line height in DIPs when a caller passes
// zero.
const defaultTitleHeight float32 = 40

// Stage computes the placements for posture in viewport. When hasDesk is false
// (no open sheets), Glance and Arrive give the field the whole viewport.
// titleHeight positions the quiet pill on the title line in Writing.
func Stage(viewport Rect, posture Posture, hasDesk bool, titleHeight float32) Scene {
	if titleHeight <= 0 {
		titleHeight = defaultTitleHeight
	}
	w, h := max(0, viewport.W), max(0, viewport.H)

	switch posture {
	case PostureWriting:
		return Scene{
			Posture: posture,
			Desk: Placement{
				ID:          "desk",
				Kind:        KindDesk,
				Rect:        viewport,
				Scale:       1,
				Rotate:      0,
				Offset:      Point{},
				Depth:       0,
				Opacity:     1,
				Brightness:  1,
				Interactive: true,
			},
			Field: Placement{
				ID:          "field",
				Kind:        KindField,
				Rect:        viewport,
				Scale:       0.96,
				Opacity:     0,
				Brightness:  0.85,
				Depth:       -120,
				Interactive: false,
			},
			FieldRoom: viewport,
			Pill: Placement{
				ID:          "pill",
				Kind:        KindControl,
				Rect:        Rect{X: viewport.X + w/2, Y: viewport.Y + titleHeight/2},
				Scale:       0.92,
				Opacity:     0,
				Brightness:  1,
				Interactive: false,
			},
			PillAnchor: viewport.Center(),
		}

	case PostureGlance:
		desk := Placement{
			ID:          "desk",
			Kind:        KindDesk,
			Rect:        viewport,
			Scale:       0.72,
			Rotate:      5,
			Offset:      Point{X: 0.44 * w, Y: 0.04 * h},
			Depth:       -40,
			Opacity:     0.96,
			Brightness:  0.94,
			Interactive: hasDesk,
		}
		if !hasDesk {
			desk.Opacity = 0
			desk.Interactive = false
		}
		room := fieldRoomFor(viewport, desk, hasDesk)
		anchor := room.Center()
		return Scene{
			Posture: posture,
			Desk:    desk,
			Field: Placement{
				ID:          "field",
				Kind:        KindField,
				Rect:        viewport,
				Scale:       1,
				Opacity:     1,
				Brightness:  1,
				Depth:       0,
				Interactive: true,
			},
			FieldRoom: room,
			Pill: Placement{
				ID:          "pill",
				Kind:        KindControl,
				Rect:        Rect{X: anchor.X, Y: anchor.Y},
				Scale:       1,
				Opacity:     1,
				Brightness:  1,
				Interactive: true,
			},
			PillAnchor: anchor,
		}

	default: // PostureArrive
		desk := Placement{
			ID:          "desk",
			Kind:        KindDesk,
			Rect:        viewport,
			Scale:       0.54,
			Rotate:      11,
			Offset:      Point{X: 0.48 * w, Y: 0.16 * h},
			Depth:       -80,
			Opacity:     0.92,
			Brightness:  0.90,
			Interactive: hasDesk,
		}
		if !hasDesk {
			desk.Opacity = 0
			desk.Interactive = false
		}
		room := fieldRoomFor(viewport, desk, hasDesk)
		anchor := room.Center()
		return Scene{
			Posture: posture,
			Desk:    desk,
			Field: Placement{
				ID:          "field",
				Kind:        KindField,
				Rect:        viewport,
				Scale:       1,
				Opacity:     1,
				Brightness:  1,
				Depth:       0,
				Interactive: true,
			},
			FieldRoom: room,
			Pill: Placement{
				ID:          "pill",
				Kind:        KindControl,
				Rect:        Rect{X: anchor.X, Y: anchor.Y},
				Scale:       1,
				Opacity:     1,
				Brightness:  1,
				Interactive: true,
			},
			PillAnchor: anchor,
		}
	}
}

// BlendScene interpolates smoothly between from and to postures by t.
func BlendScene(viewport Rect, from, to Posture, t float32, hasDesk bool, titleHeight float32) Scene {
	if t <= 0 {
		return Stage(viewport, from, hasDesk, titleHeight)
	}
	if t >= 1 {
		return Stage(viewport, to, hasDesk, titleHeight)
	}
	a := Stage(viewport, from, hasDesk, titleHeight)
	b := Stage(viewport, to, hasDesk, titleHeight)
	posture := to
	if t < 0.5 {
		posture = from
	}
	room := Rect{
		X: lerp(a.FieldRoom.X, b.FieldRoom.X, t),
		Y: lerp(a.FieldRoom.Y, b.FieldRoom.Y, t),
		W: max(0, lerp(a.FieldRoom.W, b.FieldRoom.W, t)),
		H: max(0, lerp(a.FieldRoom.H, b.FieldRoom.H, t)),
	}
	anchor := Point{
		X: lerp(a.PillAnchor.X, b.PillAnchor.X, t),
		Y: lerp(a.PillAnchor.Y, b.PillAnchor.Y, t),
	}
	return Scene{
		Posture:    posture,
		Desk:       BlendPlacement(a.Desk, b.Desk, t),
		Field:      BlendPlacement(a.Field, b.Field, t),
		FieldRoom:  room,
		Pill:       BlendPlacement(a.Pill, b.Pill, t),
		PillAnchor: anchor,
	}
}

// fieldRoomFor computes the clear space left to the left of the leaned desk.
// It uses the desk's rotated bounding box so neither cards nor the pill sit
// underneath the leaned desk edge.
func fieldRoomFor(viewport Rect, desk Placement, hasDesk bool) Rect {
	if !hasDesk || desk.Opacity <= 0.01 || viewport.Empty() {
		return viewport
	}
	left := DeskVisualBounds(desk).X
	clearW := max(viewport.W*0.35, min(viewport.W, left-viewport.X))
	return Rect{
		X: viewport.X,
		Y: viewport.Y,
		W: clearW,
		H: viewport.H,
	}
}

// DeskVisualBounds returns the axis-aligned bounding box of the desk after
// applying its scale, clockwise rotation around its center, and offset.
func DeskVisualBounds(desk Placement) Rect {
	w, h := desk.Rect.W*desk.Scale, desk.Rect.H*desk.Scale
	rad := float64(desk.Rotate) * math.Pi / 180
	sin, cos := math.Abs(math.Sin(rad)), math.Abs(math.Cos(rad))
	bw := float32(float64(w)*cos + float64(h)*sin)
	bh := float32(float64(w)*sin + float64(h)*cos)
	c := desk.Rect.Center()
	cx := c.X + desk.Offset.X
	cy := c.Y + desk.Offset.Y
	return Rect{
		X: cx - bw/2,
		Y: cy - bh/2,
		W: bw,
		H: bh,
	}
}
