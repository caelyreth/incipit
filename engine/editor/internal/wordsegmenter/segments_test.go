package wordsegmenter

import (
	"slices"
	"testing"
)

type expectedSegment struct {
	text     string
	index    int
	wordLike bool
}

func collectSegments(text string) []expectedSegment {
	segments := make([]expectedSegment, 0)
	for segment := range Segments(text) {
		segments = append(segments, expectedSegment{
			text:     segment.Segment,
			index:    segment.Index,
			wordLike: segment.IsWordLike,
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
				{"测试", 2, true},
			},
		},
		{
			"日本語の文章です",
			[]expectedSegment{
				{"日本語", 0, true},
				{"の", 3, true},
				{"文章", 4, true},
				{"です", 6, true},
			},
		},
		{
			"한국어 문장입니다",
			[]expectedSegment{
				{"한국어", 0, true},
				{" ", 3, false},
				{"문장입니다", 4, true},
			},
		},
		{
			"Latin 中文混合 text.",
			[]expectedSegment{
				{"Latin", 0, true},
				{" ", 5, false},
				{"中文", 6, true},
				{"混合", 8, true},
				{" ", 10, false},
				{"text", 11, true},
				{".", 15, false},
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
		{"า", 1, true},
		{"ษ", 2, true},
		{"า", 3, true},
		{"ไ", 4, true},
		{"ท", 5, true},
		{"ย", 6, true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("segments = %#v, want %#v", got, want)
	}
}

func TestSegmentsUTF16IndexesAndCombiningMarks(t *testing.T) {
	t.Parallel()
	got := collectSegments("🦀中文 e\u0301")
	want := []expectedSegment{
		{"🦀", 0, false},
		{"中文", 2, true},
		{" ", 4, false},
		{"e\u0301", 5, true},
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
