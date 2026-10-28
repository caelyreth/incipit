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
	beforeFrame, beforeOK := s.historyFrame()
	beforeVisual, visualErr := s.ProjectedState()
	if !beforeOK || visualErr != nil {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	selection := s.presentation.Selection
	range16, err := selection.Range()
	if err != nil {
		return 0, err
	}
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
			range16: range16,
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
			active = findExactMark(node, range16)
			projectedValue, projectErr := MarkupProjectDocument(
				view.Document().CandidateSnapshot(view.Revision()),
				view.Revision(),
				rebased.Node(),
				s.generation+1,
				s.runtime.Configuration(),
			)
			if projectErr != nil {
				return struct{}{}, projectErr
			}
			caret, projectErr := projectedValue.ProjectAnchor(rebased.Head())
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
		s.inline.Activate(active)
		s.history.Record(beforeFrame, beforeVisual, !transaction.Changes.Empty(), false)
	}
	return transaction.Value.Status, nil
}

type markupFormatInvoker struct {
	session *Session
	command MarkupCommand
	range16 model.UTF16Range
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
	nodeID := i.session.Selection().Node()
	node, ok := document.Node(nodeID)
	if !ok {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: nodeID},
		}
	}
	level := i.command.Level()
	for _, mark := range node.Marks() {
		if mark.Range() == i.range16 && mark.Level() == level {
			if err := document.SetMarkLevel(nodeID, mark.ID(), 0); err != nil {
				return 0, err
			}
			return kernel.OutcomeHandled, nil
		}
	}
	if err := document.FormatEmphasis(nodeID, i.range16, level); err != nil {
		return 0, err
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
