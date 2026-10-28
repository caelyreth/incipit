package textlayout

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func testFlow(t *testing.T, text string, boldFrom int) *surface.Flow {
	t.Helper()
	runs := []surface.StyleRun{}
	if text != "" {
		if boldFrom <= 0 {
			runs = append(runs, surface.StyleRun{
				Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
				Style: surface.TextStyle{},
			})
		} else {
			runs = append(runs,
				surface.StyleRun{
					Bytes: model.NewByteRange(0, model.ByteOffset(boldFrom)),
					Style: surface.TextStyle{},
				},
				surface.StyleRun{
					Bytes: model.NewByteRange(model.ByteOffset(boldFrom), model.ByteOffset(len(text))),
					Style: surface.TextStyle{Bold: true},
				},
			)
		}
	}
	flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, runs)
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func testStyle() Style {
	settings := theme.DefaultSettings()
	return NewStyle(theme.NewProseStyle(settings.ProseFontSize), settings.Semantic())
}

func TestParagraphRetentionAndUnicodeCoordinates(t *testing.T) {
	cache := NewCache()
	first := cache.Update(testFlow(t, "alpha\nbeta", 0), 240, testStyle())
	second := cache.Update(testFlow(t, "alpha\ngamma", 0), 240, testStyle())
	firstParagraphs := first.Paragraphs()
	secondParagraphs := second.Paragraphs()
	if len(firstParagraphs) != 2 || len(secondParagraphs) != 2 {
		t.Fatalf("paragraph counts = %d,%d", len(firstParagraphs), len(secondParagraphs))
	}
	if firstParagraphs[0].layout != secondParagraphs[0].layout {
		t.Fatal("unchanged paragraph was reshaped")
	}
	if firstParagraphs[1].layout == secondParagraphs[1].layout {
		t.Fatal("changed paragraph reused layout")
	}
	if firstParagraphs[0].startUTF16 != 0 || firstParagraphs[0].contentUTF16 != 5 ||
		firstParagraphs[0].endUTF16 != 6 ||
		firstParagraphs[1].startUTF16 != 6 || firstParagraphs[1].endUTF16 != 10 {
		t.Fatalf("ranges = %#v", firstParagraphs)
	}
	if firstParagraphs[0].top != 0 || firstParagraphs[1].top <= firstParagraphs[0].top {
		t.Fatalf("tops = %v,%v", firstParagraphs[0].top, firstParagraphs[1].top)
	}
}

func TestCaretHitSelectionAndBoundaries(t *testing.T) {
	style := testStyle()
	layout := NewCache().Update(testFlow(t, "one\n\n中文🦀", 0), 300, style)
	if len(layout.Paragraphs()) != 3 {
		t.Fatalf("paragraphs = %d", len(layout.Paragraphs()))
	}
	for _, offset := range []model.UTF16Offset{0, 3, 4, 5, 11} {
		caret := layout.Caret(offset)
		if caret.H <= 0 {
			t.Fatalf("caret %d height = %v", offset, caret.H)
		}
	}
	if got := layout.Hit(ui.Point{X: 0, Y: 0}); got != 0 {
		t.Fatalf("top hit = %d", got)
	}
	if got := layout.Hit(ui.Point{X: 0, Y: layout.Height() + 100}); got != 9 {
		t.Fatalf("below hit = %d", got)
	}
	rects := layout.RangeRects(model.NewUTF16Range(0, 9))
	if len(rects) < 3 {
		t.Fatalf("selection rects = %#v", rects)
	}
	if rect, ok := layout.BoundsForRange(model.NewUTF16Range(5, 9)); !ok || rect.H <= 0 {
		t.Fatalf("bounds = %#v,%v", rect, ok)
	}
	if got := layout.PreviousBoundary(6); got >= 6 {
		t.Fatalf("previous boundary = %d", got)
	}
	if got := layout.NextBoundary(3); got != 4 {
		t.Fatalf("next newline boundary = %d", got)
	}
}

func TestBoldStyleReachesMyGoSpans(t *testing.T) {
	flow := testFlow(t, "plainbold", 5)
	spans := spansFor(flow, 0, len(flow.Text()), testStyle())
	if len(spans) != 2 || spans[0].Weight != 400 || spans[1].Weight != 700 {
		t.Fatalf("spans = %#v", spans)
	}
	if spans[1].Text != "bold" {
		t.Fatalf("bold text = %q", spans[1].Text)
	}
}

func TestVisibleBoundsAndCaretAtPointRespectViewport(t *testing.T) {
	flow := testFlow(t, "one\ntwo\nthree", -1)
	layout := NewCache().Update(flow, 300, testStyle())
	if layout == nil {
		t.Fatal("layout missing")
	}
	visible := layout.VisibleBounds(layout.Height()/2, 10)
	if visible.Y != layout.Height()/2 || visible.H <= 0 {
		t.Fatalf("visible = %#v height=%v", visible, layout.Height())
	}
	caret := layout.CaretAtPoint(ui.Point{X: 0, Y: layout.Height() / 2})
	if caret.H <= 0 {
		t.Fatalf("caret = %#v", caret)
	}
}
