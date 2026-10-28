package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func applyPlan(text string, batch CorrectionBatch) (string, bool) {
	result, _, ok := ApplyCorrections(text, model.UTF16Offset(model.StringLenUTF16(text)), batch)
	return result, ok
}

func TestPlanCJKRefinesParagraphsAndPreservesTokens(t *testing.T) {
	t.Parallel()
	text := "中文test, 123.4 https://x.com"
	plans, err := PlanCJK(text, model.NewUTF16Range(0, model.UTF16Offset(model.StringLenUTF16(text))), CJKSettings{
		Punctuation: true,
		Spacing:     true,
	})
	if err != nil || len(plans) != 1 {
		t.Fatalf("plans=%#v err=%v", plans, err)
	}
	result, ok := applyPlan(text, plans[0].Corrections)
	if !ok || result != "中文 test，123.4 https://x.com" {
		t.Fatalf("result=%q ok=%v", result, ok)
	}
}

func TestPlanCJKOnlyTouchesInsertedLines(t *testing.T) {
	t.Parallel()
	text := "english，line\n中文line"
	start := model.UTF16Offset(len("english，line\n"))
	end := start + 2
	plans, err := PlanCJK(text, model.NewUTF16Range(start, end), CJKSettings{Punctuation: true, Spacing: true})
	if err != nil || len(plans) != 1 || plans[0].Line != model.NewByteRange(model.ByteOffset(len("english，line\n")), model.ByteOffset(len(text))) {
		t.Fatalf("plans=%#v err=%v", plans, err)
	}
	result, _ := applyPlan(text, plans[0].Corrections)
	if result != "english，line\n中文 line" {
		t.Fatalf("result=%q", result)
	}
}

func TestPlanCJKPunctuationSpacingAndLatinNarrowing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text     string
		settings CJKSettings
		expected string
	}{
		{"中文, test.", CJKSettings{Punctuation: true}, "中文，test。"},
		{"中文 ,test", CJKSettings{Punctuation: true}, "中文，test"},
		{"english， only。", CJKSettings{Punctuation: true}, "english, only."},
		{"中文test", CJKSettings{Spacing: true}, "中文 test"},
		{"test中文", CJKSettings{Spacing: true}, "test 中文"},
		{"中文 ,test.", CJKSettings{Spacing: true}, "中文 ,test."},
	}
	for _, test := range tests {
		plans, err := PlanCJK(test.text, model.NewUTF16Range(0, model.UTF16Offset(model.StringLenUTF16(test.text))), test.settings)
		if err != nil || len(plans) != 1 {
			t.Fatalf("%q plans=%#v err=%v", test.text, plans, err)
		}
		result, ok := applyPlan(test.text, plans[0].Corrections)
		if !ok || result != test.expected {
			t.Errorf("%q = %q, want %q, ok=%v", test.text, result, test.expected, ok)
		}
	}
}

func TestPlanCJKDoesNotSplitCombiningGraphemes(t *testing.T) {
	t.Parallel()
	text := "e\u0301中"
	plans, err := PlanCJK(text, model.NewUTF16Range(0, 3), CJKSettings{Punctuation: false, Spacing: true})
	if err != nil || len(plans) != 1 {
		t.Fatalf("plans=%#v err=%v", plans, err)
	}
	result, _ := applyPlan(text, plans[0].Corrections)
	if result != "e\u0301 中" {
		t.Fatalf("result=%q", result)
	}
}

func TestCorrectionBatchIsReverseSortedAndRejectsOverlap(t *testing.T) {
	t.Parallel()
	batch, err := NewCorrectionBatch([]Correction{
		{Range: model.NewUTF16Range(1, 2), Replacement: "b"},
		{Range: model.NewUTF16Range(4, 5), Replacement: "e"},
	})
	if err != nil || len(batch.Edits()) != 2 || batch.Edits()[0].Range.Start != 4 {
		t.Fatalf("batch=%#v err=%v", batch, err)
	}
	if _, err := NewCorrectionBatch([]Correction{
		{Range: model.NewUTF16Range(0, 2)},
		{Range: model.NewUTF16Range(1, 3)},
	}); err == nil {
		t.Fatal("overlap accepted")
	}
	if _, err := NewCorrectionBatch([]Correction{{Range: model.NewUTF16Range(2, 1)}}); err == nil {
		t.Fatal("reversed range accepted")
	}
}

func TestCorrectionLogPreservesOrderAndCopies(t *testing.T) {
	t.Parallel()
	var log CorrectionLog
	log.Record(3)
	log.Record(-1)
	log.Record(5)
	got := log.Indices()
	if !slices.Equal(got, []int{3, 5}) {
		t.Fatalf("indices=%#v", got)
	}
	got[0] = 9
	if log.Indices()[0] != 3 {
		t.Fatal("indices aliased internal storage")
	}
	log.Clear()
	if log.Len() != 0 {
		t.Fatal("clear retained indices")
	}
}
