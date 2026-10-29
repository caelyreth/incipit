package editor

import (
	"unicode/utf16"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
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
	var err error
	switch command {
	case HistoryUndo:
		_, err = s.history.Undo(current, func(frame Frame) error {
			return s.restoreHistoryFrame(frame, true)
		})
	case HistoryRedo:
		_, err = s.history.Redo(current, func(frame Frame) error {
			return s.restoreHistoryFrame(frame, false)
		})
	default:
		return kernel.OutcomePass, nil
	}
	if err != nil {
		return 0, err
	}
	return kernel.OutcomeHandled, nil
}

// restoreHistoryFrame moves to frame, back for an undo and forward for a
// redo. Correction pulses go with the text: a step that made a correction,
// or one that takes back text a pulse still plays over, plays that pulse
// back on undo, from where it was, and forward again on redo; every other
// pulse carries on over its text.
func (s *Session) restoreHistoryFrame(frame Frame, undo bool) error {
	now := s.feedback.clock.Now()
	node := frame.node.ID()
	was, changed, restored := s.historyChange(frame)
	phase, playing := s.feedback.Take(node, was, now)
	var candidate Presentation
	transaction, err := s.runtime.TransactPrepared(
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
	s.refreshMetrics()
	s.navigation.Clear()
	revision := transaction.Revision
	s.feedback.Rebase([]model.TextEdit{model.NewTextEdit(node, was, changed)}, revision)
	if (frame.corrected || playing) && !restored.Empty() {
		targets := []input.TextTarget{input.NewTextTarget(node, restored, revision)}
		switch {
		case undo && playing:
			s.feedback.Rewind(targets, phase, now)
		case undo:
			s.feedback.Rewind(targets, 1, now)
		default:
			s.feedback.Replay(targets, phase, now)
		}
	}
	return nil
}

// historyChange is what moving to frame changes in its node's text: the
// range it replaces in the text now, the length of what takes its place,
// and that range in the restored text.
func (s *Session) historyChange(frame Frame) (was model.UTF16Range, length model.UTF16Offset, restored model.UTF16Range) {
	before := utf16.Encode([]rune(nodeText(mustNode(s.runtime.Snapshot().Document(), frame.node.ID()))))
	after := utf16.Encode([]rune(nodeText(&frame.node)))
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(before)-prefix && suffix < len(after)-prefix &&
		before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	was = model.NewUTF16Range(model.UTF16Offset(prefix), model.UTF16Offset(len(before)-suffix))
	restored = model.NewUTF16Range(model.UTF16Offset(prefix), model.UTF16Offset(len(after)-suffix))
	return was, restored.End - restored.Start, restored
}

func nodeText(node *model.Node) string {
	if node == nil || !node.TextPresent() {
		return ""
	}
	return node.Text().String()
}
