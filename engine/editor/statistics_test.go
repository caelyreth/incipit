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

func TestTextMetricsSourceTracksImmutableRootIdentity(t *testing.T) {
	t.Parallel()
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	root := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(root, "one")
	}); err != nil {
		t.Fatal(err)
	}
	node, _ := document.Snapshot().Node(root)
	textRoot, hasRoot := node.TextRoot()
	source, ok := NewTextMetricsSource(root, textRoot, hasRoot)
	if !ok || !source.Matches(node) || source.Metrics.Words != 1 {
		t.Fatalf("source=%#v ok=%v", source, ok)
	}
	if source.Matches(nil) {
		t.Fatal("nil node matched")
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(root, "two")
	}); err != nil {
		t.Fatal(err)
	}
	changed, _ := document.Snapshot().Node(root)
	if source.Matches(changed) {
		t.Fatal("changed text root treated as retained")
	}
}
