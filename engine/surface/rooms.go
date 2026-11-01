package surface

import (
	"slices"

	"github.com/caelyreth/neverland/engine/model"
)

// Room is space a view opens before the line at At, as a drag parts the
// page where what it carries would land; At of -1 opens it after the last
// line.
type Room struct {
	At     model.UTF16Offset
	Height float32
}

// WithRooms is f with rooms opened in it.
func (f *Flow) WithRooms(rooms []Room) *Flow {
	next := *f
	next.rooms = slices.Clone(rooms)
	return &next
}

// RoomAt is the room open before the line at at, -1 for after the last.
func (f *Flow) RoomAt(at model.UTF16Offset) float32 {
	room := float32(0)
	for _, candidate := range f.rooms {
		if candidate.At == at {
			room += candidate.Height
		}
	}
	return room
}
