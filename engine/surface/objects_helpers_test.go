package surface

import (
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// asObjects are items as the flow's objects.
func asObjects[T Object](items []T) []Object {
	objects := make([]Object, len(items))
	for index, item := range items {
		objects[index] = item
	}
	return objects
}

// lineBlocks are text's lines, each a block of plain prose.
func lineBlocks(text string) []Block {
	var blocks []Block
	start := model.UTF16Offset(0)
	for _, line := range strings.Split(text, "\n") {
		end := start + model.UTF16Offset(model.StringLenUTF16(line))
		blocks = append(blocks, Block{Range: model.NewUTF16Range(start, end)})
		start = end + 1
	}
	return blocks
}
func ownedBy(flow *Flow, objects []Object) (*Flow, error) {
	// A block takes in the next when an object runs across the break
	// between them.
	var blocks []Block
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
	named, err := NewFlow(flow.Version(), flow.Text(), flow.Runs(), blocks)
	if err != nil {
		return nil, err
	}
	return named.WithObjects(objects)
}

func nodeOf(object Object) model.NodeID {
	switch object := object.(type) {
	case Checkbox:
		return object.Node
	case Formula:
		return object.Node
	case Figure:
		return object.Node
	}
	return 0
}

// runsAcross reports whether one of objects runs across offset.
func runsAcross(objects []Object, offset model.UTF16Offset) bool {
	for _, object := range objects {
		if run := object.Run(); run.Start <= offset && offset < run.End {
			return true
		}
	}
	return false
}
