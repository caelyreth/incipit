package editor

import (
	"errors"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// ErrUnknownCommand reports an identity outside the editor command registry.
var ErrUnknownCommand = errors.New("editor: unknown command")

// ErrNeedsGeometry reports an intent only the page's drawn rows can
// resolve, as moving by a row is: the host resolves it with its layout.
var ErrNeedsGeometry = errors.New("editor: intent needs the page's geometry")

// DoCommand runs the intent a command names. A host command, or one the
// editor does not know, is ErrUnknownCommand.
func (s *Session) DoCommand(command kernel.CommandID, now time.Time) (EditingOutcome, error) {
	declared, ok := CommandFor(command)
	if !ok || !declared.Edits() {
		return EditingOutcome{Outcome: kernel.OutcomePass}, ErrUnknownCommand
	}
	if !s.runtime.Configuration().IsEnabled(declared.Policy) {
		return EditingOutcome{Outcome: kernel.OutcomePass}, nil
	}
	return s.Do(declared.Intent, now)
}

// Do resolves one intent at the caret and applies what answers it. Every
// keystroke, typed text, menu item and palette choice reaches the page
// here.
func (s *Session) Do(intent Intent, now time.Time) (EditingOutcome, error) {
	before := s.runtime.Revision()
	if intent.Kind != IntentDelete || intent.By != ByGrapheme {
		s.disarm()
	}
	if intent.Kind == IntentInsert && intent.Range == nil && s.absorbs(intent.Text) {
		return EditingOutcome{Outcome: kernel.OutcomeHandled, Revision: before}, nil
	}
	s.absorb = nil
	outcome, took, err := s.resolveKind(intent)
	if !took && err == nil {
		outcome, err = s.resolve(intent, now)
	}
	if err != nil {
		return EditingOutcome{}, err
	}
	if outcome.Revision == 0 {
		outcome.Revision = s.runtime.Revision()
	}
	if outcome.Outcome == kernel.OutcomeHandled && outcome.Revision == before {
		outcome.Changes = kernel.EngineChanges{}
	}
	return outcome, nil
}

func (s *Session) resolve(intent Intent, now time.Time) (EditingOutcome, error) {
	command := func(outcome kernel.Outcome, err error) (EditingOutcome, error) {
		return EditingOutcome{Outcome: outcome}, err
	}
	switch intent.Kind {
	case IntentInsert:
		if intent.Range != nil {
			target, err := s.presentation.Projection.Target(*intent.Range)
			if err != nil {
				return EditingOutcome{}, err
			}
			edit, err := FromProjectionTarget(s.presentation.Projection, target, intent.Text, intent.Origin)
			if err != nil {
				return EditingOutcome{}, err
			}
			return s.applyProjectedEditChecked(edit)
		}
		if outcome, typed, err := s.typeAtToken(intent.Text); typed || err != nil {
			return outcome, err
		}
		return s.replaceSelection(intent.Text, intent.Origin)
	case IntentDelete:
		switch intent.By {
		case ByWord:
			return command(s.doWord(pick(intent.Dir, wordDeletePrevious, wordDeleteNext)))
		case ByGrapheme:
			return command(s.doText(pick(intent.Dir, textBackspace, textDelete)))
		}
	case IntentMove:
		switch intent.By {
		case ByGrapheme:
			return command(s.doText(pick(intent.Dir,
				pick(boolDir(intent.Extend), textLeft, textSelectLeft),
				pick(boolDir(intent.Extend), textRight, textSelectRight))))
		case ByLine:
			return command(s.doText(pick(intent.Dir,
				pick(boolDir(intent.Extend), textLineStart, textSelectLineStart),
				pick(boolDir(intent.Extend), textLineEnd, textSelectLineEnd))))
		case ByPage:
			return command(s.doText(pick(intent.Dir,
				pick(boolDir(intent.Extend), textDocumentStart, textSelectDocumentStart),
				pick(boolDir(intent.Extend), textDocumentEnd, textSelectDocumentEnd))))
		case ByWord:
			return command(s.doWord(pick(intent.Dir,
				pick(boolDir(intent.Extend), wordPrevious, wordExtendPrevious),
				pick(boolDir(intent.Extend), wordNext, wordExtendNext))))
		case ByRow:
			return EditingOutcome{Outcome: kernel.OutcomePass}, ErrNeedsGeometry
		}
	case IntentSelect:
		switch intent.By {
		case ByScope:
			return command(s.doText(textSelectAll))
		case ByWord:
			return command(s.doWord(selectWord))
		case BySentence:
			return command(s.doWord(selectSentence))
		case ByParagraph:
			return command(s.doWord(selectParagraph))
		}
	case IntentDismiss:
		return command(s.doText(textDismissSelection))
	case IntentBreak:
		if intent.Soft {
			return s.replaceSelection("\n", kernel.EventOriginPaste)
		}
		return command(s.doText(textNewline))
	case IntentLeave:
		return command(s.leave(now))
	case IntentReturn:
		return command(s.enter())
	case IntentHistory:
		return command(s.doHistory(pick(intent.Dir, historyUndo, historyRedo)))
	case IntentFormat:
		return command(s.doFormat(markupOp(intent.Level)))
	case IntentNest:
		return command(s.doNest(pick(intent.Dir, nestShallower, nestDeeper)))
	case IntentTable:
		return command(s.doTable(intent.Table))
	}
	return EditingOutcome{Outcome: kernel.OutcomePass}, nil
}

// pick is backward's value when dir goes back, forward's otherwise.
func pick[T any](dir Direction, backward, forward T) T {
	if dir == Backward {
		return backward
	}
	return forward
}

// boolDir reads a flag as a direction for pick: set is Forward.
func boolDir(set bool) Direction {
	if set {
		return Forward
	}
	return Backward
}

func markerAdjacent(p *projection.Projection, caret model.UTF16Offset, backward bool) bool {
	for _, segment := range p.Segments() {
		// A caret steps through a mark's notation, where typing edits it,
		// but not a block's: it stands at the text a block's notation
		// opens (blockNotationRest).
		target, ok := segment.AttributeTarget()
		if !ok || !MarkerKeyIsInline(target.Key()) {
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

func lineOffsets(text string, head model.UTF16Offset) (model.UTF16Offset, model.UTF16Offset) {
	byteOffset, ok := model.UTF16OffsetToByte(text, head)
	if !ok {
		return 0, 0
	}
	start := lineStartByte(text, lineOfByte(text, int(byteOffset)))
	end := lineEndByte(text, int(byteOffset))
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
