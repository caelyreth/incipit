package documentview

import "github.com/egoist/mygo/ui"

// gripGesture distinguishes a click from a drag without moving the grip:
// the document, rather than the grip, paints what follows the pointer.
type gripGesture struct {
	owner    string
	origin   ui.Point
	dragging bool
	canceled bool
}

type gripEvent uint8

const (
	gripNone gripEvent = iota
	gripClick
	gripStart
	gripMove
	gripEnd
	gripCancel
)

// gripDragDistance is how far a pressed grip moves before it carries what
// it holds: twice the distance at which MyGo starts a value drag, so a
// click that wobbles still selects rather than lifting the block.
const gripDragDistance = 8

// follow reads a grip once per build. point is in the stationary viewport,
// not in the grip or the scrolling document.
func (g *gripGesture) follow(owner string, pressed, clicked, released bool, point ui.Point) (gripEvent, ui.Point) {
	if g.owner == "" {
		if pressed {
			*g = gripGesture{owner: owner, origin: point}
		} else if clicked {
			return gripClick, ui.Point{}
		}
		return gripNone, ui.Point{}
	}
	if g.owner != owner {
		return gripNone, ui.Point{}
	}
	moved := ui.Point{X: point.X - g.origin.X, Y: point.Y - g.origin.Y}
	if !pressed {
		dragging, canceled := g.dragging, g.canceled
		*g = gripGesture{}
		if canceled {
			return gripNone, moved
		}
		if dragging {
			if released {
				return gripEnd, moved
			}
			return gripCancel, moved
		}
		if clicked {
			return gripClick, moved
		}
		return gripNone, moved
	}
	if g.canceled {
		return gripNone, moved
	}
	if g.dragging {
		return gripMove, moved
	}
	if moved.X*moved.X+moved.Y*moved.Y >= gripDragDistance*gripDragDistance {
		g.dragging = true
		return gripStart, moved
	}
	return gripNone, moved
}

func (g *gripGesture) cancel() {
	if g.owner != "" {
		g.canceled = true
	}
}

// followGripGesture also asks MyGo to deliver frames as the pressed grip moves.
func (e *Element) followGripGesture(handle ui.Element, owner string) (gripEvent, ui.Point) {
	_, _, pressed := handle.Dragged()
	_, _, released := handle.DragEnded()
	return e.gesture.follow(owner, pressed, handle.Clicked(), released, e.gripPointer)
}

func (e *Element) handleGripInput(event ui.InputEvent) bool {
	if event.Kind != ui.InputKeyDown || event.Key != ui.KeyEscape || e.gesture.owner == "" {
		return false
	}
	e.gesture.cancel()
	bridge, _ := e.Input.(*Bridge)
	e.dropCarry(bridge, true, event.Time)
	e.tables.drag = nil
	return true
}
