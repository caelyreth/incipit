package controller

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// SetCheckbox changes a checkbox item in one step of editor history.
func (c *DocumentController) SetCheckbox(node model.NodeID, checked bool) (kernel.Outcome, error) {
	c.resetPreferredColumn()
	return c.finishCommand(c.session.SetCheckbox(node, checked))
}
