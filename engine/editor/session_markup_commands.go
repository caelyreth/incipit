package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// ExecuteMarkup toggles emphasis for the current semantic selection. Selected
// generated notation is handled by replacing the marker; ordinary ranges use
// the semantic formatting event.
func (s *Session) ExecuteMarkup(command MarkupCommand) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(MarkupPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	if command == MarkupStrong &&
		!markupSettingBool(s.runtime.Configuration(), allowStrongSetting, true) {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorInput,
			Err: &kernel.InputError{
				Kind: kernel.InputErrorInvalidAttributeInput,
				Node: s.presentation.Selection.Node(),
				Key:  legacyEmphasisAttribute,
			},
		}
	}
	beforeFrame, beforeOK := s.historyFrame()
	beforeVisual, visualErr := s.ProjectedState()
	if !beforeOK || visualErr != nil {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	selection := s.presentation.Selection
	ranges := selectionBlockRanges(s.runtime.Snapshot().Document(), selection)
	projected, err := s.VisualSelection()
	if err != nil {
		return 0, err
	}
	if !projected.Empty() {
		for _, segment := range s.presentation.Projection.Segments() {
			target, ok := segment.AttributeTarget()
			if !ok || (!MarkerKeyIsInline(target.Key()) && target.Key() != "emphasis") {
				continue
			}
			if segment.ProjectedRange().Start <= projected.Start &&
				projected.End <= segment.ProjectedRange().End {
				text, err := s.presentation.Projection.TextRange(segment.ProjectedRange())
				if err != nil {
					return 0, err
				}
				runes := []rune(text)
				if len(runes) > 0 {
					replacement := ""
					for index := 0; index < int(command.Level()); index++ {
						replacement += string(runes[0])
					}
					result, err := s.ReplaceSelection(replacement, kernel.EventOriginSemantic)
					return result.Outcome, err
				}
			}
		}
	}
	if markerNotationSelected(s.presentation.Projection, projected) {
		name := "*"
		if value, ok := s.runtime.Configuration().Setting(NewMarkupSettingKey(underscoresSetting)); ok {
			if boolean, ok := value.Bool(); ok && boolean {
				name = "_"
			}
		}
		replacement := ""
		for index := 0; index < int(command.Level()); index++ {
			replacement += name
		}
		result, err := s.ReplaceSelection(replacement, kernel.EventOriginSemantic)
		return result.Outcome, err
	}
	var candidate Presentation
	var active *model.MarkID
	event := kernel.NewCommandEvent(markupFormatTrigger).WithOrigin(kernel.EventOriginSemantic)
	transaction, err := s.runtime.DispatchPreparedWithOutcome(
		event,
		&markupFormatInvoker{
			session: s,
			command: command,
			ranges:  ranges,
		},
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
			if !selection.Spans() {
				active = findExactMark(node, ranges[0].Range)
			}
			projectedValue, projectErr := MarkupProjectDocument(
				view.Document().CandidateSnapshot(view.Revision()),
				view.Revision(),
				s.generation+1,
				s.runtime.Configuration(),
			)
			if projectErr != nil {
				return struct{}{}, projectErr
			}
			caret, projectErr := placeCaret(projectedValue, node, rebased.Head(), active)
			if projectErr != nil {
				return struct{}{}, projectErr
			}
			flow, flowErr := projectionFlowFromView(
				view,
				s.runtime.Configuration(),
				projectedValue,
				rebased,
			)
			if flowErr != nil {
				return struct{}{}, flowErr
			}
			candidate = Presentation{
				Projection: projectedValue,
				Selection:  rebased,
				Caret:      caret,
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
	session *Session
	command MarkupCommand
	ranges  []blockRange
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

// markCovers reports whether marks of kind and level cover all of range16.
func markCovers(node *model.Node, range16 model.UTF16Range, kind model.MarkKind, level uint8) bool {
	end := range16.Start
	for _, mark := range node.Marks() {
		if mark.Kind() == kind && mark.Level() == level &&
			mark.Range().Start <= end && end < mark.Range().End {
			end = max(end, mark.Range().End)
		}
	}
	return end >= range16.End
}

func (i *markupFormatInvoker) Invoke(
	rule *kernel.LinkedRule,
	leaf kernel.LeafID,
	context *kernel.RuleContext,
) (kernel.Outcome, error) {
	if rule == nil || rule.Key.Owner != MarkupPolicyID || rule.Key.ID != "format" {
		return i.session.invoker.Invoke(rule, leaf, context)
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	level := i.command.Level()
	if len(i.ranges) == 1 {
		part := i.ranges[0]
		node, ok := document.Node(part.Node)
		if !ok {
			return 0, &kernel.EngineError{
				Kind: kernel.EngineErrorDocument,
				Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: part.Node},
			}
		}
		for _, mark := range node.Marks() {
			if mark.Range() == part.Range && mark.Kind() == EmphasisMark && mark.Level() == level {
				if err := document.SetMarkLevel(part.Node, mark.ID(), 0); err != nil {
					return 0, err
				}
				return kernel.OutcomeHandled, nil
			}
		}
	}
	// Across blocks the selection gains the mark everywhere it lacks it,
	// or loses it everywhere when it already has it all over.
	covered := true
	for _, part := range i.ranges {
		node, ok := document.Node(part.Node)
		covered = covered && ok && markCovers(node, part.Range, EmphasisMark, level)
	}
	for _, part := range i.ranges {
		node, _ := document.Node(part.Node)
		if part.Range.Empty() && len(i.ranges) > 1 || !covered && markCovers(node, part.Range, EmphasisMark, level) {
			continue
		}
		if err := ToggleMark(document, part.Node, part.Range, EmphasisMark, level); err != nil {
			return 0, err
		}
	}
	return kernel.OutcomeHandled, nil
}

func markerNotationSelected(p *projection.Projection, range16 model.UTF16Range) bool {
	if range16.Empty() {
		return false
	}
	for _, segment := range p.Segments() {
		if _, ok := segment.AttributeTarget(); !ok {
			continue
		}
		if segment.ProjectedRange().Start <= range16.Start &&
			range16.End <= segment.ProjectedRange().End {
			return true
		}
	}
	return false
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
