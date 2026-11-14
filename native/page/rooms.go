package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/egoist/mygo/ui"
)

// The page's units are a flow (flows.go), down the page: an object
// entering comes in, one leaving goes, one taking or giving a text's place
// crosses with it (a table made of a row written between pipes, a figure
// placed in an empty line, and each undone), and units reordered glide to
// their places. A unit changes shape when a surface opens or closes in it:
// a formula's source card, a math block's source panel or display, a
// figure's caption line. Its height then springs from where it was drawn
// to where it is going, the units after it gliding with it; what opened
// fades in as it does and what closed fades out where it stood. A text
// resizing grows to its size by itself (sizes.go); any other change of
// height, as a line typed, takes effect at once.

// room is what the page keeps of a unit's shape across frames: what it
// holds open; from, the height drawn when that last changed; and prev, the
// layout it changed from, whose closed surfaces fade out while it closes.
type room struct {
	shape int
	from  float32
	prev  *unitLayout
	last  *unitLayout
}

// left is a unit that left the page, drawn as it was kept at y in the
// page, within clip of its height, as strongly as alpha.
type left struct {
	u              *unitLayout
	y, clip, alpha float32
}

// shapeOf is what a unit holds open: its surfaces, and what its object
// holds open besides.
func shapeOf(u *unitLayout) int {
	n := 0
	for _, s := range u.surfaces {
		// A grid's cells are its lines', which come in by themselves.
		if s.shown && (u.table == nil || !u.table.holdsCell(s)) {
			n += 4
		}
	}
	if u.kind != nil {
		n += u.kind.holds(u)
	}
	return n
}

// settleRooms draws each unit as the page's flow has it on its way, moving
// the units after it; it may run on a layout any number of times. Only a
// paint tells the flow and steps it, at its own time: a view is built anew
// only when invalidated, and an animation frame only paints. A layout
// between paints, as an input is handled, draws each where it stands, so a
// change starts with the first paint that draws it. Before any paint, as
// when a page is first laid out, every unit stands at its height.
func (el *Element) settleRooms(pl *pageLayout, now time.Time, frame, animate bool) {
	if el.rooms == nil {
		el.rooms = map[doc.ID]*room{}
	}
	animate = animate && !now.IsZero()
	if el.roomsOf == pl && el.roomsSettled && el.roomsAnimate == animate {
		// Every unit stands as the last pass left it: nothing to step.
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
	f := &el.flow
	moving := false
	if frame {
		// What the layout holds, each where it stands after what is drawn
		// before it as it is drawn now; objects come and go with motion.
		in := make([]flowIn, len(pl.units))
		shift := float32(0)
		for i, u := range pl.units {
			id := u.pres.Unit
			shape := shapeOf(u)
			extent, _, _ := f.drawn(id, u.h)
			r := el.rooms[id]
			switch {
			case r == nil:
				r = &room{shape: shape, from: u.h}
				el.rooms[id] = r
			case r.shape != shape:
				r.from, r.prev, r.shape = extent, r.last, shape
			}
			in[i] = flowIn{id: id, place: u.top + shift, extent: u.h, shape: shape, moves: u.kind != nil, kept: u}
			shift += extent - u.h
		}
		f.tell(in, now, animate)
		moving = f.step(now, animate)
	}
	gap := el.Editor.metrics.Gap
	el.left = el.left[:0]
	shift := float32(0)
	// What left after after stands past end, each taking the room it
	// closes from, with the gap it had.
	leave := func(after doc.ID, end float32) {
		for _, x := range f.leaving(after) {
			if x.cover != 0 {
				continue
			}
			room := x.extent.Value() / x.size * (x.size + gap)
			el.left = append(el.left, left{u: x.kept.(*unitLayout), y: end + shift + gap*room/(x.size+gap), clip: x.extent.Value(), alpha: x.alpha.Value()})
			shift += room
		}
	}
	leave(0, -gap)
	for _, u := range pl.units {
		id := u.pres.Unit
		extent, alpha, offset := f.drawn(id, u.h)
		u.y = u.top + shift + offset
		u.shown, u.fade = extent, alpha
		for _, x := range f.exits {
			if x.cover == id {
				el.left = append(el.left, left{u: x.kept.(*unitLayout), y: u.y + x.lead, clip: max(extent-x.lead, 0), alpha: x.alpha.Value()})
			}
		}
		shift += extent - u.h
		if frame {
			f.saw(id, u.y)
			r := el.rooms[id]
			if extent == u.h && alpha >= 1 {
				r.prev, r.from = nil, u.h
			}
			r.last = u
		}
		leave(id, u.top+u.h)
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
