package documentview

import (
	"time"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
)

// A picture with no caption takes no room for one. Its line opens, the
// page below gliding down, while the picture is selected, so Escape or a
// click there writes one, and while the caret is in it, brought by the
// menu's Add Caption or the keys; it closes again once neither is so and
// it is still empty. A centered caption grows from its middle, gliding as
// it is typed, and its hint is centered too, the caret at the hint's
// start, as an empty field's.

// captionsOf is how open each figure's caption line shows: one with a
// caption all the way, one without as far as its line has opened, opening
// while it or its picture is selected. It reports whether one is still
// opening or closing.
func (e *Element) captionsOf(flow *surface.Flow, selection model.UTF16Range, now time.Time, animate bool) (map[model.NodeID]float32, bool) {
	figures := surface.ObjectsOf[surface.Figure](flow)
	if len(figures) == 0 {
		e.captions = nil
		return nil, false
	}
	springs := make(map[model.NodeID]motion.Spring, len(figures))
	shown := make(map[model.NodeID]float32, len(figures))
	moving := false
	for _, figure := range figures {
		open := float32(0)
		if figure.Captioned || captionCaret(figure, selection) || selection == figure.Range {
			open = 1
		}
		spring, known := e.captions[figure.Node]
		if !known || !animate {
			spring = motion.NewUnitSpring(open, now)
		}
		spring.Step(open, now, figureFade)
		springs[figure.Node] = spring
		shown[figure.Node] = spring.Value()
		moving = moving || spring.Moving()
	}
	e.captions = springs
	return shown, moving
}

// captionCaret reports whether the caret is at the start of a figure's
// caption, as it is in one with none.
func captionCaret(figure surface.Figure, selection model.UTF16Range) bool {
	return selection.Empty() && selection.Start == figure.CaptionStart()
}
