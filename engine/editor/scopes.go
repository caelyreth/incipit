package editor

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Select All selects by scope: the block the selection is in, each group
// it nests in from the deepest out, then the whole page, each press the
// next scope that holds more than the selection does. A scope has two
// separate facts: the blocks it owns, which decides whether the selection
// is in it, and the projected range selecting it shows. Generated notation
// can put a caret outside that range while its semantic head remains in an
// owned block, so membership never reads projected containment.

// widening is a run of Select All: the selection it grew from, which
// dismissing what it selected goes back to, and the page's revision then.
// Selecting anything else ends the run (Select), as does an edit.
type widening struct {
	from     model.TextSelection
	revision uint64
}

// widenedFrom is what the run of Select All that made the selection grew
// from, while it stands.
func (s *Session) widenedFrom() (model.TextSelection, bool) {
	w := s.widening
	if w == nil || w.revision != s.runtime.Revision() {
		return model.TextSelection{}, false
	}
	return w.from, true
}

// selectionScope is one scope around the selection. Its block interval owns
// the selection semantically; range16 is the projected range selecting it
// shows.
type selectionScope struct {
	range16       model.UTF16Range
	firstBlock    int
	lastBlock     int
	ownsSelection bool
}

func (s selectionScope) owns(first, last int) bool {
	return s.firstBlock <= first && last <= s.lastBlock
}

// selectionScopes are the scopes around the selection, narrowest first.
func (s *Session) selectionScopes() []selectionScope {
	p := s.presentation.Projection
	page := model.NewUTF16Range(0, model.UTF16Offset(p.UTF16Len()))
	blocks := DocumentBlocks(s.runtime.Snapshot().Document())
	selection := s.presentation.Selection
	index := slices.IndexFunc(blocks, func(block *model.Node) bool {
		return block.ID() == selection.Head().Node()
	})
	if index < 0 {
		return []selectionScope{{range16: page, ownsSelection: true}}
	}
	firstSelected := slices.IndexFunc(blocks, func(block *model.Node) bool {
		return block.ID() == selection.Anchor().Node()
	})
	if firstSelected < 0 {
		return []selectionScope{{range16: page, ownsSelection: true}}
	}
	lastSelected := index
	if lastSelected < firstSelected {
		firstSelected, lastSelected = lastSelected, firstSelected
	}
	scopes := make([]selectionScope, 0, 4)
	if text, ok := blockProjectedRange(p, blocks[index], blockExtentInline); ok {
		scope := selectionScope{
			range16:    text,
			firstBlock: index,
			lastBlock:  index,
		}
		scope.ownsSelection = scope.owns(firstSelected, lastSelected)
		scopes = append(scopes, scope)
	}
	for _, run := range groupRuns(blocks, index) {
		first, firstOK := blockProjectedRange(p, blocks[run[0]], blockExtentAll)
		last, lastOK := blockProjectedRange(p, blocks[run[1]], blockExtentAll)
		if firstOK && lastOK {
			scope := selectionScope{
				range16:    model.NewUTF16Range(first.Start, last.End),
				firstBlock: run[0],
				lastBlock:  run[1],
			}
			scope.ownsSelection = scope.owns(firstSelected, lastSelected)
			scopes = append(scopes, scope)
		}
	}
	document := s.runtime.Snapshot().Document()
	if table, ok := TableOf(document, blocks[index].ID()); ok {
		first, firstOK := blockProjectedRange(p, mustNode(document, table.First()), blockExtentAll)
		last, lastOK := blockProjectedRange(p, mustNode(document, table.Last()), blockExtentAll)
		if firstOK && lastOK {
			scope := selectionScope{
				range16:    model.NewUTF16Range(first.Start, last.End),
				firstBlock: slices.IndexFunc(blocks, func(b *model.Node) bool { return b.ID() == table.First() }),
				lastBlock:  slices.IndexFunc(blocks, func(b *model.Node) bool { return b.ID() == table.Last() }),
			}
			scope.ownsSelection = scope.owns(firstSelected, lastSelected)
			scopes = append(scopes, scope)
		}
	}
	scopes = append(scopes, selectionScope{
		range16: page, firstBlock: 0, lastBlock: len(blocks) - 1, ownsSelection: true,
	})
	return scopes
}

// blockExtent is how much of one block a selection scope includes.
type blockExtent uint8

const (
	// blockExtentInline keeps inline notation with its text, but leaves the
	// block's structural notation to the enclosing group or page.
	blockExtentInline blockExtent = iota
	// blockExtentAll includes structural notation and objects as well.
	blockExtentAll
)

// blockProjectedRange is a block's projected extent at one scope level.
// Inline notation belongs to the block's local scope; structural notation
// belongs only once selection widens to its group or page.
func blockProjectedRange(
	p *projection.Projection,
	block *model.Node,
	extent blockExtent,
) (model.UTF16Range, bool) {
	return projectedSegments(p, block, extent)
}

func projectedSegments(
	p *projection.Projection,
	block *model.Node,
	extent blockExtent,
) (model.UTF16Range, bool) {
	start, end, found := model.UTF16Offset(0), model.UTF16Offset(0), false
	for _, segment := range p.Segments() {
		owner, ok := segmentNode(segment)
		if !ok || owner != block.ID() {
			continue
		}
		if extent == blockExtentInline {
			if target, ok := segment.AttributeTarget(); ok {
				if !MarkerKeyIsInline(target.Key()) {
					continue
				}
			} else if _, ok := segment.SemanticTarget(); !ok {
				continue
			}
		}
		span := segment.ProjectedRange()
		if !found {
			start, found = span.Start, true
		}
		end = max(end, span.End)
	}
	return model.NewUTF16Range(start, end), found
}

// nextScope is the narrowest scope holding more than the selection.
func (s *Session) nextScope() (model.UTF16Range, bool) {
	selected, err := s.VisualSelection()
	if err != nil {
		return model.UTF16Range{}, false
	}
	scopes := s.selectionScopes()
	// An exact projected match is the selection already made by this
	// sequence, even when its semantic anchors have normalized into one
	// block. Step past it rather than treating that anchor as a new start.
	current := false
	for _, scope := range scopes {
		if scope.range16 == selected {
			current = true
			continue
		}
		if current {
			return scope.range16, true
		}
	}
	if current {
		return model.UTF16Range{}, false
	}
	// A selection not matching a scope, as a caret in generated notation,
	// starts at the narrowest scope that owns its semantic endpoints, never
	// one inside it: a selected picture widens to the page, not narrowing
	// to its caption.
	for _, scope := range scopes {
		inside := !selected.Empty() && selected.Start <= scope.range16.Start && scope.range16.End <= selected.End
		if scope.ownsSelection && scope.range16 != selected && !inside {
			return scope.range16, true
		}
	}
	return model.UTF16Range{}, false
}

// scopeGuides mark the next scope while a scope is selected: pressing
// Select All again would select the area they mark.
func (s *Session) scopeGuides(time.Time) []Guide {
	selected, err := s.VisualSelection()
	if err != nil || selected.Empty() {
		return nil
	}
	for _, scope := range s.selectionScopes() {
		if scope.range16 != selected {
			continue
		}
		next, ok := s.nextScope()
		if !ok {
			return nil
		}
		return []Guide{{
			Producer: surface.ProducerID(TextPolicyID),
			Command:  TextSelectAll.ID(),
			Area:     []model.UTF16Range{next},
		}}
	}
	return nil
}
