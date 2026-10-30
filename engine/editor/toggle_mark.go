package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// EmphasisMark is the kind of an emphasis mark: level one for emphasis,
// two for strong.
const EmphasisMark model.MarkKind = "emphasis"

// ToggleMark applies or takes away one kind and level of mark over a range
// of node's text, as a formatting command does. A range already wholly
// covered loses the mark; otherwise every line of it gains it, newlines never
// do, and marks reaching beyond the range keep what lies outside it. An empty
// range toggles an empty mark, waiting for typing.
func ToggleMark(
	draft *model.DocumentDraft,
	node model.NodeID,
	rangeUTF16 model.UTF16Range,
	kind model.MarkKind,
	level uint8,
) error {
	target, ok := draft.Node(node)
	if !ok {
		return &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: node}
	}
	if err := target.ValidateTextRangeUTF16(rangeUTF16.Start, rangeUTF16.End); err != nil {
		return err
	}
	if rangeUTF16.Empty() {
		for _, mark := range target.Marks() {
			if mark.Range() == rangeUTF16 && mark.Kind() == kind && mark.Level() == level {
				return draft.SetMarkLevel(node, mark.ID(), 0)
			}
		}
		_, err := draft.AddMark(node, rangeUTF16, kind, level)
		return err
	}
	byteRange, err := target.TextByteRangeUTF16(rangeUTF16.Start, rangeUTF16.End)
	if err != nil {
		return err
	}
	source := target.Text().Slice(int(byteRange.Start), int(byteRange.End))
	lines := nonEmptyLines(source, rangeUTF16.Start)
	remove := true
	for _, line := range lines {
		end := line.Start
		for _, mark := range target.Marks() {
			if mark.Kind() == kind && mark.Level() == level &&
				mark.Range().Start <= end && end < mark.Range().End {
				end = max(end, mark.Range().End)
			}
		}
		if end < line.End {
			remove = false
			break
		}
	}
	for _, mark := range target.Marks() {
		covered := mark.Range()
		if mark.Kind() != kind || covered.Start >= rangeUTF16.End || rangeUTF16.Start >= covered.End {
			continue
		}
		if err := draft.SetMarkLevel(node, mark.ID(), 0); err != nil {
			return err
		}
		if covered.Start < rangeUTF16.Start {
			if _, err := draft.AddMark(node, model.NewUTF16Range(covered.Start, rangeUTF16.Start), kind, mark.Level()); err != nil {
				return err
			}
		}
		if rangeUTF16.End < covered.End {
			if _, err := draft.AddMark(node, model.NewUTF16Range(rangeUTF16.End, covered.End), kind, mark.Level()); err != nil {
				return err
			}
		}
	}
	if !remove {
		for _, line := range lines {
			if _, err := draft.AddMark(node, line, kind, level); err != nil {
				return err
			}
		}
	}
	return nil
}

func nonEmptyLines(text string, start model.UTF16Offset) []model.UTF16Range {
	ranges := make([]model.UTF16Range, 0)
	offset := start
	for len(text) > 0 {
		newline := strings.IndexByte(text, '\n')
		line := text
		if newline >= 0 {
			line = text[:newline+1]
		}
		content := strings.TrimRight(line, "\r\n")
		if length := model.UTF16Offset(model.StringLenUTF16(content)); length > 0 {
			ranges = append(ranges, model.NewUTF16Range(offset, offset+length))
		}
		offset += model.UTF16Offset(model.StringLenUTF16(line))
		text = text[len(line):]
	}
	return ranges
}
