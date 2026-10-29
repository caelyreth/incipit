package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
)

// ExecuteHistory restores a history frame through the runtime transaction.
func (s *Session) ExecuteHistory(command HistoryCommand) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(HistoryPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	current, ok := s.historyFrame()
	if !ok {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	restore := func(frame Frame) error {
		return s.restoreHistoryFrame(frame)
	}
	var err error
	switch command {
	case HistoryUndo:
		_, err = s.history.Undo(current, restore)
	case HistoryRedo:
		_, err = s.history.Redo(current, restore)
	default:
		return kernel.OutcomePass, nil
	}
	if err != nil {
		return 0, err
	}
	return kernel.OutcomeHandled, nil
}

func (s *Session) restoreHistoryFrame(frame Frame) error {
	var candidate Presentation
	_, err := s.runtime.TransactPrepared(
		func(draft *kernel.EngineDraft) error {
			return draft.Document().RestoreContent(&frame.node)
		},
		func(view kernel.EngineView, _ kernel.EngineChanges) (struct{}, error) {
			selection, ok := frame.selection.RebaseAll(nil, view.Revision())
			if !ok {
				return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			}
			projected, err := MarkupProjectDocument(
				view.Document().CandidateSnapshot(view.Revision()),
				view.Revision(),
				selection.Node(),
				s.generation+1,
				s.runtime.Configuration(),
			)
			if err != nil {
				return struct{}{}, err
			}
			node, _ := view.Document().Node(selection.Node())
			caret, err := placeCaret(projected, node, selection.Head(), frame.active)
			if err != nil {
				return struct{}{}, err
			}
			var viewAnchor *projection.Target
			if anchor, head, resolveErr := projected.ResolveSelection(
				frame.visual,
				view.Revision(),
			); resolveErr == nil {
				caret = head
				viewAnchor = &anchor
			}
			flow, err := projectionFlowFromView(view, s.runtime.Configuration(), projected, selection)
			if err != nil {
				return struct{}{}, err
			}
			candidate = Presentation{
				Projection: projected,
				Selection:  selection,
				Caret:      caret,
				ViewAnchor: viewAnchor,
				Flow:       flow,
			}
			return struct{}{}, nil
		},
	)
	if err != nil {
		return err
	}
	s.applyPresentation(candidate)
	s.navigation.Clear()
	s.feedback.Clear()
	return nil
}
