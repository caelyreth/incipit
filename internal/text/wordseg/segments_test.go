package wordseg

import (
	"slices"
	"testing"
)

type expectedSegment struct {
	text     string
	start    int
	wordLike bool
}

func collectSegments(text string) []expectedSegment {
	segments := make([]expectedSegment, 0)
	for segment := range Segments(text) {
		segments = append(segments, expectedSegment{
			text:     segment.Text,
			start:    segment.Start,
			wordLike: segment.WordLike,
		})
	}
	return segments
}

func TestSegmentsLatinAndCJK(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want []expectedSegment
	}{
		{
			"Hello, can't stop 3.14 foo_bar.",
			[]expectedSegment{
				{"Hello", 0, true},
				{",", 5, false},
				{" ", 6, false},
				{"can't", 7, true},
				{" ", 12, false},
				{"stop", 13, true},
				{" ", 17, false},
				{"3.14", 18, true},
				{" ", 22, false},
				{"foo_bar", 23, true},
				{".", 30, false},
			},
		},
		{
			"中文测试",
			[]expectedSegment{
				{"中文", 0, true},
				{"测试", 6, true},
			},
		},
		{
			"日本語の文章です",
			[]expectedSegment{
				{"日本語", 0, true},
				{"の", 9, true},
				{"文章", 12, true},
				{"です", 18, true},
			},
		},
		{
			"한국어 문장입니다",
			[]expectedSegment{
				{"한국어", 0, true},
				{" ", 9, false},
				{"문장입니다", 10, true},
			},
		},
		{
			"Latin 中文混合 text.",
			[]expectedSegment{
				{"Latin", 0, true},
				{" ", 5, false},
				{"中文", 6, true},
				{"混合", 12, true},
				{" ", 18, false},
				{"text", 19, true},
				{".", 23, false},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			t.Parallel()
			got := collectSegments(test.text)
			if !slices.Equal(got, test.want) {
				t.Fatalf("segments = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSegmentsUsesUnicodeFallbackForDeferredScripts(t *testing.T) {
	t.Parallel()
	got := collectSegments("ภาษาไทย")
	want := []expectedSegment{
		{"ภ", 0, true},
		{"า", 3, true},
		{"ษ", 6, true},
		{"า", 9, true},
		{"ไ", 12, true},
		{"ท", 15, true},
		{"ย", 18, true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
}

func TestSegmentsByteStartsAndCombiningMarks(t *testing.T) {
	t.Parallel()
	got := collectSegments("🦀中文 e\u0301")
	want := []expectedSegment{
		{"🦀", 0, false},
		{"中文", 4, true},
		{" ", 10, false},
		{"e\u0301", 11, true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
}

func BenchmarkSegments(b *testing.B) {
	text := "Latin words and 中文混合 text. 日本語の文章です."
	for b.Loop() {
		for range Segments(text) {
		}
	}
}

// TestDictionaryIsNeverlands has the dictionary embedded in the format
// cjk.go reads (tools/cjkdict).
func TestDictionaryIsNeverlands(t *testing.T) {
	if !dictionaryValid() {
		t.Fatalf("the dictionary embedded is not Neverland's, version %d: run go tool fetchassets", dictionaryVersion)
	}
}
