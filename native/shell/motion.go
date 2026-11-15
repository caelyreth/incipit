package shell

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/caelyreth/neverland/native/components"
)

// The library answers at the speed of a click: the sidebar is something
// the hand drives over and over, so its motion only says where things went
// and gets out of the way. Settings, dialogs and the page keep their own
// pace.
//
// Motion moves, scales, fades and tints; it never animates a font size or
// weight. Each size is a font of its own, and MyGo lets all its fonts go
// when they pile up, under text layouts that may still be drawn with them.
const (
	// motionReveal opens and closes the sidebar.
	motionReveal = components.MotionPanel
	// motionPlace moves the place chips from one place to the next.
	motionPlace = 220 * time.Millisecond
	// motionFold opens and closes a group in the tree.
	motionFold = components.MotionPanel
	// motionList moves rows as others come and go, and opens and
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
	// dropQuiet is how long create areas stay shut after a drag ends,
	// before the pointer's rest begins to count: a row dropped at the head
	// of a list leaves the pointer by its top, which is not a call for a
	// new one.
	dropQuiet = time.Second
)

// listTransition is how rows come, go and make room.
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

// appearing is a transition that only brings an element in and takes it
// out. MyGo animates an element's place, size and colors when none of
// them is chosen, so one choosing nothing would also trail behind every
// move of its parent, as a pane opening: it chooses colors, which stay.
func appearing(d time.Duration, enter, exit *ui.Motion) ui.ElementTransition {
	return ui.ElementTransition{Duration: d, Colors: true, Enter: enter, Exit: exit}
}

// fadeTransition swaps what stands in one place: the new rises in as the
// old fades.
func fadeTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return appearing(motionPlace, &ui.Motion{Y: 6, Opacity: 0}, &ui.Motion{Opacity: 0})
}

// swapTransition shows what replaces something in place: the old goes at
// once, the new fades in, sliding x DIPs, so the two are never seen
// together.
func swapTransition(animate bool, x float32) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return appearing(motionEnter, &ui.Motion{Opacity: 0, X: x}, nil)
}

// popTransition lets something small appear and leave where it stands.
func popTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return appearing(motionList, &ui.Motion{Scale: 0.9, Opacity: 0}, &ui.Motion{Scale: 0.9, Opacity: 0})
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
