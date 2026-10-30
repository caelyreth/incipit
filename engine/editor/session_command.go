package editor

import (
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// ExecuteCommand is the common command entry point used by controllers and
// keymap adapters. It returns Pass for an unknown or disabled command.
func (s *Session) ExecuteCommand(command kernel.CommandID, now time.Time) (kernel.Outcome, error) {
	for _, textCommand := range AllTextCommands {
		if textCommand.ID() == command {
			return s.ExecuteText(textCommand)
		}
	}
	for _, historyCommand := range AllHistoryCommands {
		if historyCommand.ID() == command {
			return s.ExecuteHistory(historyCommand)
		}
	}
	for _, wordCommand := range AllWordCommands {
		if wordCommand.ID() == command {
			return s.ExecuteWord(wordCommand)
		}
	}
	switch command {
	case MarkupEmphasis.ID():
		return s.ExecuteMarkup(MarkupEmphasis)
	case MarkupStrong.ID():
		return s.ExecuteMarkup(MarkupStrong)
	case TabCommand().ID:
		return s.Tab(now)
	default:
		return kernel.OutcomePass, nil
	}
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
