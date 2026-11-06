package editor

import (
	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// doFormat toggles emphasis for the current semantic selection. Selected
// generated notation is handled by replacing the marker; ordinary ranges use
// the semantic formatting event.
func (s *Session) doFormat(command markupOp) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(WritingPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	selection := s.presentation.Selection
	document := s.runtime.Snapshot().Document()
	ranges := selectionBlockRanges(document, selection)
	projected, err := s.VisualSelection()
	if err != nil {
		return 0, err
	}
	// Selected notation of emphasis is rewritten to the command's; other
	// notation is no place for it.
	if target, ok := notationWithin(s.presentation.Projection, projected); ok {
		node, _ := s.runtime.Snapshot().Document().Node(target.Node())
		for _, mark := range node.Marks() {
			if MarkerKeyIdentifies(target.Key(), mark.ID()) && mark.Kind() == FormatMark {
				notation := inlineNotation(model.NewInlineMark(0, model.UTF16Range{}, FormatMark, command.Level()))
				result, err := s.replaceSelection(notation, kernel.EventOriginSemantic)
				return result.Outcome, err
			}
		}
		return kernel.OutcomePass, nil
	}
	invoker := &markupFormatInvoker{
		command:  command,
		ranges:   ranges,
		coverage: formattingCoverageFor(document, ranges, FormatMark, activeMark(s.inline)),
	}
	return s.dispatchSelectionCommand(
		markupFormatTrigger,
		s.command(formatRule, invoker.run),
		func(node *model.Node) *model.MarkID {
			if selection.Spans() {
				return nil
			}
			return findExactMark(node, ranges[0].Range)
		},
	)
}

// dispatchSelectionCommand dispatches a command that acts on what the
// selection covers, as formatting and nesting do, and installs the page it
// leaves: the selection carried through its edits, the caret in the mark
// active names. It records one history step.
func (s *Session) dispatchSelectionCommand(
	trigger string,
	invoker kernel.EngineInvoker,
	active func(*model.Node) *model.MarkID,
) (kernel.Outcome, error) {
	return s.dispatchSelectionCommandWithVisual(trigger, invoker, active, false)
}

// dispatchSelectionCommandWithVisual can retain the exact projected selection
// for commands that change no text or notation positions, such as checking an
// item elsewhere on the page.
func (s *Session) dispatchSelectionCommandWithVisual(
	trigger string,
	invoker kernel.EngineInvoker,
	active func(*model.Node) *model.MarkID,
	preserveVisual bool,
) (kernel.Outcome, error) {
	beforeFrame, beforeOK := s.historyFrame()
	beforeVisual, visualErr := s.ProjectedState()
	if !beforeOK || visualErr != nil {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	selection := s.presentation.Selection
	var candidate Presentation
	event := kernel.NewCommandEvent(trigger).WithOrigin(kernel.EventOriginSemantic)
	transaction, err := s.runtime.DispatchPreparedWithOutcome(
		event,
		invoker,
		func(view kernel.EngineView, changes kernel.EngineChanges, outcome kernel.DispatchOutcome) (struct{}, error) {
			if outcome.Status != kernel.OutcomeHandled {
				return struct{}{}, nil
			}
			rebased, ok := selection.RebaseAll(changes.TextEdits(), view.Revision())
			if !ok {
				return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			}
			node, ok := view.Document().Node(rebased.Node())
			if !ok {
				return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			}
			projectedValue, projectErr := ProjectPage(
				view.Document(),
				view.Revision(),
				s.generation+1,
				s.runtime.Configuration(),
			)
			if projectErr != nil {
				return struct{}{}, projectErr
			}
			caret, projectErr := placeCaret(projectedValue, node, rebased.Head(), active(node))
			if projectErr != nil {
				return struct{}{}, projectErr
			}
			var viewAnchor *projection.Target
			if preserveVisual {
				if len(changes.TextEdits()) != 0 {
					return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
				}
				anchor, head, resolveErr := projectedValue.ResolveSelection(beforeVisual, view.Revision())
				if resolveErr != nil {
					return struct{}{}, resolveErr
				}
				caret, viewAnchor = head, &anchor
			}
			flow, flowErr := s.pieces.pageFlow(projectedValue, view.Document())
			if flowErr != nil {
				return struct{}{}, flowErr
			}
			candidate = Presentation{
				Projection: projectedValue,
				Selection:  rebased,
				Caret:      caret,
				ViewAnchor: viewAnchor,
				Flow:       flow,
			}
			return struct{}{}, nil
		},
	)
	if err != nil {
		return 0, err
	}
	if transaction.Value.Status == kernel.OutcomeHandled {
		s.applyPresentation(candidate)
		s.history.Record(beforeFrame, beforeVisual, !transaction.Changes.Empty(), false)
	}
	return transaction.Value.Status, nil
}

type markupFormatInvoker struct {
	session  *Session
	command  markupOp
	ranges   []blockRange
	coverage formattingCoverage
}

// blockRange is the part of a selection that lies in one block.
type blockRange struct {
	Node  model.NodeID
	Range model.UTF16Range
}

// selectionBlockRanges splits a selection into its part in each block it
// covers, in page order.
func selectionBlockRanges(document model.DocumentSnapshot, selection model.TextSelection) []blockRange {
	if !selection.Spans() {
		range16, _ := selection.Range()
		return []blockRange{{Node: selection.Node(), Range: range16}}
	}
	first, last := orderedAnchors(document, selection.Anchor(), selection.Head())
	blocks := blocksBetween(document, first.Node(), last.Node())
	ranges := make([]blockRange, 0, len(blocks))
	for index, id := range blocks {
		node, _ := document.Node(id)
		start, end := model.UTF16Offset(0), blockLength(node)
		if index == 0 {
			start = first.Offset()
		}
		if index == len(blocks)-1 {
			end = last.Offset()
		}
		ranges = append(ranges, blockRange{Node: id, Range: model.NewUTF16Range(start, end)})
	}
	return ranges
}

func (i *markupFormatInvoker) run(context *kernel.RuleContext) (kernel.Outcome, error) {
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	level := i.command.Level()
	if i.coverage.collapsed && len(i.ranges) == 1 {
		part := i.ranges[0]
		node, ok := document.Node(part.Node)
		if !ok {
			return 0, &kernel.EngineError{
				Kind: kernel.EngineErrorDocument,
				Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: part.Node},
			}
		}
		for _, mark := range node.Marks() {
			if mark.Range() == part.Range && mark.Kind() == FormatMark && mark.Level() == level &&
				(i.coverage.active == nil || mark.ID() == i.coverage.active.ID()) {
				if err := document.SetMarkLevel(part.Node, mark.ID(), 0); err != nil {
					return 0, err
				}
				return kernel.OutcomeHandled, nil
			}
		}
	}
	if i.coverage.collapsed {
		if i.coverage.active != nil && i.coverage.active.Level() == level {
			if err := document.SetMarkLevel(i.ranges[0].Node, i.coverage.active.ID(), 0); err != nil {
				return 0, err
			}
			return kernel.OutcomeHandled, nil
		}
		if _, err := applyMarkToggle(
			document,
			i.ranges[0].Node,
			i.ranges[0].Range,
			FormatMark,
			level,
			false,
		); err != nil {
			return 0, err
		}
		return kernel.OutcomeHandled, nil
	}
	// Across blocks the selection gains the mark everywhere it lacks it, or
	// loses it everywhere when it is the strongest effective level throughout.
	remove := i.coverage.strongest() == level
	for index, part := range i.ranges {
		if part.Range.Empty() && len(i.ranges) > 1 {
			continue
		}
		if !remove && i.coverage.partStrongest(index) == level {
			continue
		}
		if _, err := applyMarkToggle(
			document,
			part.Node,
			part.Range,
			FormatMark,
			level,
			remove,
		); err != nil {
			return 0, err
		}
	}
	return kernel.OutcomeHandled, nil
}

// notationWithin is the notation a non-empty projected range lies within.
func notationWithin(p *projection.Projection, range16 model.UTF16Range) (input.AttributeTarget, bool) {
	if range16.Empty() {
		return input.AttributeTarget{}, false
	}
	for _, segment := range p.Segments() {
		target, ok := segment.AttributeTarget()
		if ok && segment.ProjectedRange().Start <= range16.Start &&
			range16.End <= segment.ProjectedRange().End {
			return target, true
		}
	}
	return input.AttributeTarget{}, false
}

func findExactMark(node *model.Node, range16 model.UTF16Range) *model.MarkID {
	var newest *model.MarkID
	for _, mark := range node.Marks() {
		if mark.Range() != range16 {
			continue
		}
		if newest == nil || mark.ID() > *newest {
			id := mark.ID()
			newest = &id
		}
	}
	return newest
}
