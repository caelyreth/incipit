package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// ExecuteText applies one text/navigation command through the current
// projection. It preserves marker-aware movement and deletion by routing
// projected edits through the same publication path as native input.
func (s *Session) ExecuteText(command TextCommand) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(TextPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	selection := s.presentation.Selection
	snapshot := s.runtime.Snapshot()
	document := snapshot.Document()
	node, ok := document.Node(selection.Node())
	if !ok {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	if command == TextDismissSelection {
		if s.presentation.VisualCollapsed() {
			return kernel.OutcomeHandled, nil
		}
		at := s.presentation.Caret.RangeUTF16().Start
		return kernel.OutcomeHandled, s.SelectProjected(at, at)
	}
	text := ""
	if node.TextPresent() {
		text = node.Text().String()
	}
	projected, err := s.VisualSelection()
	if err != nil {
		return 0, err
	}
	if command == TextLeft || command == TextRight ||
		command == TextSelectLeft || command == TextSelectRight {
		backward := command == TextLeft || command == TextSelectLeft
		extend := command == TextSelectLeft || command == TextSelectRight
		caret := s.presentation.Caret.RangeUTF16().Start
		if !extend && !projected.Empty() {
			edge := projected.End
			if backward {
				edge = projected.Start
			}
			return kernel.OutcomeHandled, s.SelectProjected(edge, edge)
		}
		if markerAdjacent(s.presentation.Projection, caret, backward) {
			next := caret + 1
			if backward {
				next = caret - 1
			}
			viewAnchor := next
			if extend {
				viewAnchor, err = s.currentViewAnchor()
				if err != nil {
					return 0, err
				}
			}
			return kernel.OutcomeHandled, s.SelectProjected(viewAnchor, next)
		}
	}

	if !projected.Empty() && (command == TextBackspace || command == TextDelete) {
		outcome, err := s.ReplaceSelection("", kernel.EventOriginSemantic)
		return outcome.Outcome, err
	}
	if command == TextBackspace || command == TextDelete {
		if outcome, guarded, err := s.guardAtom(command == TextBackspace); guarded || err != nil {
			return outcome, err
		}
	}
	if command == TextBackspace {
		if outcome, peeled, err := s.peelBlock(); peeled || err != nil {
			return outcome, err
		}
	}
	if command == TextBackspace || command == TextDelete {
		backward := command == TextBackspace
		caret := s.presentation.Caret.RangeUTF16().Start
		if marker, ok := markerDeletionRange(s.presentation.Projection, caret, backward); ok {
			edit, err := FromProjectedRange(
				s.presentation.Projection,
				marker,
				"",
				kernel.EventOriginSemantic,
			)
			if err != nil {
				return 0, err
			}
			outcome, err := s.applyProjectedEdit(edit)
			return outcome.Outcome, err
		}
	}
	if command == TextNewline {
		outcome, err := s.ReplaceSelection("\n", kernel.EventOriginTyping)
		return outcome.Outcome, err
	}
	// Select All steps out scope by scope (scopes.go). Selecting the whole
	// page, or to either end of it, reaches the page's ends as it shows
	// them, the notation there included: deleting it then takes a
	// heading's "# " along with its text.
	switch command {
	case TextSelectAll:
		scope, ok := s.nextScope()
		if !ok {
			return kernel.OutcomeHandled, nil
		}
		return kernel.OutcomeHandled, s.SelectProjected(scope.Start, scope.End)
	case TextSelectDocumentStart, TextSelectDocumentEnd:
		viewAnchor, err := s.currentViewAnchor()
		if err != nil {
			return 0, err
		}
		end := model.UTF16Offset(0)
		if command == TextSelectDocumentEnd {
			end = model.UTF16Offset(s.presentation.Projection.UTF16Len())
		}
		return kernel.OutcomeHandled, s.SelectProjected(viewAnchor, end)
	}

	// Positions carry their block: stepping past a block's edge lands in
	// the next one, and the whole-page commands reach the first and last.
	type position struct {
		node   model.NodeID
		offset model.UTF16Offset
	}
	at := func(block *model.Node, offset model.UTF16Offset) position {
		return position{node: block.ID(), offset: offset}
	}
	head := at(node, selection.Head().Offset())
	anchor := position{node: selection.Anchor().Node(), offset: selection.Anchor().Offset()}
	blocks := DocumentBlocks(document)
	first, last := blocks[0], blocks[len(blocks)-1]
	previous := func() position {
		if grapheme, ok := PreviousGrapheme(text, head.offset); ok {
			return at(node, grapheme.Start)
		}
		if before, ok := neighborBlock(document, node.ID(), -1); ok {
			return at(before, blockLength(before))
		}
		return head
	}
	next := func() position {
		if grapheme, ok := NextGrapheme(text, head.offset); ok {
			return at(node, grapheme.End)
		}
		if after, ok := neighborBlock(document, node.ID(), 1); ok {
			return at(after, 0)
		}
		return head
	}
	// edge is the selection's start, or its end, in page order.
	edge := func(end bool) position {
		low, high := orderedAnchors(document,
			model.NewTextAnchor(anchor.node, anchor.offset, model.AffinityRight, 0),
			model.NewTextAnchor(head.node, head.offset, model.AffinityRight, 0))
		if end {
			return position{high.Node(), high.Offset()}
		}
		return position{low.Node(), low.Offset()}
	}
	collapsed := s.presentation.VisualCollapsed()
	var projectedHead *model.UTF16Offset
	lineStart, lineEnd := lineOffsets(text, head.offset)
	switch command {
	case TextLeft:
		target := previous()
		if !collapsed {
			target = edge(false)
		}
		if collapsed && target.node == node.ID() {
			if at, ok := s.visualStepTarget(
				node.ID(), s.presentation.Caret.RangeUTF16().Start, target.offset, -1,
			); ok {
				projectedHead = &at
			}
		}
		anchor, head = target, target
	case TextRight:
		target := next()
		if !collapsed {
			target = edge(true)
		}
		if collapsed && target.node == node.ID() {
			if at, ok := s.visualStepTarget(
				node.ID(), s.presentation.Caret.RangeUTF16().Start, target.offset, 1,
			); ok {
				projectedHead = &at
			}
		}
		anchor, head = target, target
	case TextSelectLeft:
		target := previous()
		if target.node == node.ID() {
			if at, ok := s.visualStepTarget(
				node.ID(), s.presentation.Caret.RangeUTF16().Start, target.offset, -1,
			); ok {
				if anchor, anchorErr := s.currentViewAnchor(); anchorErr == nil {
					return kernel.OutcomeHandled, s.SelectProjected(anchor, at)
				}
			}
		}
		head = target
	case TextSelectRight:
		target := next()
		if target.node == node.ID() {
			if at, ok := s.visualStepTarget(
				node.ID(), s.presentation.Caret.RangeUTF16().Start, target.offset, 1,
			); ok {
				if anchor, anchorErr := s.currentViewAnchor(); anchorErr == nil {
					return kernel.OutcomeHandled, s.SelectProjected(anchor, at)
				}
			}
		}
		head = target
	case TextLineStart:
		head = at(node, lineStart)
		anchor = head
	case TextLineEnd:
		head = at(node, lineEnd)
		anchor = head
	case TextSelectLineStart:
		head = at(node, lineStart)
	case TextSelectLineEnd:
		head = at(node, lineEnd)
	case TextDocumentStart:
		head = at(first, 0)
		anchor = head
	case TextDocumentEnd:
		head = at(last, blockLength(last))
		anchor = head
	case TextSelectDocumentStart:
		head = at(first, 0)
	case TextSelectDocumentEnd:
		head = at(last, blockLength(last))
	case TextSelectAll:
		anchor, head = at(first, 0), at(last, blockLength(last))
	case TextBackspace, TextDelete:
		from, to := previous(), head
		if command == TextDelete {
			from, to = head, next()
		}
		if !collapsed {
			from, to = edge(false), edge(true)
		}
		if from == to {
			return kernel.OutcomeHandled, nil
		}
		// Projected, the range reaches across whatever notation and block
		// boundary lies between, and the edit path decides what that means.
		projectedFrom, projectErr := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(from.node, from.offset, model.AffinityRight, snapshot.Revision()),
		)
		if projectErr != nil {
			return 0, projectErr
		}
		projectedTo, projectErr := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(to.node, to.offset, model.AffinityLeft, snapshot.Revision()),
		)
		if projectErr != nil {
			return 0, projectErr
		}
		edit, editErr := FromProjectedRange(
			s.presentation.Projection,
			model.NewUTF16Range(
				min(projectedFrom.RangeUTF16().Start, projectedTo.RangeUTF16().Start),
				max(projectedFrom.RangeUTF16().Start, projectedTo.RangeUTF16().Start),
			),
			"",
			kernel.EventOriginSemantic,
		)
		if editErr != nil {
			return 0, editErr
		}
		editOutcome, editErr := s.applyProjectedEdit(edit)
		return editOutcome.Outcome, editErr
	}
	if anchor.node == selection.Anchor().Node() && anchor.offset == selection.Anchor().Offset() &&
		head.node == selection.Head().Node() && head.offset == selection.Head().Offset() {
		return kernel.OutcomeHandled, nil
	}
	if collapsed && projectedHead != nil {
		return kernel.OutcomeHandled, s.SelectProjected(*projectedHead, *projectedHead)
	}
	headAffinity := model.AffinityRight
	switch command {
	case TextLeft, TextRight, TextSelectLeft, TextSelectRight:
		if head.node == selection.Head().Node() {
			headAffinity = motionAffinity(selection.Head().OffsetUTF16(), head.offset)
		}
	}
	if err := s.Select(model.NewTextSelection(
		model.NewTextAnchor(anchor.node, anchor.offset, model.AffinityRight, snapshot.Revision()),
		model.NewTextAnchor(head.node, head.offset, headAffinity, snapshot.Revision()),
	)); err != nil {
		return 0, err
	}
	return kernel.OutcomeHandled, nil
}

