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
	// motionPlace fades the sheet list from one place to the next.
	motionPlace = 220 * time.Millisecond
	// motionFold opens and closes a group in the tree.
	motionFold = components.MotionPanel
	// motionList moves rows and tabs as others come and go, and opens and
	// closes them in place.
	motionList = 150 * time.Millisecond
	// motionTurn turns a disclosure chevron.
	motionTurn = components.MotionQuick
	// motionArm turns a button to ask for its second click.
	motionArm = components.MotionQuick
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

// fadeTransition swaps what stands in one place: the new rises in as the
// old fades.
func fadeTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return ui.ElementTransition{
		Duration: motionPlace,
		Enter:    &ui.Motion{Y: 6, Opacity: 0},
		Exit:     &ui.Motion{Opacity: 0},
	}
}

// popTransition lets something small appear and leave where it stands.
func popTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return ui.ElementTransition{
		Duration: motionList,
		Enter:    &ui.Motion{Scale: 0.9, Opacity: 0},
		Exit:     &ui.Motion{Scale: 0.9, Opacity: 0},
	}
}
