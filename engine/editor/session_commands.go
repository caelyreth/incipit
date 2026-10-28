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
	node, ok := snapshot.Document().Node(selection.Node())
	if !ok {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	text := ""
	if node.TextPresent() {
		text = node.Text().String()
	}
	range16, err := selection.Range()
	if err != nil {
		return 0, err
	}
	projected, err := s.VisualSelection()
	if err != nil {
		return 0, err
	}
	anchor := selection.Anchor().OffsetUTF16()
	head := selection.Head().OffsetUTF16()

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

	previous := func() model.UTF16Offset {
		if grapheme, ok := PreviousGrapheme(text, head); ok {
			return grapheme.Start
		}
		return 0
	}
	next := func() model.UTF16Offset {
		if grapheme, ok := NextGrapheme(text, head); ok {
			return grapheme.End
		}
		return model.UTF16Offset(model.StringLenUTF16(text))
	}
	lineStart, lineEnd := lineOffsets(text, head)
	length := model.UTF16Offset(model.StringLenUTF16(text))
	switch command {
	case TextLeft:
		offset := previous()
		if !range16.Empty() {
			offset = range16.Start
		}
		anchor, head = offset, offset
	case TextRight:
		offset := next()
		if !range16.Empty() {
			offset = range16.End
		}
		anchor, head = offset, offset
	case TextSelectLeft:
		head = previous()
	case TextSelectRight:
		head = next()
	case TextLineStart:
		anchor, head = lineStart, lineStart
	case TextLineEnd:
		anchor, head = lineEnd, lineEnd
	case TextSelectLineStart:
		head = lineStart
	case TextSelectLineEnd:
		head = lineEnd
	case TextDocumentStart:
		anchor, head = 0, 0
	case TextDocumentEnd:
		anchor, head = length, length
	case TextSelectDocumentStart:
		head = 0
	case TextSelectDocumentEnd:
		head = length
	case TextSelectAll:
		anchor, head = 0, length
	case TextBackspace, TextDelete:
		target := model.NewUTF16Range(previous(), head)
		if command == TextDelete {
			target = model.NewUTF16Range(head, next())
		}
		if !range16.Empty() {
			target = range16
		}
		if target.Empty() {
			return kernel.OutcomeHandled, nil
		}
		projectedTarget, projectErr := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(selection.Node(), target.Start, model.AffinityRight, snapshot.Revision()),
		)
		if projectErr != nil {
			return 0, projectErr
		}
		projectedHead, projectErr := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(selection.Node(), target.End, model.AffinityLeft, snapshot.Revision()),
		)
		if projectErr != nil {
			return 0, projectErr
		}
		edit, editErr := FromProjectedRange(
			s.presentation.Projection,
			model.NewUTF16Range(
				min(projectedTarget.RangeUTF16().Start, projectedHead.RangeUTF16().Start),
				max(projectedTarget.RangeUTF16().Start, projectedHead.RangeUTF16().Start),
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
	if anchor == selection.Anchor().OffsetUTF16() && head == selection.Head().OffsetUTF16() {
		return kernel.OutcomeHandled, nil
	}
	if err := s.Select(model.NewTextSelection(
		model.NewTextAnchor(selection.Node(), anchor, model.AffinityRight, snapshot.Revision()),
		model.NewTextAnchor(selection.Node(), head, model.AffinityRight, snapshot.Revision()),
	)); err != nil {
		return 0, err
	}
	return kernel.OutcomeHandled, nil
}

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
	var restored bool
	var err error
	switch command {
	case HistoryUndo:
		restored, err = s.history.Undo(current, restore)
	case HistoryRedo:
		restored, err = s.history.Redo(current, restore)
	default:
		return kernel.OutcomePass, nil
	}
	if err != nil {
		return 0, err
	}
	if !restored {
		return kernel.OutcomeHandled, nil
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
			caret, err := projected.ProjectAnchor(selection.Head())
			if err != nil {
				return struct{}{}, err
			}
			flow, err := projectionFlowFromView(view, s.runtime.Configuration(), projected, selection)
			if err != nil {
				return struct{}{}, err
			}
			candidate = Presentation{
				Projection: projected,
				Selection:  selection,
				Caret:      caret,
				Flow:       flow,
			}
			return struct{}{}, nil
		},
	)
	if err != nil {
		return err
	}
	s.applyPresentation(candidate)
	s.inline.Activate(frame.active)
	s.navigation.Clear()
	s.feedback.Clear()
	return nil
}

func markerAdjacent(p *projection.Projection, caret model.UTF16Offset, backward bool) bool {
	for _, segment := range p.Segments() {
		if _, ok := segment.AttributeTarget(); !ok {
			continue
		}
		r := segment.ProjectedRange()
		if backward && r.Start < caret && caret <= r.End ||
			!backward && r.Start <= caret && caret < r.End {
			return true
		}
	}
	return false
}

func markerDeletionRange(
	p *projection.Projection,
	caret model.UTF16Offset,
	backward bool,
) (model.UTF16Range, bool) {
	for _, segment := range p.Segments() {
		if _, ok := segment.AttributeTarget(); !ok {
			continue
		}
		r := segment.ProjectedRange()
		if backward {
			if r.End == caret || caret == 0 && r.Start == 0 {
				return model.NewUTF16Range(max(0, caret-1), caret), true
			}
		} else if r.Start == caret || caret == p.UTF16Len() && r.End == caret {
			return model.NewUTF16Range(caret, min(caret+1, p.UTF16Len())), true
		}
	}
	return model.UTF16Range{}, false
}

func lineOffsets(text string, head model.UTF16Offset) (model.UTF16Offset, model.UTF16Offset) {
	byteOffset, ok := model.UTF16OffsetToByte(text, head)
	if !ok {
		return 0, 0
	}
	start := 0
	for index := int(byteOffset) - 1; index >= 0; index-- {
		if text[index] == '\n' {
			start = index + 1
			break
		}
	}
	end := int(byteOffset)
	for end < len(text) && text[end] != '\n' && text[end] != '\r' {
		end++
	}
	return model.UTF16Offset(model.StringLenUTF16(text[:start])),
		model.UTF16Offset(model.StringLenUTF16(text[:end]))
}

func (s *Session) currentViewAnchor() (model.UTF16Offset, error) {
	if s.presentation.ViewAnchor != nil {
		return s.presentation.ViewAnchor.RangeUTF16().Start, nil
	}
	target, err := s.presentation.Projection.ProjectAnchor(s.presentation.Selection.Anchor())
	if err != nil {
		return 0, err
	}
	return target.RangeUTF16().Start, nil
}
