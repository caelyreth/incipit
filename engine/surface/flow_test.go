package surface

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

func fixtureFlow(t *testing.T) *Flow {
	t.Helper()
	flow, err := NewFlow(
		projection.NewVersion(1, 2),
		"a🦀bc",
		[]StyleRun{
			{Bytes: model.NewByteRange(0, 5), Style: TextStyle{}},
			{Bytes: model.NewByteRange(5, 7), Style: TextStyle{Bold: true}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func TestFlowRejectsGapsOverlapsAndSplitUnicode(t *testing.T) {
	t.Parallel()
	for _, bytes := range []model.ByteRange{
		model.NewByteRange(0, 1),
		model.NewByteRange(1, 7),
		model.NewByteRange(0, 2),
		model.NewByteRange(7, 0),
	} {
		if _, err := NewFlow(
			projection.NewVersion(0, 0),
			"a🦀bc",
			[]StyleRun{{Bytes: bytes}},
		); err == nil {
			t.Fatalf("invalid run accepted: %#v", bytes)
		}
	}
	if _, err := NewFrame(
		fixtureFlow(t),
		model.NewUTF16Range(2, 2),
		model.UTF16Offset(2),
		TextStyle{},
		nil,
	); err == nil {
		t.Fatal("split-surrogate selection accepted")
	}
}

func TestPreviewPreservesSurroundingRuns(t *testing.T) {
	t.Parallel()
	flow := fixtureFlow(t)
	original, err := NewFrame(
		flow,
		model.NewUTF16Range(1, 4),
		model.UTF16Offset(4),
		TextStyle{Italic: true},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := original.WithPreview(
		model.NewUTF16Range(1, 4),
		"你",
		model.NewUTF16Range(1, 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Flow().Text() != "a你c" {
		t.Fatalf("preview = %q", preview.Flow().Text())
	}
	if preview.Selection() != model.NewUTF16Range(2, 2) {
		t.Fatalf("selection = %#v", preview.Selection())
	}
	runs := preview.Flow().Runs()
	if len(runs) != 3 || runs[0].Bytes != model.NewByteRange(0, 1) ||
		runs[1].Bytes != model.NewByteRange(1, 4) ||
		runs[2].Bytes != model.NewByteRange(4, 5) {
		t.Fatalf("runs = %#v", runs)
	}
	if !runs[1].Style.Italic || !runs[2].Style.Bold {
		t.Fatalf("preview styles = %#v", runs)
	}
	if flow.Text() != "a🦀bc" {
		t.Fatal("preview mutated source")
	}
}
