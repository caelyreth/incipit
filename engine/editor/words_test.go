package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func u16(value int) model.UTF16Offset {
	return model.UTF16Offset(value)
}

func TestEnclosingByteRangeMatchesWordSentenceAndParagraphRules(t *testing.T) {
	t.Parallel()
	text := "Hello world. 中文测试！\nNext line."
	tests := []struct {
		at       model.ByteOffset
		unit     SelectionUnit
		expected model.ByteRange
	}{
		{7, SelectionWord, ranges(6, 11)[0]},
		{11, SelectionWord, ranges(6, 11)[0]},
		{12, SelectionWord, ranges(12, 12)[0]},
		{7, SelectionSentence, ranges(0, 13)[0]},
		{13, SelectionSentence, ranges(13, 29)[0]},
		{model.ByteOffset(len(text)), SelectionSentence, ranges(29, len(text))[0]},
		{7, SelectionParagraph, ranges(0, 29)[0]},
		{model.ByteOffset(len(text)), SelectionParagraph, ranges(29, len(text))[0]},
	}
	for _, test := range tests {
		got, err := EnclosingByteRange(text, test.at, test.unit)
		if err != nil || got != test.expected {
			t.Errorf("%q at %d unit %d = %#v, want %#v, err=%v", text, test.at, test.unit, got, test.expected, err)
		}
	}
	if _, err := EnclosingByteRange(text, 2, SelectionUnit(99)); err == nil {
		t.Fatal("unknown unit accepted")
	}
	if _, err := EnclosingByteRange("e\u0301", 2, SelectionWord); err == nil {
		t.Fatal("continuation-byte query accepted")
	}
}

func TestEnclosingUTF16RangeRejectsSurrogateInteriors(t *testing.T) {
	t.Parallel()
	if _, err := EnclosingUTF16Range("a🦀b", 2, SelectionWord); err == nil {
		t.Fatal("surrogate-interior query accepted")
	}
	got, err := EnclosingUTF16Range("Hello world.", 7, SelectionWord)
	if err != nil || got != model.NewUTF16Range(6, 11) {
		t.Fatalf("word range = %#v, err=%v", got, err)
	}
}

func TestPlanSelectionUnitAlwaysReturnsForwardRange(t *testing.T) {
	t.Parallel()
	got, err := PlanSelectionUnit("Hello world.", 7, SelectionWord)
	if err != nil || got != model.NewUTF16Range(6, 11) {
		t.Fatalf("unit range = %#v, err=%v", got, err)
	}
}

func TestPlanWordCommandMovementExtensionAndDeletion(t *testing.T) {
	t.Parallel()
	text := "Hello world. 中文测试！\nNext line."
	tests := []struct {
		name      string
		selection WordSelection
		command   WordCommand
		expected  WordCommandPlan
	}{
		{
			"previous",
			WordSelection{Anchor: 7, Head: 7},
			WordPrevious,
			WordCommandPlan{Command: WordPrevious, Anchor: 6, Head: 6},
		},
		{
			"next",
			WordSelection{Anchor: 6, Head: 6},
			WordNext,
			WordCommandPlan{Command: WordNext, Anchor: 11, Head: 11},
		},
		{
			"next skips punctuation to CJK",
			WordSelection{Anchor: 11, Head: 11},
			WordNext,
			WordCommandPlan{Command: WordNext, Anchor: 14, Head: 14},
		},
		{
			"extend previous preserves anchor",
			WordSelection{Anchor: 10, Head: 10},
			WordExtendPrevious,
			WordCommandPlan{Command: WordExtendPrevious, Anchor: 10, Head: 6},
		},
		{
			"delete previous",
			WordSelection{Anchor: 11, Head: 11},
			WordDeletePrevious,
			WordCommandPlan{
				Command:  WordDeletePrevious,
				Anchor:   11,
				Head:     11,
				Delete:   true,
				DeleteAt: model.NewUTF16Range(6, 11),
			},
		},
		{
			"delete next",
			WordSelection{Anchor: 6, Head: 6},
			WordDeleteNext,
			WordCommandPlan{
				Command:  WordDeleteNext,
				Anchor:   6,
				Head:     6,
				Delete:   true,
				DeleteAt: model.NewUTF16Range(6, 11),
			},
		},
		{
			"delete selection",
			WordSelection{Anchor: 12, Head: 15},
			WordDeletePrevious,
			WordCommandPlan{
				Command:  WordDeletePrevious,
				Anchor:   12,
				Head:     15,
				Delete:   true,
				DeleteAt: model.NewUTF16Range(12, 15),
			},
		},
		{
			"select word",
			WordSelection{Anchor: 7, Head: 7},
			SelectWord,
			WordCommandPlan{
				Command:    SelectWord,
				Anchor:     6,
				Head:       11,
				SelectUnit: true,
				Unit:       SelectionWord,
			},
		},
		{
			"select sentence trailing whitespace",
			WordSelection{Anchor: 13, Head: 13},
			SelectSentence,
			WordCommandPlan{
				Command:    SelectSentence,
				Anchor:     13,
				Head:       19,
				SelectUnit: true,
				Unit:       SelectionSentence,
			},
		},
	}
	for _, test := range tests {
		got, err := PlanWordCommand(text, test.selection, test.command)
		if err != nil || got != test.expected {
			t.Errorf("%s = %#v, want %#v, err=%v", test.name, got, test.expected, err)
		}
	}
}

func TestPlanWordCommandFallbackBoundsAndUnicodeEmoji(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		text     string
		head     model.UTF16Offset
		command  WordCommand
		expected model.UTF16Offset
	}{
		{"one", 3, WordNext, 3},
		{"one", 0, WordPrevious, 0},
		{"🦀 word", 2, WordNext, 7},
		{"word 🦀", 5, WordPrevious, 0},
		{"e\u0301x", 2, WordNext, 3},
	} {
		plan, err := PlanWordCommand(test.text, WordSelection{Anchor: test.head, Head: test.head}, test.command)
		if err != nil || plan.Head != test.expected {
			t.Errorf("%q head=%d command=%d => %#v, want %d, err=%v", test.text, test.head, test.command, plan, test.expected, err)
		}
	}
	if _, err := PlanWordCommand("x", WordSelection{Anchor: 0, Head: 0}, WordCommand(99)); err == nil {
		t.Fatal("unknown command accepted")
	}
}

func TestPlanWordCommandUsesICUThaiBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		head     model.UTF16Offset
		command  WordCommand
		expected model.UTF16Offset
	}{
		{0, WordNext, 4},
		{4, WordNext, 7},
		{7, WordPrevious, 4},
	} {
		plan, err := PlanWordCommand(
			"ภาษาไทย",
			WordSelection{Anchor: test.head, Head: test.head},
			test.command,
		)
		if err != nil || plan.Head != test.expected {
			t.Errorf(
				"head=%d command=%d => %#v, want %d, err=%v",
				test.head,
				test.command,
				plan,
				test.expected,
				err,
			)
		}
	}
}

func TestEnclosingUnitsAreDeterministicAndParagraphsKeepNewlines(t *testing.T) {
	t.Parallel()
	text := "one two\n\nthree"
	words, err := EnclosingUnits(text, SelectionWord)
	if err != nil || !slices.Equal(words, ranges(0, 3, 4, 7, 9, 14)) {
		t.Fatalf("words=%#v err=%v", words, err)
	}
	paragraphs, err := EnclosingUnits(text, SelectionParagraph)
	if err != nil || !slices.Equal(paragraphs, ranges(0, 8, 8, 9, 9, 14)) {
		t.Fatalf("paragraphs=%#v err=%v", paragraphs, err)
	}
}
