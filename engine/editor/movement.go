package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

type TextCommand uint8

const (
	TextLeft TextCommand = iota + 1
	TextRight
	TextSelectLeft
	TextSelectRight
	TextLineStart
	TextLineEnd
	TextSelectLineStart
	TextSelectLineEnd
	TextDocumentStart
	TextDocumentEnd
	TextSelectDocumentStart
	TextSelectDocumentEnd
	TextSelectAll
	TextBackspace
	TextDelete
	TextNewline
)

var AllTextCommands = [...]TextCommand{
	TextLeft, TextRight, TextSelectLeft, TextSelectRight,
	TextLineStart, TextLineEnd, TextSelectLineStart, TextSelectLineEnd,
	TextDocumentStart, TextDocumentEnd, TextSelectDocumentStart,
	TextSelectDocumentEnd, TextSelectAll, TextBackspace, TextDelete,
	TextNewline,
}

func (c TextCommand) Description() Command {
	meta := Command{Policy: EditorTextPolicyID, BindingCtx: kernel.EditorBindingContext}
	switch c {
	case TextLeft:
		meta.ID, meta.Label, meta.Key = "text.left", "Move left", "left"
	case TextRight:
		meta.ID, meta.Label, meta.Key = "text.right", "Move right", "right"
	case TextSelectLeft:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-left", "Extend left", "left", kernel.ModifiersShift
	case TextSelectRight:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-right", "Extend right", "right", kernel.ModifiersShift
	case TextLineStart:
		meta.ID, meta.Label, meta.Key = "text.line-start", "Start of line", "home"
	case TextLineEnd:
		meta.ID, meta.Label, meta.Key = "text.line-end", "End of line", "end"
	case TextSelectLineStart:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-line-start", "Extend to line start", "home", kernel.ModifiersShift
	case TextSelectLineEnd:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-line-end", "Extend to line end", "end", kernel.ModifiersShift
	case TextDocumentStart:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.document-start", "Start of document", "home", kernel.ModifiersPrimary
	case TextDocumentEnd:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.document-end", "End of document", "end", kernel.ModifiersPrimary
	case TextSelectDocumentStart:
		meta.ID, meta.Label, meta.Key = "text.select-document-start", "Extend to document start", "home"
		meta.Modifiers = kernel.ModifiersPrimary | kernel.ModifiersShift
	case TextSelectDocumentEnd:
		meta.ID, meta.Label, meta.Key = "text.select-document-end", "Extend to document end", "end"
		meta.Modifiers = kernel.ModifiersPrimary | kernel.ModifiersShift
	case TextSelectAll:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-all", "Select all", "a", kernel.ModifiersPrimary
	case TextBackspace:
		meta.ID, meta.Label, meta.Key = "text.backspace", "Delete backward", "backspace"
	case TextDelete:
		meta.ID, meta.Label, meta.Key = "text.delete", "Delete forward", "delete"
	case TextNewline:
		meta.ID, meta.Label, meta.Key = "text.newline", "Insert newline", "enter"
	}
	return meta
}

func (c TextCommand) ID() kernel.CommandID { return c.Description().ID }
func (c TextCommand) Label() string        { return c.Description().Label }

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
		prefix := text[:byteOffset]
		if newline := strings.LastIndexByte(prefix, '\n'); newline >= 0 {
			return model.UTF16Offset(model.StringLenUTF16(text[:newline+1]))
		}
		return 0
	}
	lineEnd := func(offset model.UTF16Offset) model.UTF16Offset {
		byteOffset, ok := model.UTF16OffsetToByte(text, offset)
		if !ok {
			return 0
		}
		end := int(byteOffset)
		for end < len(text) && text[end] != '\n' && text[end] != '\r' {
			end++
		}
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
