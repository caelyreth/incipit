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
	current := s.runtime.Snapshot().Document()
	node := frame.selection.Node()
	was, restored := textChange(blockText(current, node), blockText(frame.document, node))
	phase, playing := s.feedback.Take(node, was, now)
	carried := historyEdits(current, frame.document)
	var candidate Presentation
	transaction, err := s.runtime.TransactPrepared(
		func(draft *kernel.EngineDraft) error {
			return draft.Document().RestoreDocument(frame.document)
		},
		func(view kernel.EngineView, _ kernel.EngineChanges) (struct{}, error) {
			selection, ok := frame.selection.RebaseAll(nil, view.Revision())
			if !ok {
				return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			}
			projected, err := ProjectPage(
				view.Document(),
				view.Revision(),
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
			flow, err := s.pieces.pageFlow(projected, view.Document())
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
	revision := transaction.Revision
	s.feedback.Rebase(carried, revision)
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

// historyEdits are the text changes moving from one page to another makes
// in the blocks both hold, each as the one replacement that covers it, so
// what follows text, as a pulse does, keeps its place across the move.
func historyEdits(from, to model.DocumentSnapshot) []model.TextEdit {
	edits := make([]model.TextEdit, 0, 1)
	for _, block := range DocumentBlocks(from) {
		if _, kept := to.Node(block.ID()); !kept {
			continue
		}
		was, restored := textChange(blockText(from, block.ID()), blockText(to, block.ID()))
		if was.Empty() && restored.Empty() {
			continue
		}
		edits = append(edits, model.NewTextEdit(block.ID(), was, restored.End-restored.Start))
	}
	return edits
}

// textChange is the one replacement that turns before into after: the
// range it replaces in before, and the range its text takes in after.
func textChange(before, after string) (was, restored model.UTF16Range) {
	left, right := utf16.Encode([]rune(before)), utf16.Encode([]rune(after))
	prefix := 0
	for prefix < len(left) && prefix < len(right) && left[prefix] == right[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(left)-prefix && suffix < len(right)-prefix &&
		left[len(left)-1-suffix] == right[len(right)-1-suffix] {
		suffix++
	}
	was = model.NewUTF16Range(model.UTF16Offset(prefix), model.UTF16Offset(len(left)-suffix))
	restored = model.NewUTF16Range(model.UTF16Offset(prefix), model.UTF16Offset(len(right)-suffix))
	return was, restored
}

// blockText is a block's text on a page, or nothing when the page lacks it.
func blockText(document model.DocumentSnapshot, node model.NodeID) string {
	block, ok := document.Node(node)
	if !ok || !block.TextPresent() {
		return ""
	}
	return block.Text().String()
}
