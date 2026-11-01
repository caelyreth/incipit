package documentview

import (
	"time"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

// A picture with no caption takes no room for one. Its line opens, the
// page below gliding down, as the caret comes into it: by the chip that
// offers a caption once the pointer rests on the picture, by its menu, or
// by the keys; and closes again as the caret leaves it still empty. A
// centered caption grows from its middle, gliding as it is typed, and its
// hint is centered too, the caret at the hint's start, as an empty
// field's.

const (
	// captionDwell is how long the pointer rests on a picture with no
	// caption before it offers one.
	captionDwell = 600 * time.Millisecond
	// captionScale counts how open a caption is in hundredths, so its
	// spring settles as finely as a position's.
	captionScale = 100
	captionOffer = "Add caption"
)

// captionedFlow is flow with each figure's caption as open as it shows:
// one with a caption all the way, one without as far as its line has
// opened, opening while the caret is in it. It reports whether one is
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
		if captionCaret(figure.At, selection) {
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

// captionCaret reports whether the caret is in the caption of the figure
// whose line starts at at, past the character its picture stands on.
func captionCaret(at model.UTF16Offset, selection model.UTF16Range) bool {
	return selection.Empty() && selection.Start == at+1
}

// captionless reports whether a figure has no caption and its line is
// closed, the caret elsewhere.
func captionless(figure textlayout.PlacedFigure, selection model.UTF16Range) bool {
	return figure.Line.Len() == 1 && !captionCaret(figure.At, selection)
}

// offerCaption is the figure the pointer has rested on long enough, with
// no caption, to offer one; otherwise when to look again.
func (e *Element) offerCaption(now time.Time) (model.NodeID, time.Duration) {
	if !e.hover.ok || e.resizing != nil {
		e.dwell = figureDwell{}
		return 0, 0
	}
	if e.dwell.node != e.hover.node {
		e.dwell = figureDwell{node: e.hover.node, since: now}
	}
	if wait := captionDwell - now.Sub(e.dwell.since); wait > 0 {
		return 0, wait
	}
	return e.dwell.node, 0
}

// figureDwell is how long the pointer has rested on a figure.
type figureDwell struct {
	node  model.NodeID
	since time.Time
}

// pressCaptionOffer opens the caption of the figure whose offer a press is
// on, as a click in its line would.
func (e *Element) pressCaptionOffer(figure textlayout.PlacedFigure, point ui.Point, event ui.InputEvent) bool {
	offer := e.captionOffer
	if offer.node != figure.Node || !offer.box.Contains(point.X, point.Y) || event.Button != 0 {
		return false
	}
	_ = e.Input.Select(int(figure.Line.End), int(figure.Line.End), event.Time)
	return true
}

// shownOffer is the chip offering a caption as last drawn, content-local.
type shownOffer struct {
	node model.NodeID
	box  ui.Rect
}
