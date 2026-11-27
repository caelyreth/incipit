package shell

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/caelyreth/incipit/internal/ui/components"
)

// The library answers at the speed of a click: the field is something
// the hand drives over and over, so its motion only says where things went
// and gets out of the way. Settings and the page keep their own pace.
//
// Motion moves, scales, fades and tints; text keeps its layout while its
// transform carries it between places.
// motionArm turns a button to ask for its second click.
const motionArm = components.MotionQuick

// listTransition is how rows come, go and make room: gliding, as a row
// folding open past its height and back would only wobble.
func listTransition(animate, size bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	glide := components.Glide(components.MotionNormal)
	return ui.ElementTransition{
		Animation: &glide,
		Position:  true, Size: size,
		Enter: collapsing, Exit: collapsing,
	}
}

// collapsing is a row folding open as it comes and shut as it goes,
// shared by every row as MyGo only reads it.
var collapsing = &ui.Motion{Collapse: true}

// appearing is a transition that only brings an element in and takes it
// out. MyGo animates an element's place, size and colors when none of
// them is chosen, so one choosing nothing would also trail behind every
// move of its parent, as a pane opening: it chooses colors, which stay.
func appearing(d time.Duration, enter, exit *ui.Motion) ui.ElementTransition {
	return ui.ElementTransition{Duration: d, Colors: true, Enter: enter, Exit: exit}
}

// popTransition lets something small appear where it stands, a little
// past its size and back, and leave quickly.
func popTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	spring := components.Settle(components.MotionPanel)
	gone := &ui.Motion{Scale: 0.9, Opacity: 0}
	return ui.ElementTransition{Animation: &spring, ExitDuration: components.MotionQuick, Colors: true, Enter: gone, Exit: gone}
}

func unfoldAnimation() ui.Animation {
	return components.Glide(components.MotionNormal)
}

// unfoldTransition opens something called up in a list, pushing the rows
// below aside, and folds it away.
func unfoldTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	animation := unfoldAnimation()
	return ui.ElementTransition{
		Animation: &animation,
		Position:  true,
		Enter:     &ui.Motion{Collapse: true, Opacity: 0},
		Exit:      &ui.Motion{Collapse: true, Opacity: 0},
	}
}
