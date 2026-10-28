package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// SelectionUnit is an enclosing semantic unit selected by a command.
type SelectionUnit uint8

const (
	SelectionWord SelectionUnit = iota + 1
	SelectionSentence
	SelectionParagraph
)

// WordSelection is a directional semantic selection in one node.
type WordSelection struct {
	Anchor model.UTF16Offset
	Head   model.UTF16Offset
}

// Normalized returns the directional range without losing selection direction.
func (s WordSelection) Normalized() model.UTF16Range {
	return model.NewUTF16Range(min(s.Anchor, s.Head), max(s.Anchor, s.Head))
}

// Collapsed reports whether both endpoints coincide.
func (s WordSelection) Collapsed() bool { return s.Anchor == s.Head }

// WordCommandPlan is a pure selection or deletion proposal. Deleting commands
// provide DeleteRange for projectable semantic input; non-deleting commands
// provide Anchor and Head for the next semantic selection.
type WordCommandPlan struct {
	Command    WordCommand
	Anchor     model.UTF16Offset
	Head       model.UTF16Offset
	Delete     bool
	DeleteAt   model.UTF16Range
	SelectUnit bool
	Unit       SelectionUnit
}

// WordErrorKind identifies malformed text or unit selection requests.
type WordErrorKind uint8

const (
	WordErrorInvalidText WordErrorKind = iota + 1
	WordErrorInvalidOffset
	WordErrorInvalidUnit
	WordErrorInvalidCommand
)

// WordError reports a word or unit selection contract failure.
type WordError struct {
	Kind WordErrorKind
}

func (e *WordError) Error() string {
	switch e.Kind {
	case WordErrorInvalidText:
		return "word segmentation requires valid UTF-8 without NUL"
	case WordErrorInvalidOffset:
		return "word selection offset is not a scalar boundary"
	case WordErrorInvalidUnit:
		return "unknown selection unit"
	case WordErrorInvalidCommand:
		return "unknown word command"
	default:
		return "unknown word selection error"
	}
}

// EnclosingByteRange returns the word-like, sentence, or line paragraph range
// enclosing a byte position. Containment is half-open; an offset exactly at a
// unit end falls back to that final unit. Unmatched text returns an empty range
// at the query position.
func EnclosingByteRange(text string, at model.ByteOffset, unit SelectionUnit) (model.ByteRange, error) {
	if !model.ValidDocumentText(text) {
		return model.ByteRange{}, &WordError{Kind: WordErrorInvalidText}
	}
	index := int(at)
	if index < 0 || index > len(text) {
		return model.ByteRange{}, &WordError{Kind: WordErrorInvalidOffset}
	}
	if index < len(text) {
		if !utf8Boundary(text, index) {
			return model.ByteRange{}, &WordError{Kind: WordErrorInvalidOffset}
		}
	}
	if unit == SelectionParagraph {
		start := 0
		if newline := lastIndexByte(text[:index], '\n'); newline >= 0 {
			start = newline + 1
		}
		end := len(text)
		if newline := indexByte(text[index:], '\n'); newline >= 0 {
			end = index + newline + 1
		}
		return model.NewByteRange(model.ByteOffset(start), model.ByteOffset(end)), nil
	}
	var ranges []model.ByteRange
	var ok bool
	switch unit {
	case SelectionWord:
		ranges, ok = WordLikeRanges(text)
	case SelectionSentence:
		ranges, ok = SentenceRanges(text)
	default:
		return model.ByteRange{}, &WordError{Kind: WordErrorInvalidUnit}
	}
	if !ok {
		return model.ByteRange{}, &WordError{Kind: WordErrorInvalidText}
	}
	for _, candidate := range ranges {
		if candidate.Start <= at && at < candidate.End {
			return candidate, nil
		}
	}
	for index := len(ranges) - 1; index >= 0; index-- {
		if ranges[index].End == at {
			return ranges[index], nil
		}
	}
	return model.NewByteRange(at, at), nil
}

// EnclosingUTF16Range is the scalar-checked UTF-16 form of
// EnclosingByteRange.
func EnclosingUTF16Range(text string, at model.UTF16Offset, unit SelectionUnit) (model.UTF16Range, error) {
	byte, ok := model.UTF16OffsetToByte(text, at)
	if !ok {
		return model.UTF16Range{}, &WordError{Kind: WordErrorInvalidOffset}
	}
	rng, err := EnclosingByteRange(text, byte, unit)
	if err != nil {
		return model.UTF16Range{}, err
	}
	converted, ok := model.ByteRangeToUTF16(text, rng)
	if !ok {
		return model.UTF16Range{}, &WordError{Kind: WordErrorInvalidOffset}
	}
	return converted, nil
}

