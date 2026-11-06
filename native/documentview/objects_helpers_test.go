package documentview

import (
	"strings"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// asObjects are items as the flow's objects.
func asObjects[T surface.Object](items []T) []surface.Object {
	objects := make([]surface.Object, len(items))
	for index, item := range items {
		objects[index] = item
	}
	return objects
}

// lineBlocks are text's lines, each a block of plain prose.
func lineBlocks(text string) []surface.Block {
	var blocks []surface.Block
	start := model.UTF16Offset(0)
	for _, line := range strings.Split(text, "\n") {
		end := start + model.UTF16Offset(model.StringLenUTF16(line))
		blocks = append(blocks, surface.Block{Range: model.NewUTF16Range(start, end)})
		start = end + 1
	}
	return blocks
}

// ownedBy is flow with objects standing for its text, each block named
// after the node of the object it holds and holding the whole of it, as
// the profile names and draws blocks.
func ownedBy(flow *surface.Flow, objects []surface.Object) (*surface.Flow, error) {
	// A block takes in the next when an object runs across the break
	// between them.
	var blocks []surface.Block
	for _, block := range flow.Blocks() {
		if last := len(blocks) - 1; last >= 0 && runsAcross(objects, blocks[last].Range.End) {
			blocks[last].Range.End = block.Range.End
			continue
		}
		blocks = append(blocks, block)
	}
	for _, object := range objects {
		for index := range blocks {
			if run := object.Run(); blocks[index].Range.Start <= run.Start && run.Start < blocks[index].Range.End+1 {
				blocks[index].Node = nodeOf(object)
			}
		}
	}
	named, err := surface.NewFlow(flow.Version(), flow.Text(), flow.Runs(), blocks)
	if err != nil {
		return nil, err
	}
	return named.WithObjects(objects)
}

func nodeOf(object surface.Object) model.NodeID {
	switch object := object.(type) {
	case surface.Checkbox:
		return object.Node
	case surface.Formula:
		return object.Node
	case surface.Figure:
		return object.Node
	}
	return 0
}

// runsAcross reports whether one of objects runs across offset.
func runsAcross(objects []surface.Object, offset model.UTF16Offset) bool {
	for _, object := range objects {
		if run := object.Run(); run.Start <= offset && offset < run.End {
			return true
		}
	}
	return false
}
