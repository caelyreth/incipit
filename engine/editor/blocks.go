package editor

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// A page is a document of blocks: the root's children, in order, each with
// its own text and marks. Text policies work inside one block; moving,
// selecting and deleting across a block boundary goes through the projection,
// which lays the blocks out one after another.
const (
	// DocumentNode is the kind of a page's root.
	DocumentNode model.NodeTypeID = "document"
	// ParagraphNode is the plain block every page starts with.
	ParagraphNode model.NodeTypeID = "paragraph"
)

// NewBlockDocument builds a page holding text, one paragraph per line.
func NewBlockDocument(text string) (*model.Document, []model.NodeID, error) {
	document, err := model.NewDocument(DocumentNode)
	if err != nil {
		return nil, nil, err
	}
	root := document.Snapshot().Root()
	blocks := make([]model.NodeID, 0, strings.Count(text, "\n")+1)
	_, err = document.Transact(func(draft *model.DocumentDraft) error {
		for index, line := range strings.Split(text, "\n") {
			id, err := draft.CreateNode(root, index, ParagraphNode)
			if err != nil {
				return err
			}
			if err := draft.SetTextValue(id, line); err != nil {
				return err
			}
			blocks = append(blocks, id)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return document, blocks, nil
}

// DocumentBlocks returns a page's blocks in order.
func DocumentBlocks(document model.DocumentSnapshot) []*model.Node {
	root, ok := document.Node(document.Root())
	if !ok {
		return nil
	}
	blocks := make([]*model.Node, 0, len(root.Children()))
	for _, id := range root.Children() {
		if node, ok := document.Node(id); ok {
			blocks = append(blocks, node)
		}
	}
	return blocks
}

// blockOrder lists a page's block identities in order.
func blockOrder(document model.DocumentSnapshot) []model.NodeID {
	root, ok := document.Node(document.Root())
	if !ok {
		return nil
	}
	return root.Children()
}

// neighborBlock returns the block before (step -1) or after (step 1) node.
func neighborBlock(document model.DocumentSnapshot, node model.NodeID, step int) (*model.Node, bool) {
	order := blockOrder(document)
	index := slices.Index(order, node)
	if index < 0 || index+step < 0 || index+step >= len(order) {
		return nil, false
	}
	return document.Node(order[index+step])
}

// blockLength is a block's text length in UTF-16 units.
func blockLength(node *model.Node) model.UTF16Offset {
	if node == nil || !node.TextPresent() {
		return 0
	}
	return model.UTF16Offset(node.Text().UTF16Len())
}

// blocksBetween lists the blocks from first to last, in page order, both
// included, whichever way round they are given.
func blocksBetween(document model.DocumentSnapshot, first, last model.NodeID) []model.NodeID {
	order := blockOrder(document)
	from, to := slices.Index(order, first), slices.Index(order, last)
	if from < 0 || to < 0 {
		return nil
	}
	if from > to {
		from, to = to, from
	}
	return slices.Clone(order[from : to+1])
}

// orderedAnchors puts two anchors in page order.
func orderedAnchors(document model.DocumentSnapshot, a, b model.TextAnchor) (model.TextAnchor, model.TextAnchor) {
	if a.Node() == b.Node() {
		if b.Offset() < a.Offset() {
			return b, a
		}
		return a, b
	}
	order := blockOrder(document)
	if slices.Index(order, b.Node()) < slices.Index(order, a.Node()) {
		return b, a
	}
	return a, b
}
