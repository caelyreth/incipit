package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

// A unit changes shape when a surface opens or closes in it: a formula's
// source card, a math block's source panel or display, a figure's caption
// line. Its
// height then springs from where it was drawn to where it is going, the
// units after it gliding with it; what opened fades in as it does and
// what closed fades out where it stood. Any other change of height, as a
// line typed, takes effect at once.

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

// shapeOf is what a unit holds open: its surfaces, and what its object
// holds open besides.
func shapeOf(u *unitLayout) int {
	n := 0
	for _, s := range u.surfaces {
		// A grid's cells are its lines', which come in by themselves
		// (tableIn).
		if s.shown && (u.table == nil || !u.table.holdsCell(s)) {
			n += 4
		}
	}
	if u.kind != nil {
		n += u.kind.holds(u)
	}
	return n
}

// settleRooms draws each unit at its height on the way there, moving the
// units after it; it may run on a layout any number of times. Only a paint
// steps the rooms, at its own time: a view is built anew only when
// invalidated, and an animation frame only paints. A layout between paints,
// as an input is handled, draws each where it stands, so a change starts
// with the first paint that draws it. Before any paint, as when a page is
// first laid out, every unit stands at its height.
func (el *Element) settleRooms(pl *pageLayout, now time.Time, frame, animate bool) {
	if el.rooms == nil {
		el.rooms = map[doc.ID]*room{}
	}
	animate = animate && !now.IsZero()
	if el.roomsOf == pl && el.roomsSettled && el.roomsAnimate == animate {
		// Every room stands as the last pass left it: nothing to step. A
		// settled spring keeps its target however long it rests.
		return
	}
	if el.roomsOf != pl {
		// A new layout: the rooms of units it no longer has go.
		el.roomsOf = pl
		seen := make(map[doc.ID]bool, len(pl.units))
		for _, u := range pl.units {
			seen[u.pres.Unit] = true
		}
		for id := range el.rooms {
			if !seen[id] {
				delete(el.rooms, id)
			}
		}
	}
	shift := float32(0)
	moving := false
	for _, u := range pl.units {
		u.y = u.top + shift
		id := u.pres.Unit
		shape := shapeOf(u)
		r, ok := el.rooms[id]
		switch {
		case ok && !frame && animate:
			// Between frames: where it stands, a change it has yet to
			// start asking for the frame that starts it.
			moving = moving || r.shape != shape
		case !ok:
			r = &room{spring: motion.NewSpring(u.h, now), shape: shape}
			el.rooms[id] = r
		case !animate:
			*r = room{spring: motion.NewSpring(u.h, now), shape: shape}
		case r.shape != shape:
			r.from, r.prev, r.shape = r.spring.Value(), r.last, shape
			r.spring.Step(u.h, now, layoutSettle)
		case r.spring.Moving():
			r.spring.Step(u.h, now, layoutSettle)
		default:
			r.spring = motion.NewSpring(u.h, now)
		}
		if frame && !r.spring.Moving() {
			r.prev, r.from = nil, u.h
		}
		r.last = u
		u.shown = max(r.spring.Value(), 0)
		shift += u.shown - u.h
		moving = moving || r.spring.Moving()
	}
	pl.height = pl.rest + shift
	el.roomsMoving = moving || el.roomsMoving && !frame
	el.roomsSettled = frame && !el.roomsMoving
	el.roomsAnimate = animate
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
