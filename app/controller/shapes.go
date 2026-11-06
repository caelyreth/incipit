package controller

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// Shape makes node the block shape describes, keeping its text, in one step
// of editor history: what a block's modifier control, or a checkbox's
// click, asks.
func (c *DocumentController) Shape(node model.NodeID, shape editor.BlockShape) (kernel.Outcome, error) {
	return c.Do(editor.Shape(node, shape))
}

// ShapeOptions are the shapes node's modifier control offers.
func (c *DocumentController) ShapeOptions(node model.NodeID) []editor.ShapeOption {
	return c.session.ShapeOptions(node)
}

// ShapeOf returns what node is apart from its text.
func (c *DocumentController) ShapeOf(node model.NodeID) (editor.BlockShape, bool) {
	block, ok := c.session.Runtime().Snapshot().Document().Node(node)
	if !ok {
		return editor.BlockShape{}, false
	}
	return editor.ShapeOf(block), true
}
