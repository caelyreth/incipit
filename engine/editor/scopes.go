package editor

import (
	"slices"
	"time"

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
	revision := s.runtime.Revision()
	project := func(block *model.Node, offset model.UTF16Offset, affinity model.Affinity) (model.UTF16Offset, bool) {
		target, err := p.ProjectAnchor(model.NewTextAnchor(block.ID(), offset, affinity, revision))
		return target.RangeUTF16().Start, err == nil
	}
	start := func(at int) (model.UTF16Offset, bool) {
		return project(blocks[at], 0, model.AffinityRight)
	}
	// A block's text ends no earlier than it starts: leaning left, an empty
	// block's end would fall before its own notation.
	end := func(at int) (model.UTF16Offset, bool) {
		first, firstOK := start(at)
		last, lastOK := project(blocks[at], blockLength(blocks[at]), model.AffinityLeft)
		return max(first, last), firstOK && lastOK
	}
	// A line starts past the break that ends the line before it.
	lineStart := func(at int) (model.UTF16Offset, bool) {
		if at == 0 {
			return 0, true
		}
		previous, ok := end(at - 1)
		return previous + 1, ok
	}
	scopes := make([]selectionScope, 0, 4)
	first, firstOK := start(index)
	last, lastOK := end(index)
	if firstOK && lastOK {
		scope := selectionScope{
			range16:    model.NewUTF16Range(first, last),
			firstBlock: index,
			lastBlock:  index,
		}
		scope.ownsSelection = scope.owns(firstSelected, lastSelected)
		scopes = append(scopes, scope)
	}
	for _, run := range groupRuns(blocks, index) {
		first, firstOK := lineStart(run[0])
		last, lastOK := end(run[1])
		if firstOK && lastOK {
			scope := selectionScope{
				range16:    model.NewUTF16Range(first, last),
				firstBlock: run[0],
				lastBlock:  run[1],
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
	// starts at the narrowest scope that owns its semantic endpoints.
	for _, scope := range scopes {
		if scope.ownsSelection && scope.range16 != selected {
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
