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
	// motionPlace moves the place chips from one place to the next.
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
	// motionEnter brings one row of a newly shown list in; each waits
	// enterStagger after the one above, up to enterRanks rows, so a long
	// list is in as soon as a short one. enterRise is how far a row rises.
	motionEnter  = 180 * time.Millisecond
	enterStagger = 24 * time.Millisecond
	enterRanks   = 8
	enterRise    = float32(8)
	// enterSlide is how far a row slides in from the side its place lies
	// on.
	enterSlide = float32(24)
	// dwellOpen is how long the pointer rests by the head of a list before
	// a create area opens there, and dwellClose how long it is away before
	// the area closes.
	dwellOpen  = 350 * time.Millisecond
	dwellClose = 250 * time.Millisecond
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

// swapTransition shows what replaces something in place: the old goes at
// once, the new fades in, sliding x DIPs, so the two are never seen
// together.
// dockShift is how far the place's title drops as it goes from the title
// bar to the top of the list, and rises coming back.
const dockShift = 8

func swapTransition(animate bool, x float32) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return ui.ElementTransition{Duration: motionEnter, Enter: &ui.Motion{Opacity: 0, X: x}}
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

// unfoldTransition opens something called up in a list, pushing the rows
// below aside, and folds it away.
func unfoldTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return ui.ElementTransition{
		Duration: motionList,
		Position: true,
		Enter:    &ui.Motion{Collapse: true, Opacity: 0},
		Exit:     &ui.Motion{Collapse: true, Opacity: 0},
	}
}
