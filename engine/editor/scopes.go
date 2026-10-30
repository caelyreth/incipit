package editor

import (
	"time"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Select All selects by scope: the block the selection is in, then the
// whole page, each press the next scope that holds more than the
// selection does. Scopes are projected ranges: a block's is its text,
// between its notation; the page's is all it shows, notation included.

// selectionScopes are the scopes around the selection, narrowest first.
func (s *Session) selectionScopes() []model.UTF16Range {
	p := s.presentation.Projection
	scopes := make([]model.UTF16Range, 0, 2)
	if !s.presentation.Selection.Spans() {
		if block, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Head().Node()); ok {
			revision := s.runtime.Revision()
			start, startErr := p.ProjectAnchor(model.NewTextAnchor(block.ID(), 0, model.AffinityRight, revision))
			end, endErr := p.ProjectAnchor(model.NewTextAnchor(block.ID(), blockLength(block), model.AffinityLeft, revision))
			if startErr == nil && endErr == nil {
				scopes = append(scopes, model.NewUTF16Range(start.RangeUTF16().Start, end.RangeUTF16().Start))
			}
		}
	}
	return append(scopes, model.NewUTF16Range(0, model.UTF16Offset(p.UTF16Len())))
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