// motionAffinity attaches a stepped head to the character it crossed, so a
// step onto a mark boundary stops on the near side of the notation there:
// "a*b|c*" steps right to "a*bc|*", inside, and the next step crosses the
// closer by itself.
func motionAffinity(from, to model.UTF16Offset) model.Affinity {
	if to > from {
		return model.AffinityLeft
	}
	return model.AffinityRight
}

// visualStepTarget chooses the nearest drawn stop at a semantic offset on
// the requested side of from. A group of zero-width marks can give one
// semantic offset several stops; semantic projection alone would pick an
// interior one and skip over the caret's neighboring drawn position.
func (s *Session) visualStepTarget(
	node model.NodeID,
	from, to model.UTF16Offset,
	direction int,
) (model.UTF16Offset, bool) {
	projectionValue := s.presentation.Projection
	if projectionValue == nil {
		return 0, false
	}
	base, err := projectionValue.ProjectAnchor(model.NewTextAnchor(
		node,
		to,
		model.AffinityRight,
		s.runtime.Revision(),
	))
	if err != nil {
		return 0, false
	}
	best, found := from, false
	for _, candidate := range headPositions(projectionValue, node, base.RangeUTF16().Start) {
		switch {
		case direction < 0 && candidate < from:
			if !found || candidate > best {
				best, found = candidate, true
			}
		case direction > 0 && from < candidate:
			if !found || candidate < best {
				best, found = candidate, true
			}
		}
	}
	return best, found
}
