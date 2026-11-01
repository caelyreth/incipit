package controller

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// InsertImage puts a picture on the page after the caret's block, or in
// place of the empty paragraph it is in, and goes on below it.
func (c *DocumentController) InsertImage(shape editor.ImageShape) (kernel.Outcome, error) {
	c.resetPreferredColumn()
	outcome, err := c.session.InsertImage(shape)
	return c.finishCommand(outcome, err)
}

// ShapeImage changes what an image block shows in one step of history.
func (c *DocumentController) ShapeImage(node model.NodeID, change func(*editor.ImageShape)) (kernel.Outcome, error) {
	outcome, err := c.session.ShapeImage(node, change)
	return c.finishCommand(outcome, err)
}
