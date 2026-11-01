package controller

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// Units are the page's blocks as they move, a table one of them.
func (c *DocumentController) Units() []editor.Unit { return c.session.Units() }

// MoveBlock moves node's block before the unit before starts, nothing for
// the page's end, as its grip drops it.
func (c *DocumentController) MoveBlock(node, before model.NodeID) (kernel.Outcome, error) {
	c.resetPreferredColumn()
	return c.finishCommand(c.session.MoveBlock(node, before))
}
