package editor

import "github.com/caelyreth/neverland/engine/model"

type TextPlan struct {
	Command  TextCommand
	Anchor   model.UTF16Offset
	Head     model.UTF16Offset
	Delete   bool
	DeleteAt model.UTF16Range
	Replace  *string
}

func PlanTextCommand(
	text string,
	anchor model.UTF16Offset,
	head model.UTF16Offset,
	command TextCommand,
) (TextPlan, error) {
	if !model.ValidDocumentText(text) {
		return TextPlan{}, &WordError{Kind: WordErrorInvalidText}
	}
	if !model.UTF16OffsetIsBoundary(text, anchor) ||
		!model.UTF16OffsetIsBoundary(text, head) {
		return TextPlan{}, &WordError{Kind: WordErrorInvalidOffset}
	}
	start, end := min(anchor, head), max(anchor, head)
	collapsed := start == end
	previous := func(offset model.UTF16Offset) model.UTF16Offset {
		if grapheme, ok := PreviousGrapheme(text, offset); ok {
			return grapheme.Start
		}
		return 0
	}
	next := func(offset model.UTF16Offset) model.UTF16Offset {
		if grapheme, ok := NextGrapheme(text, offset); ok {
			return grapheme.End
		}
		return model.UTF16Offset(model.StringLenUTF16(text))
	}
	lineStart := func(offset model.UTF16Offset) model.UTF16Offset {
		byteOffset, ok := model.UTF16OffsetToByte(text, offset)
		if !ok {
			return 0
		}
		start := lineStartByte(text, lineOfByte(text, int(byteOffset)))
		return model.UTF16Offset(model.StringLenUTF16(text[:start]))
	}
	lineEnd := func(offset model.UTF16Offset) model.UTF16Offset {
		byteOffset, ok := model.UTF16OffsetToByte(text, offset)
		if !ok {
			return 0
		}
		end := lineEndByte(text, int(byteOffset))
		return model.UTF16Offset(model.StringLenUTF16(text[:end]))
	}
	length := model.UTF16Offset(model.StringLenUTF16(text))
	plan := TextPlan{Command: command, Anchor: anchor, Head: head}

	switch command {
	case TextLeft:
		if !collapsed {
			plan.Anchor, plan.Head = start, start
		} else {
			plan.Anchor, plan.Head = previous(head), previous(head)
		}
	case TextRight:
		if !collapsed {
			plan.Anchor, plan.Head = end, end
		} else {
			plan.Anchor, plan.Head = next(head), next(head)
		}
	case TextSelectLeft:
		plan.Head = previous(head)
	case TextSelectRight:
		plan.Head = next(head)
	case TextLineStart:
		offset := lineStart(head)
		plan.Anchor, plan.Head = offset, offset
	case TextLineEnd:
		offset := lineEnd(head)
		plan.Anchor, plan.Head = offset, offset
	case TextSelectLineStart:
		plan.Head = lineStart(head)
	case TextSelectLineEnd:
		plan.Head = lineEnd(head)
	case TextDocumentStart:
		plan.Anchor, plan.Head = 0, 0
	case TextDocumentEnd:
		plan.Anchor, plan.Head = length, length
	case TextSelectDocumentStart:
		plan.Head = 0
	case TextSelectDocumentEnd:
		plan.Head = length
	case TextSelectAll:
		plan.Anchor, plan.Head = 0, length
	case TextBackspace, TextDelete:
		plan.Delete = true
		if !collapsed {
			plan.DeleteAt = model.NewUTF16Range(start, end)
			break
		}
		if atCluster, ok := GraphemeAt(text, head); ok && atCluster.Start < head && head < atCluster.End {
			plan.DeleteAt = atCluster
		} else if command == TextBackspace {
			plan.DeleteAt = model.NewUTF16Range(previous(head), head)
		} else {
			plan.DeleteAt = model.NewUTF16Range(head, next(head))
		}
	case TextNewline:
		replacement := "\n"
		plan.Replace = &replacement
		plan.Delete = true
		plan.DeleteAt = model.NewUTF16Range(start, end)
	default:
		return TextPlan{}, &WordError{Kind: WordErrorInvalidCommand}
	}
	return plan, nil
}
