package editor

import (
	"strings"
	"unicode"

	"github.com/caelyreth/neverland/engine/model"
)

// TypographySettings select independent convergence families.
type TypographySettings struct {
	Symbols        bool
	Punctuation    bool
	QuotePlacement bool
}

// TypographyTrigger reports whether a typed replacement can initiate
// typography. Paste, IME, semantic input, and multi-scalar typing do not.
func TypographyTrigger(text string) bool {
	return len(text) == 1 && strings.ContainsRune(".-',;:!?", []rune(text)[0])
}

// PlanTypography computes one convergence pass for the caret's line.
// Returned ranges are absolute UTF-16 coordinates in text.
func PlanTypography(text string, head model.UTF16Offset, settings TypographySettings) (CorrectionBatch, bool) {
	if !model.ValidDocumentText(text) || head < 0 {
		return CorrectionBatch{}, false
	}
	headByte, ok := model.UTF16OffsetToByte(text, head)
	if !ok {
		return CorrectionBatch{}, false
	}
	line := lineOfByte(text, int(headByte))
	startByte, endByte := lineByteRange(text, line)
	base := model.UTF16Offset(model.StringLenUTF16(text[:startByte]))
	source := text[startByte:endByte]
	localHead := int(head - base)
	edits, changed, ok := typographyCorrections(source, localHead, settings)
	if !ok {
		return CorrectionBatch{}, false
	}
	for index := range edits {
		edits[index].Range.Start += base
		edits[index].Range.End += base
	}
	if !changed {
		return CorrectionBatch{}, true
	}
	batch, err := NewCorrectionBatch(edits)
	return batch, err == nil
}

// ConvergeTypography repeatedly plans line corrections, updating the head
// after each accepted batch. It mirrors Neverland's three-pass convergence cap.
func ConvergeTypography(
	text string,
	head model.UTF16Offset,
	settings TypographySettings,
) ([]CorrectionBatch, model.UTF16Offset, bool) {
	batches := make([]CorrectionBatch, 0, 3)
	for range 3 {
		batch, ok := PlanTypography(text, head, settings)
		if !ok {
			return nil, head, false
		}
		if batch.Empty() {
			return batches, head, true
		}
		batches = append(batches, batch)
		nextText, nextHead, ok := ApplyCorrections(text, head, batch)
		if !ok {
			return nil, head, false
		}
		text, head = nextText, nextHead
	}
	return batches, head, true
}

// ApplyCorrections applies a batch in reverse range order and rebases one
// right-affinity head through each edit. It is a pure oracle for session
// integration and correction tests.
func ApplyCorrections(
	text string,
	head model.UTF16Offset,
	batch CorrectionBatch,
) (string, model.UTF16Offset, bool) {
	if !model.ValidDocumentText(text) {
		return "", 0, false
	}
	result := text
	anchor := model.NewTextAnchor(1, head, model.AffinityRight, 0)
	for _, edit := range batch.Edits() {
		byteRange, ok := model.UTF16RangeToByte(result, edit.Range)
		if !ok {
			return "", 0, false
		}
		result = result[:byteRange.Start] + edit.Replacement + result[byteRange.End:]
		footprint := model.NewTextEditReplacement(1, edit.Range, edit.Replacement)
		anchor, ok = anchor.Rebase(footprint, 0)
		if !ok {
			return "", 0, false
		}
	}
	return result, anchor.OffsetUTF16(), true
}

func typographyCorrections(
	text string,
	head int,
	settings TypographySettings,
) ([]Correction, bool, bool) {
	byte, ok := model.UTF16OffsetToByte(text, model.UTF16Offset(head))
	if !ok {
		return nil, false, false
	}
	before := text[:byte]
	replace := func(count int, replacement string) []Correction {
		return []Correction{{
			Range:       model.NewUTF16Range(model.UTF16Offset(head-count), model.UTF16Offset(head)),
			Replacement: replacement,
		}}
	}
	if settings.Symbols {
		if strings.HasSuffix(before, "...") {
			return replace(3, "…"), true, true
		}
		if strings.HasSuffix(before, "--") {
			return replace(2, "—"), true, true
		}
		if strings.HasSuffix(before, "'") {
			previous := []rune(before[:len(before)-1])
			if len(previous) > 0 && isAlphanumeric(previous[len(previous)-1]) {
				return replace(1, "’"), true, true
			}
		}
	}
	runes := []rune(before)
	if len(runes) == 0 {
		return nil, false, true
	}
	last := runes[len(runes)-1]
	if settings.Punctuation && strings.ContainsRune(",;:!?", last) {
		prefixRunes := runes[:len(runes)-1]
		spaces := 0
		for index := len(prefixRunes) - 1; index >= 0 && prefixRunes[index] == ' '; index-- {
			spaces++
		}
		if spaces > 0 {
			return replace(spaces+1, string(last)), true, true
		}
		if strings.ContainsRune(",;:", last) && len(prefixRunes) > 0 && prefixRunes[len(prefixRunes)-1] == last {
			return replace(1, ""), true, true
		}
	}
	closing := func(at int, quote rune) bool {
		if quote == '”' || quote == '’' {
			return true
		}
		if quote != '"' && quote != '\'' {
			return false
		}
		segment := text[:at]
		if newline := strings.LastIndexByte(segment, '\n'); newline >= 0 {
			segment = segment[newline+1:]
		}
		count := 0
		for _, r := range segment {
			if r == quote {
				count++
			}
		}
		return count%2 == 1
	}
	if settings.QuotePlacement && (last == ',' || last == '.') {
		fromByte := int(byte) - len(string(last))
		from := head - 1
		for {
			if fromByte <= 0 || from <= 0 {
				break
			}
			quote, size := lastRune(text[:fromByte])
			if size == 0 || fromByte-size < 0 || !closing(fromByte-size, quote) {
				break
			}
			fromByte -= size
			from -= model.RuneUTF16Len(quote)
		}
		if from < head-1 {
			inside, _ := lastRune(text[:fromByte])
			duplicate := inside == last || last == '.' && strings.ContainsRune("!?…", inside)
			edits := replace(1, "")
			if !duplicate {
				edits = append(edits, Correction{
					Range:       model.NewUTF16Range(model.UTF16Offset(from), model.UTF16Offset(from)),
					Replacement: string(last),
				})
			}
			return edits, true, true
		}
	}
	if settings.QuotePlacement && (last == ':' || last == ';') {
		to := head
		for at, quote := range text[byte:] {
			if !closing(int(byte)+at, quote) {
				break
			}
			to += model.RuneUTF16Len(quote)
		}
		if to > head {
			return []Correction{
				{
					Range:       model.NewUTF16Range(model.UTF16Offset(to), model.UTF16Offset(to)),
					Replacement: string(last),
				},
				{
					Range:       model.NewUTF16Range(model.UTF16Offset(head-1), model.UTF16Offset(head)),
					Replacement: "",
				},
			}, true, true
		}
	}
	return nil, false, true
}

func isAlphanumeric(r rune) bool {
	return unicode.IsLetter(r) ||
		unicode.IsNumber(r) ||
		unicode.Is(unicode.Other_Alphabetic, r)
}

func lastRune(text string) (rune, int) {
	if text == "" {
		return 0, 0
	}
	var result rune
	size := 0
	for _, r := range text {
		result, size = r, len(string(r))
	}
	return result, size
}
