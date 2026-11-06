package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// doText applies one text/navigation command through the current
// projection. It preserves marker-aware movement and deletion by routing
// projected edits through the same publication path as native input.
func (s *Session) doText(command textOp) (kernel.Outcome, error) {
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
	if command == textDismissSelection {
		if s.presentation.VisualCollapsed() {
			return s.dismissPlace()
		}
		// What Select All widened to goes back to what it grew from;
		// any other selection leaves its caret at its head.
		if from, ok := s.widenedFrom(); ok {
			return kernel.OutcomeHandled, s.Select(from)
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
	if command == textLeft || command == textRight ||
		command == textSelectLeft || command == textSelectRight {
		backward := command == textLeft || command == textSelectLeft
		extend := command == textSelectLeft || command == textSelectRight
		caret := s.presentation.Caret.RangeUTF16().Start
		if !extend && !projected.Empty() {
			edge := projected.End
			if backward {
				edge = projected.Start
			}
			return kernel.OutcomeHandled, s.SelectProjected(edge, edge)
		}
		// A formula is passed as one character from outside it; its
		// source is edited where a click, Tab or typing puts the caret.
		flow, err := s.SurfaceFlow()
		if err != nil {
			return 0, err
		}
		if to, ok := pastFormula(flow, caret, backward); ok {
			viewAnchor := to
			if extend {
				viewAnchor, err = s.currentViewAnchor()
				if err != nil {
					return 0, err
				}
			}
			return kernel.OutcomeHandled, s.SelectProjected(viewAnchor, to)
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

	if !projected.Empty() && (command == textBackspace || command == textDelete) {
		outcome, err := s.replaceSelection("", kernel.EventOriginSemantic)
		return outcome.Outcome, err
	}
	if command == textBackspace || command == textDelete {
		if outcome, guarded, err := s.guardAtom(command == textBackspace); guarded || err != nil {
			return outcome, err
		}
		if outcome, guarded, err := s.guardTable(command == textBackspace); guarded || err != nil {
			return outcome, err
		}
	}
	if command == textBackspace {
		if outcome, peeled, err := s.peelBlock(); peeled || err != nil {
			return outcome, err
		}
	}
	if command == textBackspace || command == textDelete {
		backward := command == textBackspace
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
	if command == textNewline {
		// Return in a cell goes down from it, whatever of it is selected.
		if head := s.presentation.Selection.Head(); s.inCell(head.Node()) && head.Node() == s.presentation.Selection.Anchor().Node() {
			if err := s.Select(model.NewTextSelection(head, head)); err != nil {
				return 0, err
			}
		}
		if outcome, ok, err := s.growDown(); ok {
			return outcome, err
		}
		outcome, err := s.replaceSelection("\n", kernel.EventOriginTyping)
		return outcome.Outcome, err
	}
	// Select All steps out scope by scope (scopes.go). Selecting the whole
	// page, or to either end of it, reaches the page's ends as it shows
	// them, the notation there included: deleting it then takes a
	// heading's "# " along with its text.
	switch command {
	case textSelectAll:
		scope, ok := s.nextScope()
		if !ok {
			return kernel.OutcomeHandled, nil
		}
		from, ok := s.widenedFrom()
		if !ok {
			from = s.presentation.Selection
		}
		if err := s.SelectProjected(scope.Start, scope.End); err != nil {
			return 0, err
		}
		s.widening = &widening{from: from, revision: s.runtime.Revision()}
		return kernel.OutcomeHandled, nil
	case textSelectDocumentStart, textSelectDocumentEnd:
		viewAnchor, err := s.currentViewAnchor()
		if err != nil {
			return 0, err
		}
		end := model.UTF16Offset(0)
		if command == textSelectDocumentEnd {
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
	case textLeft:
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
	case textRight:
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
	case textSelectLeft:
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
	case textSelectRight:
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
	case textLineStart:
		head = at(node, lineStart)
		anchor = head
	case textLineEnd:
		head = at(node, lineEnd)
		anchor = head
	case textSelectLineStart:
		head = at(node, lineStart)
	case textSelectLineEnd:
		head = at(node, lineEnd)
	case textDocumentStart:
		head = at(first, 0)
		anchor = head
	case textDocumentEnd:
		head = at(last, blockLength(last))
		anchor = head
	case textSelectDocumentStart:
		head = at(first, 0)
	case textSelectDocumentEnd:
		head = at(last, blockLength(last))
	case textSelectAll:
		anchor, head = at(first, 0), at(last, blockLength(last))
	case textBackspace, textDelete:
		from, to := previous(), head
		if command == textDelete {
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
	case textLeft, textRight, textSelectLeft, textSelectRight:
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
