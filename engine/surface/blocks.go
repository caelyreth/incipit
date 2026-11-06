package surface

import (
	"fmt"
	"sort"

	"github.com/caelyreth/neverland/engine/model"
)

// Block is one block of a flow: the run of its lines from Range.Start to
// Range.End, the break after it not included, how they are spaced and
// where they are set. A flow's blocks cover its text, each the next after
// one line break, so a view never guesses from the text which lines go
// together.
type Block struct {
	Node   model.NodeID
	Range  model.UTF16Range
	Rhythm Rhythm
	Align  BlockAlign
}

// BlockAlign is where a block's lines are set across the column.
type BlockAlign uint8

const (
	AlignStart BlockAlign = iota
	AlignCenter
)

// validateBlocks checks that blocks cover text, in order, one line break
// between each and the next.
func validateBlocks(f *Flow, blocks []Block) error {
	if len(blocks) == 0 {
		return fmt.Errorf("a flow has a block at least")
	}
	next := model.UTF16Offset(0)
	for index, block := range blocks {
		if block.Range.Start != next || block.Range.End < block.Range.Start {
			return fmt.Errorf("blocks must cover the text in order")
		}
		next = block.Range.End + 1
		if index == len(blocks)-1 {
			break
		}
		at, ok := f.ByteOfUTF16(block.Range.End)
		if !ok || int(at) >= len(f.text) || f.text[at] != '\n' {
			return fmt.Errorf("a line break must part each block from the next")
		}
	}
	if blocks[len(blocks)-1].Range.End != f.UTF16Len() {
		return fmt.Errorf("blocks must cover the text")
	}
	return nil
}

// Blocks are the flow's blocks, in order.
func (f *Flow) Blocks() []Block { return append([]Block(nil), f.blocks...) }

// EachBlock visits the blocks in order, without copying them, until fn
// returns false.
func (f *Flow) EachBlock(fn func(index int, block Block) bool) {
	for index, block := range f.blocks {
		if !fn(index, block) {
			return
		}
	}
}

// BlockAt is the block holding offset, the break after a block its own.
func (f *Flow) BlockAt(offset model.UTF16Offset) (Block, int, bool) {
	index := sort.Search(len(f.blocks), func(index int) bool {
		return f.blocks[index].Range.End >= offset
	})
	if index == len(f.blocks) || f.blocks[index].Range.Start > offset {
		return Block{}, 0, false
	}
	return f.blocks[index], index, true
}
