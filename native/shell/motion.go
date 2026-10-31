package shell

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/caelyreth/neverland/native/components"
)

// The library answers at the speed of a click: the sidebar is something
// the hand drives over and over, so its motion only says where things went
// and gets out of the way. Settings, dialogs and the editor keep their own
// pace.
const (
	// motionReveal opens and closes the sidebar.
	motionReveal = components.MotionPanel
	// motionPlace slides the sidebar from one place to the next.
	motionPlace = 220 * time.Millisecond
	// motionFold opens and closes a group in the tree.
	motionFold = components.MotionPanel
	// motionList moves rows and tabs as others come and go, and opens and
	// closes them in place.
	motionList = 150 * time.Millisecond
	// motionTurn turns a disclosure chevron.
	motionTurn = components.MotionQuick
)

// listTransition is how rows and tabs come, go and make room.
func listTransition(animate, size bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return ui.ElementTransition{
		Duration: motionList,
		Position: true, Size: size,
		Enter: &ui.Motion{Collapse: true},
		Exit:  &ui.Motion{Collapse: true},
	}
}
