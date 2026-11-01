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

// captionScale counts how open a caption is in hundredths, so its spring
// settles as finely as a position's.
const captionScale = 100

// captionedFlow is flow with each figure's caption as open as it shows:
// one with a caption all the way, one without as far as its line has
// opened, opening while it or its picture is selected. It reports whether one is
// still opening or closing.
func (e *Element) captionedFlow(flow *surface.Flow, selection model.UTF16Range, now time.Time, animate bool) (*surface.Flow, bool) {
	figures := flow.Figures()
	if len(figures) == 0 {
		e.captions = nil
		return flow, false
	}
	captions := make(map[model.NodeID]motion.Spring, len(figures))
	moving := false
	for index := range figures {
		figure := &figures[index]
		open := figure.Caption * captionScale
		if captionCaret(*figure, selection) || selection == figure.Object() {
			open = captionScale
		}
		spring, known := e.captions[figure.Node]
		if !known || !animate {
			spring = motion.NewSpring(open, now)
		}
		spring.Step(open, now, figureFade)
		captions[figure.Node] = spring
		figure.Caption = spring.Value() / captionScale
		moving = moving || spring.Moving()
	}
	e.captions = captions
	captioned, err := flow.WithFigures(figures)
	if err != nil {
		return flow, false
	}
	return captioned, moving
}

// captionCaret reports whether the caret is at the start of a figure's
// caption, as it is in one with none.
func captionCaret(figure surface.Figure, selection model.UTF16Range) bool {
	return selection.Empty() && selection.Start == figure.CaptionStart()
}
