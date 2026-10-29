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

func TestCaretUsesProseHeightAndVisualRow(t *testing.T) {
	style := testStyle()
	layout := NewCache().Update(testFlow(t, "one two three four five six seven eight nine ten", 0), 80, style)
	caret := layout.Caret(0)
	if caret.H != style.Prose.CaretHeight() || caret.W != 2 {
		t.Fatalf("caret = %#v, prose=%#v", caret, style.Prose)
	}
	if caret.Y < (style.Prose.LineHeight()-style.Prose.CaretHeight())/2 ||
		caret.Y > style.Prose.LineHeight() {
		t.Fatalf("wrapped first-row caret Y = %v", caret.Y)
	}
}

func TestCaretLineExposesWholeVisualRowsAcrossWraps(t *testing.T) {
	style := testStyle()
	layout := NewCache().Update(
		testFlow(t, "one two three four five six seven eight nine ten", 0),
		80,
		style,
	)
	first, ok := layout.CaretLine(0)
	if !ok || first.H <= 0 {
		t.Fatalf("first caret line = %#v, %v", first, ok)
	}
	secondOffset := -1
	var second ui.Rect
	for offset := 1; offset < int(layout.Flow().UTF16Len()); offset++ {
		line, available := layout.CaretLine(model.UTF16Offset(offset))
		if available && line.Y > first.Y {
			secondOffset, second = offset, line
			break
		}
	}
	if secondOffset < 0 || second.Y != first.Y+first.H || second.H != first.H {
		t.Fatalf("wrap rows first=%#v second=%#v offset=%d", first, second, secondOffset)
	}
	hit := layout.Hit(ui.Point{X: first.X, Y: second.Y + second.H/2})
	if hit < model.UTF16Offset(secondOffset) ||
		hit > model.UTF16Offset(secondOffset+4) {
		t.Fatalf("second-row hit = %d, row starts at %d", hit, secondOffset)
	}
}

func TestParagraphStartMapsToFollowingParagraph(t *testing.T) {
	style := testStyle()
	layout := NewCache().Update(testFlow(t, "a\nb", 0), 300, style)
	first := layout.Caret(0)
	second := layout.Caret(2)
	if second.Y <= first.Y || second.Y < style.Prose.LineHeight() {
		t.Fatalf("carets first=%#v second=%#v", first, second)
	}
}

func TestNewlineSelectionUsesOneLineHeight(t *testing.T) {
	style := testStyle()
	text := "one two three four five six seven eight nine\nten"
	layout := NewCache().Update(testFlow(t, text, 0), 80, style)
	rects := layout.RangeRects(model.NewUTF16Range(
		0,
		model.UTF16Offset(model.StringLenUTF16(text)),
	))
	if len(rects) == 0 {
		t.Fatal("selection produced no rectangles")
	}
	for _, rect := range rects {
		if rect.H != style.Prose.LineHeight() {
			t.Fatalf("selection row height = %v, want %v", rect.H, style.Prose.LineHeight())
		}
	}
}

func TestDocumentEndBelongsToFinalParagraph(t *testing.T) {
	style := testStyle()
	for _, text := range []string{"hello", "one\nhello", "one two three four five six seven eight nine ten"} {
		layout := NewCache().Update(testFlow(t, text, 0), 80, style)
		end := model.UTF16Offset(layout.Flow().UTF16Len())
		last := layout.Paragraphs()[len(layout.Paragraphs())-1]
		before := layout.Caret(end - 1)
		caret := layout.Caret(end)
		want := last.Layout().Caret(int(end) - last.StartUTF16())
		if caret.X != want.X || caret.X <= before.X {
			t.Fatalf("%q end caret = %#v, before = %#v, shaped end x = %v",
				text, caret, before, want.X)
		}
		if caret.Y < last.Top() || caret.Y >= last.Top()+last.Height() {
			t.Fatalf("%q end caret y = %v outside final paragraph %v..%v",
				text, caret.Y, last.Top(), last.Top()+last.Height())
		}
		if got := layout.PreviousBoundary(end); got != end-1 {
			t.Fatalf("%q previous boundary from end = %d", text, got)
		}
		if got := layout.NextBoundary(end); got != end {
			t.Fatalf("%q next boundary from end = %d", text, got)
		}
	}
}
