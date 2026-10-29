package controller

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/kernel"
)

// VerticalDirection is the visual-row direction of a vertical caret command.
type VerticalDirection int

const (
	VerticalUp   VerticalDirection = -1
	VerticalDown VerticalDirection = 1
)

// MoveVertical moves the projected caret by one retained visual row. Repeated
// vertical movement keeps the x column captured from the first movement; any
// typing, pointer, semantic navigation, history, or Tab operation resets it.
func (c *DocumentController) MoveVertical(direction VerticalDirection, extend bool) error {
	if c == nil {
		return fmt.Errorf("controller: document is required")
	}
	if direction != VerticalUp && direction != VerticalDown {
		return c.fail(fmt.Errorf("controller: invalid vertical direction %d", direction))
	}
	if c.preedit != nil {
		return c.fail(fmt.Errorf("controller: vertical movement during composition"))
	}
	geometry, ok := c.geometry.(VerticalGeometry)
	if !ok || geometry == nil {
		return c.fail(fmt.Errorf("controller: retained vertical geometry unavailable"))
	}

	caret := c.Caret()
	line, ok := geometry.CaretLine(c.text, caret)
	if !ok || line.H <= 0 {
		return c.fail(fmt.Errorf("controller: caret line is unavailable"))
	}
	if !c.hasPreferredX {
		c.preferredX = line.X
		c.hasPreferredX = true
	}
	targetY := line.Y + line.H/2 + float32(direction)*line.H
	head, ok := geometry.IndexForPoint(c.text, Point{X: c.preferredX, Y: targetY})
	if !ok {
		return c.fail(fmt.Errorf("controller: vertical hit test failed"))
	}

	anchor := head
	if extend {
		selection := c.Selection()
		anchor = selection.Range.Start
		if selection.Reversed {
			anchor = selection.Range.End
		}
	}
	if err := c.selectProjected(anchor, head); err != nil {
		return err
	}
	c.setStatus(kernel.OutcomeHandled, false, nil)
	return nil
}
