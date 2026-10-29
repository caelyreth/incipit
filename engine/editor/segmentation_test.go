package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func ranges(values ...int) []model.ByteRange {
	result := make([]model.ByteRange, 0, len(values)/2)
	for index := 0; index < len(values); index += 2 {
		result = append(result, model.NewByteRange(model.ByteOffset(values[index]), model.ByteOffset(values[index+1])))
	}
	return result
}

func TestWordLikeRangesIncludeUnicodeLettersAndNumbersOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		expected []model.ByteRange
	}{
		{"Hello world. 中文测试！\nNext line.", ranges(0, 5, 6, 11, 13, 19, 19, 25, 29, 33, 34, 38)},
		{"don't dogs'", ranges(0, 5, 6, 10)},
		{"🦀word", ranges(4, 8)},
		{"--word", ranges(2, 6)},
		{"中文test, 123.4 https://x.com", ranges(0, 6, 6, 10, 12, 17, 18, 23, 26, 31)},
		{"a\u0301x 🦀 🇨🇳", ranges(0, 4)},
		{"é_1 ！", ranges(0, 4)},
	}
	for _, test := range tests {
		got, ok := WordLikeRanges(test.text)
		if !ok || !slices.Equal(got, test.expected) {
			t.Errorf("%q words = %#v, want %#v, ok=%v", test.text, got, test.expected, ok)
		}
	}
	if _, ok := WordLikeRanges(string([]byte{0xff})); ok {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestWordSegmentationMatchesPlatformBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		segments []WordSegment
	}{
		{
			"_",
			[]WordSegment{{Range: ranges(0, 1)[0], WordLike: false}},
		},
		{
			"123,456",
			[]WordSegment{{Range: ranges(0, 7)[0], WordLike: true}},
		},
		{
			"a·b",
			[]WordSegment{{Range: ranges(0, 4)[0], WordLike: true}},
		},
		{
			"ภาษาไทย",
			[]WordSegment{
				{Range: ranges(0, 3)[0], WordLike: true},
				{Range: ranges(3, 6)[0], WordLike: true},
				{Range: ranges(6, 9)[0], WordLike: true},
				{Range: ranges(9, 12)[0], WordLike: true},
				{Range: ranges(12, 15)[0], WordLike: true},
				{Range: ranges(15, 18)[0], WordLike: true},
				{Range: ranges(18, 21)[0], WordLike: true},
			},
		},
		{
			"\u0301",
			[]WordSegment{{Range: ranges(0, 2)[0], WordLike: false}},
		},
	}
	for _, test := range tests {
		got, ok := Words(test.text)
		if !ok || !slices.Equal(got, test.segments) {
			t.Errorf("%q segments = %#v, want %#v, ok=%v", test.text, got, test.segments, ok)
		}
	}
}

func TestCJKWordLikeRangesUseDictionaryBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		expected []model.ByteRange
	}{
		{"中文测试", ranges(0, 6, 6, 12)},
		{"中华人民共和国", ranges(0, 6, 6, 12, 12, 21)},
		{"南京市长江大桥", ranges(0, 9, 9, 15, 15, 18, 18, 21)},
		{"日本語の文章です", ranges(0, 9, 9, 12, 12, 18, 18, 24)},
		{"한국어 문장입니다", ranges(0, 9, 10, 25)},
	}
	for _, test := range tests {
		got, ok := WordLikeRanges(test.text)
		if !ok || !slices.Equal(got, test.expected) {
			t.Errorf("%q words = %#v, want %#v, ok=%v", test.text, got, test.expected, ok)
		}
	}
}

func TestDeferredThaiUsesUnicodeFallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		segments []WordSegment
	}{
		{
			"ก่อนภาษาไทยหลัง",
			[]WordSegment{
				{Range: ranges(0, 6)[0], WordLike: true},
				{Range: ranges(6, 9)[0], WordLike: true},
				{Range: ranges(9, 12)[0], WordLike: true},
				{Range: ranges(12, 15)[0], WordLike: true},
				{Range: ranges(15, 18)[0], WordLike: true},
				{Range: ranges(18, 21)[0], WordLike: true},
				{Range: ranges(21, 24)[0], WordLike: true},
				{Range: ranges(24, 27)[0], WordLike: true},
				{Range: ranges(27, 30)[0], WordLike: true},
				{Range: ranges(30, 33)[0], WordLike: true},
				{Range: ranges(33, 36)[0], WordLike: true},
				{Range: ranges(36, 42)[0], WordLike: true},
				{Range: ranges(42, 45)[0], WordLike: true},
			},
		},
		{
			"ทุกสองสัปดาห์",
			[]WordSegment{
				{Range: ranges(0, 6)[0], WordLike: true},
				{Range: ranges(6, 9)[0], WordLike: true},
				{Range: ranges(9, 12)[0], WordLike: true},
				{Range: ranges(12, 15)[0], WordLike: true},
				{Range: ranges(15, 18)[0], WordLike: true},
				{Range: ranges(18, 24)[0], WordLike: true},
				{Range: ranges(24, 27)[0], WordLike: true},
				{Range: ranges(27, 30)[0], WordLike: true},
				{Range: ranges(30, 33)[0], WordLike: true},
				{Range: ranges(33, 39)[0], WordLike: true},
			},
		},
		{
			"ภาษาไทยabc",
			[]WordSegment{
				{Range: ranges(0, 3)[0], WordLike: true},
				{Range: ranges(3, 6)[0], WordLike: true},
				{Range: ranges(6, 9)[0], WordLike: true},
				{Range: ranges(9, 12)[0], WordLike: true},
				{Range: ranges(12, 15)[0], WordLike: true},
				{Range: ranges(15, 18)[0], WordLike: true},
				{Range: ranges(18, 21)[0], WordLike: true},
				{Range: ranges(21, 24)[0], WordLike: true},
			},
		},
	}
	for _, test := range tests {
		got, ok := Words(test.text)
		if !ok || !slices.Equal(got, test.segments) {
			t.Errorf("%q segments = %#v, want %#v, ok=%v", test.text, got, test.segments, ok)
		}
	}
}

func TestWordRangesUTF16UseSegmenterIndexes(t *testing.T) {
	t.Parallel()
	text := "🦀中文 test"
	got, ok := WordRangesUTF16(text)
	expected := []UTF16Segment{
		{Range: model.NewUTF16Range(0, 2), WordLike: false},
		{Range: model.NewUTF16Range(2, 4), WordLike: true},
		{Range: model.NewUTF16Range(4, 5), WordLike: false},
		{Range: model.NewUTF16Range(5, 9), WordLike: true},
	}
	if !ok || !slices.Equal(got, expected) {
		t.Fatalf("segments = %#v, want %#v, ok=%v", got, expected, ok)
	}
}

func TestSentenceRangesMatchUnicodeTrailingSpacePolicy(t *testing.T) {
	t.Parallel()
	text := "Hello world. 中文测试！\nNext line."
	got, ok := SentenceRanges(text)
	if !ok || !slices.Equal(got, ranges(0, 13, 13, 29, 29, 39)) {
		t.Fatalf("sentences = %#v, ok=%v", got, ok)
	}
	got, ok = SentenceRanges("one. Two! Three?")
	if !ok || !slices.Equal(got, ranges(0, 5, 5, 10, 10, 16)) {
		t.Fatalf("sentences = %#v, ok=%v", got, ok)
	}
}

func TestGraphemeHelpersKeepZWJFlagsAndCombiningMarksWhole(t *testing.T) {
	t.Parallel()
	text := "a\u0301👩‍👩‍👧‍👦🇨🇳z"
	expected := []model.UTF16Range{
		model.NewUTF16Range(0, 2),
		model.NewUTF16Range(2, 13),
		model.NewUTF16Range(13, 17),
		model.NewUTF16Range(17, 18),
	}
	got, ok := UTF16Graphemes(text)
	if !ok || !slices.Equal(got, expected) {
		t.Fatalf("graphemes = %#v, want %#v", got, expected)
	}
	for _, offset := range []model.UTF16Offset{1, 3, 6, 10, 14, 16, 17} {
		cluster, ok := GraphemeAt(text, offset)
		if !ok || cluster.Start > offset || offset >= cluster.End {
			t.Errorf("cluster at %d = %#v, ok=%v", offset, cluster, ok)
		}
	}
	if previous, ok := PreviousGrapheme(text, 1); !ok || previous != expected[0] {
		t.Fatalf("previous inside combining cluster = %#v, ok=%v", previous, ok)
	}
	if next, ok := NextGrapheme(text, 14); !ok || next != expected[2] {
		t.Fatalf("next inside flag = %#v, ok=%v", next, ok)
	}
	if _, ok := NextGrapheme(text, 18); ok {
		t.Fatal("next grapheme at end accepted")
	}
}

func TestWordsRejectInvalidUTF8(t *testing.T) {
	t.Parallel()
	if _, ok := Words(string([]byte{0xff})); ok {
		t.Fatal("invalid UTF-8 accepted")
	}
	if _, ok := GraphemeRanges(string([]byte{0xff})); ok {
		t.Fatal("invalid UTF-8 accepted")
	}
}
