package textlayout

import (
	"math"
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
	spans := spansFor(flow, 0, len(flow.Text()), testStyle(), 400)
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
	wantY := (style.Prose.LineHeight()-style.Prose.CaretHeight())/2 +
		style.Prose.CaretOffsetY()
	if caret.Y != wantY {
		t.Fatalf("wrapped first-row caret Y = %v, want %v", caret.Y, wantY)
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
	// The first row of the second paragraph takes the gap above it too.
	second := layout.Paragraphs()[1]
	for _, rect := range rects {
		want := style.Prose.LineHeight()
		if rect.Y == second.Top()-second.gap {
			want += second.gap
		}
		if rect.H != want {
			t.Fatalf("selection row %v height = %v, want %v", rect, rect.H, want)
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

// A caret grows with its row: on a heading it is taller than in body text,
// and stays centered in the heading's row.
func TestCaretGrowsWithItsRow(t *testing.T) {
	style := NewStyle(theme.NewProseStyle(19), theme.DefaultSettings().Semantic())
	text := "Heading\nbody"
	flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, []surface.StyleRun{
		{Bytes: model.NewByteRange(0, 8), Style: surface.TextStyle{Scale: 1.6}},
		{Bytes: model.NewByteRange(8, model.ByteOffset(len(text))), Style: surface.TextStyle{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, 600, style)
	heading, body := layout.Caret(3), layout.Caret(10)
	if body.H != style.Prose.CaretHeight() || heading.H <= body.H {
		t.Fatalf("heading caret %v, body caret %v", heading, body)
	}
	row, _ := layout.CaretLine(3)
	if gap := (heading.Y - row.Y) - (row.Y + row.H - heading.Y - heading.H); gap < -4 || gap > 4 {
		t.Fatalf("heading caret %v is not centered in its row %v", heading, row)
	}
}

// A selection across a heading's end is as tall there as the heading's
// row: the sliver standing for the break leaves no notch.
func TestSelectionBreakTakesItsRowHeight(t *testing.T) {
	style := NewStyle(theme.NewProseStyle(19), theme.DefaultSettings().Semantic())
	text := "Heading\nbody"
	flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, []surface.StyleRun{
		{Bytes: model.NewByteRange(0, 8), Style: surface.TextStyle{Scale: 1.6}},
		{Bytes: model.NewByteRange(8, model.ByteOffset(len(text))), Style: surface.TextStyle{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, 600, style)
	rects := layout.RangeRects(model.NewUTF16Range(0, 12))
	row, _ := layout.CaretLine(3)
	for _, rect := range rects {
		if rect.Y < row.Y+row.H && (rect.Y != row.Y || rect.H != row.H) {
			t.Fatalf("heading row %v, selection piece %v", row, rect)
		}
	}
}

// rhythmFlow is a flow of lines, each set in its own style.
func rhythmFlow(t *testing.T, lines ...surface.TextStyle) *surface.Flow {
	t.Helper()
	text, runs := "", make([]surface.StyleRun, 0, len(lines))
	for index, style := range lines {
		line := "line"
		if index < len(lines)-1 {
			line += "\n"
		}
		runs = append(runs, surface.StyleRun{
			Bytes: model.NewByteRange(model.ByteOffset(len(text)), model.ByteOffset(len(text)+len(line))),
			Style: style,
		})
		text += line
	}
	flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, runs)
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func near(a, b float32) bool { return a-b < 0.5 && b-a < 0.5 }

// Every room on the page follows the one spacing value: a body row is
// its text plus the body leading, a heading's its larger text plus the
// same, entries and verbatim lines are packed closer, and no row is
// shorter than its face. Blocks stand a block step apart, paragraphs of
// prose too, but the lines of a run of entries or code do not; a heading
// stands two steps from what is before it, and one from what it opens.
func TestRhythmDerivesFromSpacing(t *testing.T) {
	title := surface.TextStyle{Scale: 1.6, Rhythm: surface.RhythmTitle}
	prose := surface.TextStyle{}
	entry := surface.TextStyle{Rhythm: surface.RhythmEntry}
	code := surface.TextStyle{Face: surface.FaceMono, Scale: 0.88, Rhythm: surface.RhythmVerbatim}
	flow := rhythmFlow(t, prose, title, prose, prose, entry, entry, code, code, prose)
	for _, spacing := range []float32{theme.MinLineSpacing, 1.8, theme.MaxLineSpacing} {
		p := theme.NewProseStyle(19).WithSpacing(spacing)
		style := NewStyle(p, theme.DefaultSettings().Semantic())
		rows := NewCache().Update(flow, 600, style).Paragraphs()
		// A row starts at body size, so its floor is the prose face at least.
		face := func(run surface.TextStyle) float32 {
			span := style.span("line", surface.StyleRun{Style: run})
			_, height := ui.ShapeText("line", ui.Font{Family: span.Font, Size: span.Size, Weight: span.Weight}, 0).Size()
			return height
		}
		body := p.Leading()
		heading := style.span("line", surface.StyleRun{Style: title}).Size
		heights := []float32{
			p.LineHeight(), max(heading+body, face(title)), p.LineHeight(), p.LineHeight(),
			max(p.FontSize+body*entryLeading, face(prose)), 0,
			max(p.FontSize+body*verbatimLeading, face(prose)), 0,
			p.LineHeight(),
		}
		heights[5], heights[7] = heights[4], heights[6]
		px := func(v float32) float32 { return float32(math.Round(float64(v))) }
		step := px(body * blockStep)
		gaps := []float32{0, px(body * blockStep * titleSteps), step, step, step, 0, step, 0, step}
		for index, row := range rows {
			if !near(row.Height(), heights[index]) || !near(row.gap, gaps[index]) {
				t.Fatalf("spacing %v row %d: height %v gap %v, want %v and %v",
					spacing, index, row.Height(), row.gap, heights[index], gaps[index])
			}
			if index > 0 {
				previous := rows[index-1]
				if !near(row.Top(), previous.Top()+previous.Height()+row.gap) {
					t.Fatalf("spacing %v row %d top %v after %v", spacing, index, row.Top(), previous)
				}
			}
		}
		if spacing == 1.8 && !(rows[4].Height() < rows[0].Height() && rows[6].Height() < rows[4].Height()) {
			t.Fatalf("entries %v and code %v are not packed closer than prose %v",
				rows[4].Height(), rows[6].Height(), rows[0].Height())
		}
	}
}

// A selection across blocks takes the gaps between them, so it is one
// unbroken area; one that starts at a block's text leaves the gap above.
func TestSelectionSpansBlockGaps(t *testing.T) {
	flow := rhythmFlow(t, surface.TextStyle{}, surface.TextStyle{Rhythm: surface.RhythmEntry}, surface.TextStyle{})
	layout := NewCache().Update(flow, 600, testStyle())
	rows := layout.Paragraphs()
	if rows[1].gap <= 0 {
		t.Fatalf("no gap before the entry: %v", rows[1])
	}
	rects := layout.RangeRects(model.NewUTF16Range(0, model.UTF16Offset(flow.UTF16Len())))
	bottom := float32(0)
	for _, rect := range rects {
		if rect.Y > bottom+0.01 {
			t.Fatalf("selection leaves a hole above %v: %v", rect, rects)
		}
		bottom = max(bottom, rect.Y+rect.H)
	}
	inside := layout.RangeRects(model.NewUTF16Range(model.UTF16Offset(rows[1].StartUTF16()), model.UTF16Offset(rows[1].ContentEndUTF16())))
	if len(inside) == 0 || inside[0].Y != rows[1].Top() {
		t.Fatalf("selection inside the entry %v starts at %v", inside, rows[1].Top())
	}
}

// A caret in a row packed closer than body text keeps the body's size,
// centered in its row.
func TestCaretInACloseRow(t *testing.T) {
	style := testStyle()
	flow := rhythmFlow(t, surface.TextStyle{}, surface.TextStyle{Rhythm: surface.RhythmEntry})
	layout := NewCache().Update(flow, 600, style)
	row := layout.Paragraphs()[1]
	caret := layout.Caret(model.UTF16Offset(row.StartUTF16() + 1))
	if caret.H != style.Prose.CaretHeight() {
		t.Fatalf("caret %v, want height %v", caret, style.Prose.CaretHeight())
	}
	above, below := caret.Y-row.Top(), row.Top()+row.Height()-caret.Y-caret.H
	if d := above - below; d < -4 || d > 4 {
		t.Fatalf("caret %v is not centered in row %v..%v", caret, row.Top(), row.Top()+row.Height())
	}
}
