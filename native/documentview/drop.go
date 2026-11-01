package documentview

import (
	"time"

	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

// While pictures are dragged over the page, a line between its blocks
// shows where they would go: below the block under the pointer. It fades
// in where it first shows, glides as the pointer moves from block to
// block, and fades out where it was as the drag leaves or lands.

const (
	dropGlide = float32(26)
	dropFade  = float32(16)
	// dropWeight is the line's thickness, in DIPs.
	dropWeight = 2
)

type dropLine struct {
	y, opacity motion.Spring
	known      bool
}

// advance moves the line toward y, shown or not, and reports whether it
// is still moving.
func (d *dropLine) advance(y float32, shown bool, now time.Time, animate bool) bool {
	target := float32(0)
	if shown {
		target = 1
	}
	if !d.known || !animate || d.opacity.Value() <= 0.001 && shown {
		d.y = motion.NewSpring(y, now)
		if !animate {
			d.opacity = motion.NewSpring(target, now)
		}
		d.known = true
	}
	if animate {
		if shown {
			d.y.Step(y, now, dropGlide)
		} else {
			d.y.Step(d.y.Target(), now, dropGlide)
		}
		d.opacity.Step(target, now, dropFade)
	}
	return animate && (d.y.Moving() || d.opacity.Moving())
}

func drawDropLine(p *ui.Painter, rect ui.Rect, state *State) {
	line := state.Drop
	if line == nil || line.opacity.Value() <= 0.001 {
		return
	}
	y := rect.Y + state.TopInset - state.ScrollY + line.y.Value() - dropWeight/2
	p.Fill(ui.Rect{X: rect.X, Y: y, W: rect.W, H: dropWeight},
		state.Theme.Accent.Foreground.Alpha(line.opacity.Value()), dropWeight/2)
}
