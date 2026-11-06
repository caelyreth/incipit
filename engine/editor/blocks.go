package editor

import (
	"fmt"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// A page is a document of blocks: the root's children, in order, each with
// its own text and marks. Text policies work inside one block; moving,
// selecting and deleting across a block boundary goes through the projection,
// which lays the blocks out one after another.
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

// ValidatePage enforces the editor's flat page shape: a document root whose
// children are childless blocks.
func ValidatePage(document model.DocumentSnapshot) error {
	root, ok := document.Node(document.Root())
	if !ok {
		return &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: document.Root()}
	}
	if !kindOf(root).Root {
		return fmt.Errorf("editor: page root is %s, want %s", root.Kind(), DocumentNode)
	}
	for _, child := range root.Children() {
		node, ok := document.Node(child)
		if !ok {
			return &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: child}
		}
		if len(node.Children()) > 0 {
			return fmt.Errorf("editor: page block %s has nested children", child)
		}
	}
	return nil
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

// blockGroup is a block's place in a group of neighbours. Groups come in
// families, as lists, whose groups nest in one another; a family's groups
// at one depth are its kinds, as bullet and numbered lists. A run of
// consecutive blocks of one kind at a depth, with what nests deeper within
// it of any kind of the family, is one scope. The core knows groups only as
// these names and depths; the page's kinds say which blocks group
// (groupOf, in writing.go).
type blockGroup struct {
	family string
	kind   string
	depth  int
}

// groupRuns are the runs of blocks around index that group with it, the
// deepest first: at each depth from the block's own to 0, the consecutive
// blocks of its group no shallower than that depth. Each run is the first
// and last index it spans.
func groupRuns(blocks []*model.Node, index int) [][2]int {
	group, ok := groupOf(blocks[index])
	if !ok {
		return nil
	}
	runs := make([][2]int, 0, group.depth+1)
	// The kind at each depth is the one the block nests in there: its own
	// at its depth, and the nearest shallower block's before it above.
	kinds := make([]string, group.depth+1)
	kinds[group.depth] = group.kind
	for at, depth := index-1, group.depth; at >= 0 && depth > 0; at-- {
		other, ok := groupOf(blocks[at])
		if !ok || other.family != group.family {
			break
		}
		for depth > 0 && other.depth < depth {
			depth--
			kinds[depth] = other.kind
		}
	}
	within := func(at, depth int) bool {
		other, ok := groupOf(blocks[at])
		return ok && other.family == group.family &&
			(other.depth > depth || other.depth == depth && other.kind == kinds[depth])
	}
	for depth := group.depth; depth >= 0; depth-- {
		if kinds[depth] == "" {
			continue
		}
		first, last := index, index
		for first > 0 && within(first-1, depth) {
			first--
		}
		for last < len(blocks)-1 && within(last+1, depth) {
			last++
		}
		runs = append(runs, [2]int{first, last})
	}
	return runs
}
