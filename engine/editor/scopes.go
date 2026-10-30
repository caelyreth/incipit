package editor

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Select All selects by scope: the block the selection is in, each group
// it nests in from the deepest out, then the whole page, each press the
// next scope that holds more than the selection does. Scopes are projected
// ranges: a block's is its text, between its notation; a group's runs
// from its first line's start, notation included, to its last line's end;
// the page's is all it shows.

// selectionScopes are the scopes around the selection, narrowest first.
func (s *Session) selectionScopes() []model.UTF16Range {
	p := s.presentation.Projection
	page := model.NewUTF16Range(0, model.UTF16Offset(p.UTF16Len()))
	blocks := DocumentBlocks(s.runtime.Snapshot().Document())
	index := slices.IndexFunc(blocks, func(block *model.Node) bool {
		return block.ID() == s.presentation.Selection.Head().Node()
	})
	if index < 0 {
		return []model.UTF16Range{page}
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
	scopes := make([]model.UTF16Range, 0, 4)
	first, firstOK := start(index)
	last, lastOK := end(index)
	if firstOK && lastOK {
		scopes = append(scopes, model.NewUTF16Range(first, last))
	}
	for _, run := range groupRuns(blocks, index) {
		first, firstOK := lineStart(run[0])
		last, lastOK := end(run[1])
		if firstOK && lastOK {
			scopes = append(scopes, model.NewUTF16Range(first, last))
		}
	}
	return append(scopes, page)
}

// nextScope is the narrowest scope holding more than the selection.
func (s *Session) nextScope() (model.UTF16Range, bool) {
	selected, err := s.VisualSelection()
	if err != nil {
		return model.UTF16Range{}, false
	}
	for _, scope := range s.selectionScopes() {
		if scope.Start <= selected.Start && selected.End <= scope.End && scope != selected {
			return scope, true
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
		if scope != selected {
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
