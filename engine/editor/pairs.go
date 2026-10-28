package editor

import (
	"slices"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// PairRule identifies one independent pair policy rule.
type PairRule uint8

const (
	PairOpen PairRule = iota + 1
	PairOvertype
	PairMateEdit
	PairCloseQuote
)

// String returns the stable rule name.
func (r PairRule) String() string {
	switch r {
	case PairOpen:
		return "open"
	case PairOvertype:
		return "overtype"
	case PairMateEdit:
		return "mate-edit"
	case PairCloseQuote:
		return "close-quote"
	default:
		return "unknown"
	}
}

// OriginAllowed mirrors the rule's trigger gate. Opening, overtyping, and
// contextual closing are typing-only. Mate replacement is also semantic.
func (r PairRule) OriginAllowed(origin kernel.EventOrigin) bool {
	switch r {
	case PairMateEdit:
		return origin == kernel.EventOriginTyping || origin == kernel.EventOriginSemantic
	default:
		return origin == kernel.EventOriginTyping
	}
}

// PairSettings are the independent defaults owned by the pair policy.
type PairSettings struct {
	Quotes           bool
	Overtype         bool
	EditMates        bool
	ContextualQuotes bool
}

// DefaultPairSettings enables every pair behavior.
func DefaultPairSettings() PairSettings {
	return PairSettings{Quotes: true, Overtype: true, EditMates: true, ContextualQuotes: true}
}

// SettingEnabled reports whether one rule can run under these settings.
func (s PairSettings) SettingEnabled(rule PairRule) bool {
	switch rule {
	case PairOpen:
		return true
	case PairOvertype:
		return s.Overtype
	case PairMateEdit:
		return s.EditMates
	case PairCloseQuote:
		return s.ContextualQuotes
	default:
		return false
	}
}

// PairSpan identifies one discovered opener/closer in UTF-16 coordinates.
type PairSpan struct {
	Open   model.UTF16Offset
	Close  model.UTF16Offset
	Opener string
	Closer string
}

// PairInput is one proposed text replacement at a semantic UTF-16 range.
type PairInput struct {
	Range       model.UTF16Range
	Replacement string
	Origin      kernel.EventOrigin
}

// PairPlan is a pure pair policy result. Edits are ordered for reverse
// application; Head is the semantic caret before correction observers run.
type PairPlan struct {
	Handled bool
	Rule    PairRule
	Edits   CorrectionBatch
	Head    model.UTF16Offset
}

type pairSpec struct {
	opener string
	closer string
}

var pairSpecs = [...]pairSpec{
	{"(", ")"},
	{"[", "]"},
	{"{", "}"},
	{"（", "）"},
	{"［", "］"},
	{"｛", "｝"},
	{"“", "”"},
	{"‘", "’"},
	{"\"", "\""},
	{"'", "'"},
}

type pairFrame struct {
	offset model.UTF16Offset
	spec   pairSpec
}

// DiscoverPairs scans semantic text with the same stack rules as Shoreward.
// Brackets survive newlines; quote frames are discarded at a newline.
// Apostrophes inside words are skipped when they are not closing an open
// single quote.
func DiscoverPairs(text string) ([]PairSpan, bool) {
	if !model.ValidDocumentText(text) {
		return nil, false
	}
	runes := []rune(text)
	stack := make([]pairFrame, 0)
	pairs := make([]PairSpan, 0)
	offset := model.UTF16Offset(0)
	var previous rune
	hasPrevious := false
	for index, ch := range runes {
		value := string(ch)
		if ch == '\n' {
			stack = slices.DeleteFunc(stack, func(frame pairFrame) bool {
				return frame.spec.opener == "\"" || frame.spec.opener == "'" ||
					frame.spec.opener == "“" || frame.spec.opener == "‘"
			})
		}
		openSingle := slices.ContainsFunc(stack, func(frame pairFrame) bool {
			return frame.spec.opener == "'" || frame.spec.opener == "‘"
		})
		nextAlphanumeric := index+1 < len(runes) && isAlphanumeric(runes[index+1])
		apostrophe := ch == '\'' && hasPrevious && isAlphanumeric(previous) &&
			(nextAlphanumeric || !openSingle)
		if apostrophe {
			offset++
			previous, hasPrevious = ch, true
			continue
		}
		matched := -1
		for at := len(stack) - 1; at >= 0; at-- {
			if stack[at].spec.closer == value {
				matched = at
				break
			}
		}
		if matched >= 0 {
			frame := stack[matched]
			stack = stack[:matched]
			pairs = append(pairs, PairSpan{
				Open:   frame.offset,
				Close:  offset,
				Opener: frame.spec.opener,
				Closer: frame.spec.closer,
			})
		} else if spec, ok := openerSpec(value); ok {
			stack = append(stack, pairFrame{offset: offset, spec: spec})
		}
		offset += model.UTF16Offset(model.RuneUTF16Len(ch))
		previous, hasPrevious = ch, true
	}
	return pairs, true
}

// PlanPair plans one pair rule without mutating a document.
func PlanPair(
	rule PairRule,
	text string,
	input PairInput,
	settings PairSettings,
) (PairPlan, error) {
	declined := PairPlan{Rule: rule}
	if !model.ValidDocumentText(text) || !model.ValidDocumentText(input.Replacement) {
		return declined, &CorrectionError{Kind: CorrectionErrorInvalidText}
	}
	if input.Range.Start < 0 || input.Range.End < input.Range.Start ||
		!model.UTF16OffsetIsBoundary(text, input.Range.Start) ||
		!model.UTF16OffsetIsBoundary(text, input.Range.End) {
		return declined, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	if !rule.OriginAllowed(input.Origin) || !settings.SettingEnabled(rule) {
		return declined, nil
	}
	if isQuote(input.Replacement) && !settings.Quotes {
		return declined, nil
	}
	switch rule {
	case PairOvertype:
		return planPairOvertype(text, input)
	case PairCloseQuote:
		return planPairCloseQuote(text, input)
	case PairMateEdit:
		return planPairMateEdit(text, input)
	case PairOpen:
		return planPairOpen(text, input)
	default:
		return declined, nil
	}
}

func planPairOvertype(text string, input PairInput) (PairPlan, error) {
	plan := PairPlan{Rule: PairOvertype}
	if !input.Range.Empty() || !isClosing(input.Replacement) {
		return plan, nil
	}
	pairs, _ := DiscoverPairs(text)
	for _, pair := range pairs {
		if pair.Close == input.Range.Start &&
			(pair.Closer == input.Replacement ||
				pair.Closer == "”" && input.Replacement == "\"" ||
				pair.Closer == "’" && input.Replacement == "'") {
			plan.Handled = true
			plan.Head = pair.Close + model.UTF16Offset(model.StringLenUTF16(pair.Closer))
			return plan, nil
		}
	}
	return plan, nil
}

func planPairCloseQuote(text string, input PairInput) (PairPlan, error) {
	plan := PairPlan{Rule: PairCloseQuote}
	if !input.Range.Empty() || input.Replacement != "\"" && input.Replacement != "'" {
		return plan, nil
	}
	open, close := '“', '”'
	if input.Replacement == "'" {
		open, close = '‘', '’'
	}
	prefix, ok := utf16Prefix(text, input.Range.Start)
	if !ok {
		return plan, &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	if previous, size := lastRune(prefix); size > 0 && input.Replacement == "'" && isAlphanumeric(previous) {
		return plan, nil
	}
	if quoteDepth(prefix, open, close) <= 0 {
		return plan, nil
	}
	batch, err := NewCorrectionBatch([]Correction{{
		Range:       input.Range,
		Replacement: string(close),
	}})
	if err != nil {
		return plan, err
	}
	plan.Handled = true
	plan.Edits = batch
	plan.Head = input.Range.Start + 1
	return plan, nil
}

func planPairMateEdit(text string, input PairInput) (PairPlan, error) {
	plan := PairPlan{Rule: PairMateEdit}
	if input.Range.Empty() {
		return plan, nil
	}
	pairs, _ := DiscoverPairs(text)
	touched := make([]PairSpan, 0)
	for _, pair := range pairs {
		if contains(pair.Open, input.Range) != contains(pair.Close, input.Range) {
			touched = append(touched, pair)
		}
	}
	if len(touched) == 0 {
		return plan, nil
	}
	if input.Range.Len() == 1 && len(touched) == 1 {
		pair := touched[0]
		opening := input.Range.Start == pair.Open
		spelling := input.Replacement
		if opening {
			spelling = map[string]string{"\"": "“", "'": "‘"}[spelling]
			if spelling == "" {
				spelling = input.Replacement
			}
		} else {
			spelling = map[string]string{"\"": "”", "'": "’"}[spelling]
			if spelling == "" {
				spelling = input.Replacement
			}
		}
		if spec, ok := specForSpelling(spelling, opening); ok {
			batch, err := NewCorrectionBatch([]Correction{
				{
					Range:       model.NewUTF16Range(pair.Open, pair.Open+1),
					Replacement: spec.opener,
				},
				{
					Range:       model.NewUTF16Range(pair.Close, pair.Close+1),
					Replacement: spec.closer,
				},
			})
			if err != nil {
				return plan, err
			}
			plan.Handled = true
			plan.Edits = batch
			plan.Head = input.Range.Start + model.UTF16Offset(model.StringLenUTF16(spelling))
			return plan, nil
		}
	}
	mates := make([]model.UTF16Offset, 0, len(touched))
	for _, pair := range touched {
		if contains(pair.Open, input.Range) {
			mates = append(mates, pair.Close)
		} else {
			mates = append(mates, pair.Open)
		}
	}
	slices.Sort(mates)
	mates = slices.Compact(mates)
	removedBefore := 0
	for _, mate := range mates {
		if mate < input.Range.Start {
			removedBefore++
		}
	}
	head := input.Range.Start - model.UTF16Offset(removedBefore) +
		model.UTF16Offset(model.StringLenUTF16(input.Replacement))
	edits := make([]Correction, 0, len(mates)+1)
	for _, mate := range mates {
		edits = append(edits, Correction{
			Range:       model.NewUTF16Range(mate, mate+1),
			Replacement: "",
		})
	}
	edits = append(edits, Correction{Range: input.Range, Replacement: input.Replacement})
	batch, err := NewCorrectionBatch(edits)
	if err != nil {
		return plan, err
	}
	plan.Handled = true
	plan.Edits = batch
	plan.Head = max(0, head)
	return plan, nil
}

func planPairOpen(text string, input PairInput) (PairPlan, error) {
	plan := PairPlan{Rule: PairOpen}
	spec, ok := openingSpecification(input.Replacement)
	if !ok {
		return plan, nil
	}
	if input.Replacement == "'" && input.Range.Empty() {
		prefix, ok := utf16Prefix(text, input.Range.Start)
		if !ok {
			return plan, &CorrectionError{Kind: CorrectionErrorInvalidRange}
		}
		previous, ok := PreviousGrapheme(prefix, model.UTF16Offset(model.StringLenUTF16(prefix)))
		if ok {
			byteRange, ok := model.UTF16RangeToByte(prefix, previous)
			if !ok {
				return plan, &CorrectionError{Kind: CorrectionErrorInvalidRange}
			}
			grapheme := prefix[byteRange.Start:byteRange.End]
			for _, r := range grapheme {
				if isAlphanumeric(r) {
					return plan, nil
				}
			}
		}
	}
	batch, err := NewCorrectionBatch([]Correction{
		{
			Range:       model.EmptyUTF16Range(input.Range.End),
			Replacement: spec.closer,
		},
		{
			Range:       model.EmptyUTF16Range(input.Range.Start),
			Replacement: spec.opener,
		},
	})
	if err != nil {
		return plan, err
	}
	plan.Handled = true
	plan.Edits = batch
	plan.Head = input.Range.Start + model.UTF16Offset(model.StringLenUTF16(spec.opener))
	return plan, nil
}

// ShortestPairContaining chooses the narrowest pair containing a collapsed
// caret. The caret may sit immediately before the closer.
func ShortestPairContaining(pairs []PairSpan, head model.UTF16Offset) (PairSpan, bool) {
	result := PairSpan{}
	found := false
	for _, pair := range pairs {
		if pair.Open < head && head <= pair.Close && (!found || pair.Close-pair.Open < result.Close-result.Open) {
			result, found = pair, true
		}
	}
	return result, found
}

func openerSpec(value string) (pairSpec, bool) {
	for _, spec := range pairSpecs {
		if spec.opener == value {
			return spec, true
		}
	}
	return pairSpec{}, false
}

func openingSpecification(value string) (pairSpec, bool) {
	switch value {
	case "\"", "“":
		return pairSpec{opener: "“", closer: "”"}, true
	case "'", "‘":
		return pairSpec{opener: "‘", closer: "’"}, true
	default:
		return openerSpec(value)
	}
}

func specForSpelling(spelling string, opening bool) (pairSpec, bool) {
	for _, spec := range pairSpecs {
		if opening && spec.opener == spelling || !opening && spec.closer == spelling {
			return spec, true
		}
	}
	return pairSpec{}, false
}

func isClosing(value string) bool {
	for _, spec := range pairSpecs {
		if spec.closer == value {
			return true
		}
	}
	return value == "\"" || value == "'"
}

func isQuote(value string) bool {
	switch value {
	case "\"", "'", "“", "”", "‘", "’":
		return true
	default:
		return false
	}
}

func contains(offset model.UTF16Offset, textRange model.UTF16Range) bool {
	return textRange.Start <= offset && offset < textRange.End
}

func utf16Prefix(text string, end model.UTF16Offset) (string, bool) {
	byteRange, ok := model.UTF16RangeToByte(text, model.NewUTF16Range(0, end))
	if !ok {
		return "", false
	}
	return text[:byteRange.End], true
}

func quoteDepth(prefix string, open, close rune) int {
	runes := []rune(prefix)
	if lineStart := slices.IndexFunc(runes, func(r rune) bool { return r == '\n' }); lineStart >= 0 {
		runes = runes[lineStart+1:]
	}
	balance := 0
	for index, ch := range runes {
		if ch == '\'' && open == '‘' {
			var previous, next rune
			if index > 0 {
				previous = runes[index-1]
			}
			if index+1 < len(runes) {
				next = runes[index+1]
			}
			if isAlphanumeric(previous) && isAlphanumeric(next) {
				continue
			}
		}
		if ch == close {
			balance--
		} else if ch == open || open == '‘' && ch == '\'' {
			balance++
		}
	}
	return balance
}
