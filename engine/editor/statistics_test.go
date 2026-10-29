package editor

import (
	"reflect"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func TestIncrementalTextMetricsMatchesFullRecount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		before   string
		after    string
		edit     model.TextEdit
		multiple bool
	}{
		{
			name:   "append within line",
			before: "one two\nthree",
			after:  "one two three\nthree",
			edit:   model.NewTextEditReplacement(1, model.NewUTF16Range(7, 7), " three"),
		},
		{
			name:   "insert paragraph",
			before: "one two",
			after:  "one two\nnext",
			edit:   model.NewTextEditReplacement(1, model.NewUTF16Range(7, 7), "\nnext"),
		},
		{
			name:   "remove paragraph break",
			before: "one\n\ntwo",
			after:  "onetwo",
			edit:   model.NewTextEditReplacement(1, model.NewUTF16Range(3, 5), ""),
		},
		{
			name:   "replace CJK",
			before: "alpha 中文 omega",
			after:  "alpha 日本語 omega",
			edit:   model.NewTextEditReplacement(1, model.NewUTF16Range(6, 8), "日本語"),
		},
		{
			name:     "multiple edits fall back",
			before:   "one two",
			after:    "two",
			edit:     model.NewTextEditReplacement(1, model.NewUTF16Range(0, 4), ""),
			multiple: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			previous, ok := ComputeTextMetrics(test.before)
			if !ok {
				t.Fatal("before metrics rejected")
			}
			edits := []model.TextEdit{test.edit}
			if test.multiple {
				edits = append(edits, model.NewTextEditReplacement(1, model.NewUTF16Range(0, 0), ""))
			}
			got, ok := incrementalTextMetrics(
				model.NewTextRoot(test.before),
				true,
				model.NewTextRoot(test.after),
				true,
				edits,
			)
			if test.multiple {
				if ok {
					t.Fatalf("multiple edits returned incremental metrics %#v", got)
				}
				return
			}
			got.Characters += previous.Characters
			got.Words += previous.Words
			got.Paragraphs += previous.Paragraphs
			want, ok := ComputeTextMetrics(test.after)
			if !ok || !reflect.DeepEqual(got, want) {
				t.Fatalf("metrics = %#v, want %#v, ok=%v", got, want, ok)
			}
		})
	}
}

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
