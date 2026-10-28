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
	switch i.command {
	case MarkupStrong:
		if !contextMarkupBool(context, allowStrongSetting, true) {
			return 0, &kernel.EngineError{
				Kind: kernel.EngineErrorInput,
				Err: &kernel.InputError{
					Kind: kernel.InputErrorInvalidAttributeInput,
					Node: nodeID,
					Key:  "emphasis",
				},
			}
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

// ExecuteWord applies word movement, deletion, or enclosing-unit selection.
func (s *Session) ExecuteWord(command WordCommand) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(WordsPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	selection := s.presentation.Selection
	snapshot := s.runtime.Snapshot()
	node, ok := snapshot.Document().Node(selection.Node())
	if !ok || !node.TextPresent() {
		return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	text := node.Text().String()
	plan, err := PlanWordCommand(
		text,
		WordSelection{
			Anchor: selection.Anchor().OffsetUTF16(),
			Head:   selection.Head().OffsetUTF16(),
		},
		command,
	)
	if err != nil {
		return 0, err
	}
	if plan.Delete {
		if plan.DeleteAt.Empty() {
			return kernel.OutcomeHandled, nil
		}
		anchor, err := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(selection.Node(), plan.DeleteAt.Start, model.AffinityRight, snapshot.Revision()),
		)
		if err != nil {
			return 0, err
		}
		head, err := s.presentation.Projection.ProjectAnchor(
			model.NewTextAnchor(selection.Node(), plan.DeleteAt.End, model.AffinityLeft, snapshot.Revision()),
		)
		if err != nil {
			return 0, err
		}
		edit, err := FromProjectedRange(
			s.presentation.Projection,
			model.NewUTF16Range(
				min(anchor.RangeUTF16().Start, head.RangeUTF16().Start),
				max(anchor.RangeUTF16().Start, head.RangeUTF16().Start),
			),
			"",
			kernel.EventOriginSemantic,
		)
		if err != nil {
			return 0, err
		}
		result, err := s.applyProjectedEdit(edit)
		return result.Outcome, err
	}
	return kernel.OutcomeHandled, s.Select(model.NewTextSelection(
		model.NewTextAnchor(selection.Node(), plan.Anchor, model.AffinityRight, snapshot.Revision()),
		model.NewTextAnchor(selection.Node(), plan.Head, model.AffinityRight, snapshot.Revision()),
	))
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
