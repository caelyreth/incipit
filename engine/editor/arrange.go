package editor

import (
	"slices"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// A block moves whole, dragged by its grip to stand between two others: a
// paragraph, a heading, a figure, or a table with all its cells. It keeps
// its identity, so the caret in it goes along, and the move is one step of
// history.

const arrangeTrigger = "writing:arrange"

// Unit is a block the page moves whole, as its writer sees it: its first
// node, the node a table's first cell is, and where it is in the
// projection.
type Unit struct {
	Node  model.NodeID
	Range model.UTF16Range
	Table bool
}

// Units are the page's blocks in order, a table one of them.
func (s *Session) Units() []Unit {
	document := s.runtime.Snapshot().Document()
	ranges := map[model.NodeID]model.UTF16Range{}
	s.presentation.Projection.EachSegment(func(_ int, segment projection.Segment) bool {
		owner, ok := segmentNode(segment)
		if !ok {
			return true
		}
		span := segment.ProjectedRange()
		if known, ok := ranges[owner]; ok {
			span = model.NewUTF16Range(min(known.Start, span.Start), max(known.End, span.End))
		}
		ranges[owner] = span
		return true
	})
	tables := map[model.NodeID]Table{}
	for _, table := range readTables(document) {
		for _, cell := range table.Cells() {
			tables[cell] = table
		}
	}
	root, _ := document.Node(document.Root())
	var units []Unit
	for _, id := range root.Children() {
		span, ok := ranges[id]
		if !ok {
			continue
		}
		if table, inTable := tables[id]; inTable {
			if id != table.First() {
				last := &units[len(units)-1]
				last.Range = model.NewUTF16Range(last.Range.Start, max(last.Range.End, span.End))
				continue
			}
			units = append(units, Unit{Node: id, Range: span, Table: true})
			continue
		}
		units = append(units, Unit{Node: id, Range: span})
	}
	return units
}

// unitBlocks are the blocks node moves with: its table's cells, or itself.
func unitBlocks(document blockReader, node model.NodeID) []model.NodeID {
	if table, ok := tableAround(document, node); ok {
		return table.Cells()
	}
	return []model.NodeID{node}
}

// MoveBlock moves node's block, a table whole, to stand before the unit
// before stands first in, nothing for the page's end.
func (s *Session) MoveBlock(node, before model.NodeID) (kernel.Outcome, error) {
	return s.dispatchWriting(arrangeTrigger, arrangeRule, func(document *model.DocumentDraft) (bool, error) {
		blocks := draftBlocks(document)
		unit := unitBlocks(document, node)
		first := blockIndex(document, unit[0])
		last := first + len(unit) - 1
		to := len(blocks)
		if before != 0 {
			to = blockIndex(document, before)
			if starts := unitBlocks(document, before); to < 0 || starts[0] != before {
				// Only before a whole unit, never between a table's cells.
				return false, nil
			}
		}
		if first < 0 || to >= first && to <= last+1 {
			return false, nil
		}
		at, order := to, slices.Clone(unit)
		if to > last {
			// Moving down, the last goes first, each counted without
			// those still above it.
			at -= len(unit)
			slices.Reverse(order)
		}
		for index, id := range order {
			offset := index
			if to > last {
				offset = len(unit) - 1 - index
			}
			if err := document.MoveNode(id, at+offset); err != nil {
				return false, err
			}
		}
		return true, nil
	})
}