// PlanWordCommand reproduces Shoreward's previous/next word movement,
// extension, deletion, and unit-selection rules as data.
func PlanWordCommand(text string, selection WordSelection, command WordCommand) (WordCommandPlan, error) {
	if !model.ValidDocumentText(text) {
		return WordCommandPlan{}, &WordError{Kind: WordErrorInvalidText}
	}
	if !model.UTF16OffsetIsBoundary(text, selection.Anchor) ||
		!model.UTF16OffsetIsBoundary(text, selection.Head) {
		return WordCommandPlan{}, &WordError{Kind: WordErrorInvalidOffset}
	}
	if !command.Valid() {
		return WordCommandPlan{}, &WordError{Kind: WordErrorInvalidCommand}
	}
	if unit, ok := command.Unit(); ok {
		enclosing, err := PlanSelectionUnit(text, selection.Head, unit)
		if err != nil {
			return WordCommandPlan{}, err
		}
		return WordCommandPlan{
			Command:    command,
			Anchor:     enclosing.Start,
			Head:       enclosing.End,
			SelectUnit: true,
			Unit:       unit,
		}, nil
	}
	segments, ok := WordRangesUTF16(text)
	if !ok {
		return WordCommandPlan{}, &WordError{Kind: WordErrorInvalidText}
	}
	ranges := make([]model.UTF16Range, 0, len(segments))
	for _, segment := range segments {
		if segment.WordLike {
			ranges = append(ranges, segment.Range)
		}
	}
	head := selection.Head
	offset := model.UTF16Offset(0)
	previous := command == WordPrevious || command == WordExtendPrevious || command == WordDeletePrevious
	if previous {
		offset = 0
		for index := len(ranges) - 1; index >= 0; index-- {
			if ranges[index].Start < head {
				offset = ranges[index].Start
				break
			}
		}
	} else {
		offset = model.UTF16Offset(model.StringLenUTF16(text))
		for _, current := range ranges {
			if current.End > head {
				offset = current.End
				break
			}
		}
	}
	if command == WordDeletePrevious || command == WordDeleteNext {
		deleteRange := selection.Normalized()
		if selection.Collapsed() {
			deleteRange = model.NewUTF16Range(min(offset, head), max(offset, head))
		}
		return WordCommandPlan{
			Command:  command,
			Anchor:   selection.Anchor,
			Head:     head,
			Delete:   true,
			DeleteAt: deleteRange,
		}, nil
	}
	anchor := offset
	if command == WordExtendPrevious || command == WordExtendNext {
		anchor = selection.Anchor
	}
	return WordCommandPlan{
		Command: command,
		Anchor:  anchor,
		Head:    offset,
	}, nil
}

// PlanSelectionUnit returns the enclosing unit as a forward semantic
// selection. This matches Shoreward's select_unit boundary: it offers the range
// to the general selection API but does not preserve prior direction.
func PlanSelectionUnit(
	text string,
	head model.UTF16Offset,
	unit SelectionUnit,
) (model.UTF16Range, error) {
	return EnclosingUTF16Range(text, head, unit)
}

// Unit returns the selection unit represented by a selection command.
func (c WordCommand) Unit() (SelectionUnit, bool) {
	switch c {
	case SelectWord:
		return SelectionWord, true
	case SelectSentence:
		return SelectionSentence, true
	case SelectParagraph:
		return SelectionParagraph, true
	default:
		return 0, false
	}
}

// EnclosingUnits returns every unit in text, useful for tests and decoration.
func EnclosingUnits(text string, unit SelectionUnit) ([]model.ByteRange, error) {
	switch unit {
	case SelectionWord:
		ranges, ok := WordLikeRanges(text)
		if !ok {
			return nil, &WordError{Kind: WordErrorInvalidText}
		}
		return ranges, nil
	case SelectionSentence:
		ranges, ok := SentenceRanges(text)
		if !ok {
			return nil, &WordError{Kind: WordErrorInvalidText}
		}
		return ranges, nil
	case SelectionParagraph:
		result := make([]model.ByteRange, 0)
		for start := 0; start <= len(text); {
			end := len(text)
			if newline := indexByte(text[start:], '\n'); newline >= 0 {
				end = start + newline + 1
			}
			result = append(result, model.NewByteRange(model.ByteOffset(start), model.ByteOffset(end)))
			if end == len(text) {
				break
			}
			start = end
		}
		return result, nil
	default:
		return nil, &WordError{Kind: WordErrorInvalidUnit}
	}
}

func lastIndexByte(text string, value byte) int {
	return strings.LastIndexByte(text, value)
}

func indexByte(text string, value byte) int {
	return strings.IndexByte(text, value)
}

func utf8Boundary(text string, at int) bool {
	return at == len(text) || at >= 0 && at < len(text) && text[at]&0xc0 != 0x80
}

// Valid reports whether c is one of the declared word commands.
func (c WordCommand) Valid() bool {
	for _, command := range AllWordCommands {
		if c == command {
			return true
		}
	}
	return false
}
