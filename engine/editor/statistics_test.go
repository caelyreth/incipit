package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func TestComputeTextMetricsUsesScalarsWordsAndNonblankParagraphs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		expected TextMetrics
	}{
		{"one", TextMetrics{Characters: 3, Words: 1, Paragraphs: 1}},
		{"Quiet 中🦀 prose\n \nsecond line e\u0301\n", TextMetrics{Characters: 32, Words: 6, Paragraphs: 2}},
		{" \n\t\n", TextMetrics{Characters: 4, Words: 0, Paragraphs: 0}},
		{"_", TextMetrics{Characters: 1, Words: 0, Paragraphs: 1}},
		{"123,456", TextMetrics{Characters: 7, Words: 1, Paragraphs: 1}},
		{"a·b", TextMetrics{Characters: 3, Words: 1, Paragraphs: 1}},
		{"ภาษาไทย", TextMetrics{Characters: 7, Words: 7, Paragraphs: 1}},
		{"\u0301", TextMetrics{Characters: 1, Words: 0, Paragraphs: 1}},
	}
	for _, test := range tests {
		got, ok := ComputeTextMetrics(test.text)
		if !ok || got != test.expected {
			t.Errorf("%q metrics = %#v, want %#v", test.text, got, test.expected)
		}
	}
}

// A page counts block by block: a block keeps its counts while its text is
// the same, and the page's counts are the sum, line breaks being block
// boundaries rather than characters.
func TestPageMetricsCountBlocks(t *testing.T) {
	t.Parallel()
	document, blocks, err := NewBlockDocument("one two\n\nthree")
	if err != nil {
		t.Fatal(err)
	}
	var metrics PageMetrics
	if !metrics.Update(document.Snapshot()) ||
		metrics.Total() != (TextMetrics{Characters: 12, Words: 3, Paragraphs: 2}) {
		t.Fatalf("page = %#v", metrics.Total())
	}
	kept := metrics.blocks[blocks[0]]
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(blocks[2], "three four")
	}); err != nil {
		t.Fatal(err)
	}
	if !metrics.Update(document.Snapshot()) ||
		metrics.Total() != (TextMetrics{Characters: 17, Words: 4, Paragraphs: 2}) {
		t.Fatalf("after an edit = %#v", metrics.Total())
	}
	if again := metrics.blocks[blocks[0]]; !again.root.Same(kept.root) {
		t.Fatal("an untouched block was recounted")
	}
}
