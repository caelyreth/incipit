package editor

import (
	"unicode"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/text/rope"
)

// CJKSettings control CJK refinement independently. Both default to true when
// the policy itself is enabled.
type CJKSettings struct {
	Punctuation bool
	Spacing     bool
}

// CJKPlan is one line-local refinement plan. Lines are byte ranges in the
// materialized current text; corrections are UTF-16 offsets starting at Base.
type CJKPlan struct {
	Line        model.ByteRange
	Base        model.UTF16Offset
	Corrections CorrectionBatch
}

// PlanCJK refines every line touched by an inserted UTF-16 range. The input
// range is clamped to the text and must identify scalar boundaries. CJK
// punctuation is widened only on CJK-containing lines; Latin-only lines narrow
// it back. Decimal separators and URL "://" tokens remain ASCII.
func PlanCJK(text string, inserted model.UTF16Range, settings CJKSettings) ([]CJKPlan, error) {
	if !model.ValidDocumentText(text) || inserted.Start < 0 || inserted.End < inserted.Start {
		return nil, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	length := model.StringLenUTF16(text)
	start := min(int(inserted.Start), length)
	end := min(int(inserted.End), length)
	startByte, ok := model.UTF16OffsetToByte(text, model.UTF16Offset(start))
	if !ok {
		return nil, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	endByte, ok := model.UTF16OffsetToByte(text, model.UTF16Offset(end))
	if !ok {
		return nil, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	firstLine := lineOfByte(text, int(startByte))
	lastLine := lineOfByte(text, int(endByte))
	plans := make([]CJKPlan, 0, lastLine-firstLine+1)
	for line := firstLine; line <= lastLine; line++ {
		from, to := lineByteRange(text, line)
		prefixUnits := model.StringLenUTF16(text[:from])
		parts := graphemeStrings(text[from:to])
		edits := cjkCorrections(parts, model.UTF16Offset(prefixUnits), settings)
		batch, err := NewCorrectionBatch(edits)
		if err != nil {
			return nil, err
		}
		plans = append(plans, CJKPlan{
			Line:        model.NewByteRange(model.ByteOffset(from), model.ByteOffset(to)),
			Base:        model.UTF16Offset(prefixUnits),
			Corrections: batch,
		})
	}
	return plans, nil
}

func planCJKTree(tree rope.Tree, inserted model.UTF16Range, settings CJKSettings) ([]CJKPlan, error) {
	if inserted.Start < 0 || inserted.End < inserted.Start {
		return nil, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	length := tree.UTF16Len()
	start := min(int(inserted.Start), length)
	end := min(int(inserted.End), length)
	startByte, ok := tree.CheckedByteOfUTF16(start)
	if !ok {
		return nil, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	endByte, ok := tree.CheckedByteOfUTF16(end)
	if !ok {
		return nil, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	firstLine := tree.LineOf(startByte)
	lastLine := tree.LineOf(endByte)
	plans := make([]CJKPlan, 0, lastLine-firstLine+1)
	for line := firstLine; line <= lastLine; line++ {
		from := tree.LineStart(line)
		to := tree.ByteLen()
		if line+1 < tree.LineCount() {
			to = tree.LineStart(line + 1)
		}
		base := model.UTF16Offset(tree.UTF16UnitsIn(0, from))
		parts := graphemeStrings(tree.Slice(from, to))
		edits := cjkCorrections(parts, base, settings)
		batch, err := NewCorrectionBatch(edits)
		if err != nil {
			return nil, err
		}
		plans = append(plans, CJKPlan{
			Line:        model.NewByteRange(model.ByteOffset(from), model.ByteOffset(to)),
			Base:        base,
			Corrections: batch,
		})
	}
	return plans, nil
}

func cjkCorrections(parts []string, base model.UTF16Offset, settings CJKSettings) []Correction {
	offsets := make([]model.UTF16Offset, 1, len(parts)+1)
	offsets[0] = base
	for _, part := range parts {
		offsets = append(offsets, offsets[len(offsets)-1]+model.UTF16Offset(model.StringLenUTF16(part)))
	}
	hasCJK := slicesAny(parts, containsCJK)
	hasLatin := slicesAny(parts, containsLatin)
	wide := make([]bool, len(parts))
	edits := make([]Correction, 0)

	if settings.Punctuation {
		for at, part := range parts {
			half, full, ok := punctuationForms(part)
			if !ok {
				continue
			}
			left := ""
			if at > 0 {
				left = parts[at-1]
			}
			right := ""
			if at+1 < len(parts) {
				right = parts[at+1]
			}
			token := half == "." && containsLatin(left) && containsLatin(right) ||
				(half == "," || half == ":") && containsDigit(left) && containsDigit(right) ||
				half == ":" && containsLatin(left) && right == "/" &&
					at+2 < len(parts) && parts[at+2] == "/"
			replacement := ""
			switch {
			case hasCJK && !token:
				wide[at] = true
				replacement = full
			case hasCJK || hasLatin:
				replacement = half
			default:
				continue
			}
			if part != replacement {
				edits = append(edits, Correction{
					Range:       model.NewUTF16Range(offsets[at], offsets[at+1]),
					Replacement: replacement,
				})
			}
		}
		for at := 0; at < len(parts); at++ {
			if parts[at] != " " {
				continue
			}
			start := at
			for at+1 < len(parts) && parts[at+1] == " " {
				at++
			}
			if start > 0 && wide[start-1] || at+1 < len(parts) && wide[at+1] {
				edits = append(edits, Correction{
					Range:       model.NewUTF16Range(offsets[start], offsets[at+1]),
					Replacement: "",
				})
			}
		}
	}
	if settings.Spacing {
		for at := 0; at+1 < len(parts); at++ {
			if containsCJK(parts[at]) && containsLatin(parts[at+1]) ||
				containsLatin(parts[at]) && containsCJK(parts[at+1]) {
				edits = append(edits, Correction{
					Range:       model.NewUTF16Range(offsets[at+1], offsets[at+1]),
					Replacement: " ",
				})
			}
		}
	}
	return edits
}

func containsCJK(text string) bool {
	for _, r := range text {
		if r >= '\u3400' && r <= '\u9fff' ||
			r >= '\u3040' && r <= '\u30ff' ||
			r >= '\uac00' && r <= '\ud7af' {
			return true
		}
	}
	return false
}

func containsLatin(text string) bool {
	for _, r := range text {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return true
		}
	}
	return false
}

func containsDigit(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func punctuationForms(text string) (half, full string, ok bool) {
	switch text {
	case ",", "，":
		return ",", "，", true
	case ".", "．", "。":
		return ".", "。", true
	case ":", "：":
		return ":", "：", true
	case ";", "；":
		return ";", "；", true
	case "!", "！":
		return "!", "！", true
	case "?", "？":
		return "?", "？", true
	default:
		return "", "", false
	}
}

func graphemeStrings(text string) []string {
	ranges, ok := GraphemeRanges(text)
	if !ok {
		return nil
	}
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		parts = append(parts, text[r.Start:r.End])
	}
	return parts
}

func slicesAny(values []string, predicate func(string) bool) bool {
	for _, value := range values {
		if predicate(value) {
			return true
		}
	}
	return false
}

func lineByteRange(text string, line int) (int, int) {
	from := lineStartByte(text, line)
	to := len(text)
	if next := lineStartByte(text, line+1); next != len(text) || line < lineCount(text)-1 {
		to = max(from, next)
	}
	return from, to
}
