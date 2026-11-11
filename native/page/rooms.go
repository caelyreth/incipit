package page

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

// A unit changes shape when a surface opens or closes in it: a formula's
// source card, a math block's source panel, a figure's caption line. Its
// height then springs from where it was drawn to where it is going, the
// units after it gliding with it; what opened fades in as it does and
// what closed fades out where it stood. Any other change of height, as a
// line typed, takes effect at once.

// roomFrequency springs a unit's height as its shape changes.
const roomFrequency = float32(14)

// room is a unit's drawn height across frames.
type room struct {
	spring motion.Spring
	// shape is what the unit holds open; from is the height drawn when it
	// last changed, and prev the layout it changed from, whose closed
	// surfaces fade out while it closes.
	shape int
	from  float32
	prev  *unitLayout
	last  *unitLayout
}

// shapeOf is what a unit holds open: its surfaces, and a math block's
// panel.
func shapeOf(u *unitLayout) int {
	n := len(u.surfaces) * 2
	if u.math != nil && u.math.held {
		n++
	}
	return n
}

// settleRooms draws each unit at its height on the way there, moving the
// units after it. Only a frame steps the rooms: a layout between frames,
// as an input is handled, draws each where it stands, so a change starts
// with the first frame that draws it rather than at the last frame's
// time. Without a frame's time, as when a page is first laid out, every
// unit stands at its height.
func (el *Element) settleRooms(pl *pageLayout, frame bool) {
	if el.rooms == nil {
		el.rooms = map[doc.ID]*room{}
	}
	animate := el.animate && !el.now.IsZero()
	seen := make(map[doc.ID]bool, len(pl.units))
	shift := float32(0)
	moving := false
	for _, u := range pl.units {
		u.y += shift
		id := u.pres.Unit
		seen[id] = true
		shape := shapeOf(u)
		r, ok := el.rooms[id]
		switch {
		case ok && !frame && animate:
			// Between frames: where it stands.
		case !ok || !animate:
			r = &room{spring: motion.NewSpring(u.h, el.now), shape: shape}
			el.rooms[id] = r
		case r.shape != shape:
			r.from, r.prev, r.shape = r.spring.Value(), r.last, shape
			r.spring.Step(u.h, el.now, roomFrequency)
		case r.spring.Moving():
			r.spring.Step(u.h, el.now, roomFrequency)
		default:
			r.spring = motion.NewSpring(u.h, el.now)
		}
		if frame && !r.spring.Moving() {
			r.prev, r.from = nil, u.h
		}
		r.last = u
		u.shown = max(r.spring.Value(), 0)
		shift += u.shown - u.h
		moving = moving || r.spring.Moving()
	}
	pl.height += shift
	for id := range el.rooms {
		if !seen[id] {
			delete(el.rooms, id)
		}
	}
	el.roomsMoving = moving || el.roomsMoving && !frame
}

// opened is how far what opened in a unit has: 1 once it stands open.
func (el *Element) opened(u *unitLayout) float32 {
	r, ok := el.rooms[u.pres.Unit]
	if !ok || u.h <= r.from {
		return 1
	}
	return min(max((u.shown-r.from)/(u.h-r.from), 0), 1)
}

// closing is the layout a unit closed from and how far it still shows,
// while it closes.
func (el *Element) closing(u *unitLayout) (*unitLayout, float32) {
	r, ok := el.rooms[u.pres.Unit]
	if !ok || r.prev == nil || r.from <= u.h {
		return nil, 0
	}
	return r.prev, min(max((u.shown-u.h)/(r.from-u.h), 0), 1)
}

// veil fades what is drawn under it into the page: a surface opening or
// closing shows at a, its text and fill alike.
func (el *Element) veil(p *ui.Painter, r ui.Rect, a float32) {
	if a < 0.999 {
		p.Fill(r, el.Theme.Surface.Page.Alpha(1-a), 0)
	}
}
