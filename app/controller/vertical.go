package controller

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
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
		return c.fail(fmt.Errorf("controller: vertical movement: %w", editor.ErrCompositionActive))
	}
	geometry, ok := c.geometry.(VerticalGeometry)
	if !ok || geometry == nil {
		return c.fail(fmt.Errorf("%w: no retained vertical geometry", ErrNoGeometry))
	}

	caret := c.Caret()
	line, ok := geometry.CaretLine(c.text, caret)
	if !ok || line.H <= 0 {
		return c.fail(fmt.Errorf("%w: no caret line", ErrNoGeometry))
	}
	if !c.hasPreferredX {
		c.preferredX = line.X
		c.hasPreferredX = true
	}
	head, ok := geometry.RowIndex(c.text, caret, direction, c.preferredX)
	if !ok {
		return c.fail(fmt.Errorf("%w: vertical hit test failed", ErrNoGeometry))
	}
	// The row's hit lands as the engine's own motion does: past an atom
	// whole, never inside a formula seen from outside.
	head = int(c.session.SnapMotion(model.UTF16Offset(caret), model.UTF16Offset(head)))

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
