package motion

import (
	"github.com/egoist/mygo/ui"
	"time"
)

type rectangle struct {
	axes []Spring
}

func (r *rectangle) sample(target ui.Rect, now time.Time, frequency float32) ui.Rect {
	values := [4]float32{target.X, target.Y, target.W, target.H}
	if r.axes == nil {
		r.axes = make([]Spring, 4)
		for index, value := range values {
			r.axes[index] = NewSpring(value, now)
		}
	}
	var current [4]float32
	for index := range values {
		current[index] = r.axes[index].Step(values[index], now, frequency)
	}
	return ui.Rect{
		X: current[0],
		Y: current[1],
		W: max(0, current[2]),
		H: max(0, current[3]),
	}
}

// start places the rectangle at from, to move on from there.
func (r *rectangle) start(from ui.Rect, now time.Time) {
	r.axes = []Spring{
		NewSpring(from.X, now), NewSpring(from.Y, now),
		NewSpring(from.W, now), NewSpring(from.H, now),
	}
}

func (r *rectangle) row() (float32, bool) {
	if r.axes == nil {
		return 0, false
	}
	return r.axes[1].Target(), true
}

func (r *rectangle) moving() bool {
	if r.axes == nil {
		return false
	}
	for index := range r.axes {
		if r.axes[index].Moving() {
			return true
		}
	}
	return false
}
