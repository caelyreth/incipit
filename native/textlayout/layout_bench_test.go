package textlayout

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

const benchmarkLayoutWidth = 700

var (
	benchmarkLayout         *Layout
	benchmarkSelectionRects []ui.Rect
	benchmarkFloat32        float32
)

func benchmarkFlow(b *testing.B, text string) *surface.Flow {
	b.Helper()
	return benchmarkFlowText(b, text)
}

func benchmarkFlowText(tb testing.TB, text string) *surface.Flow {
	tb.Helper()
	runs := []surface.StyleRun{}
	if text != "" {
		runs = append(runs, surface.StyleRun{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: surface.TextStyle{},
		})
	}
	flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, runs)
	if err != nil {
		tb.Fatal(err)
	}
	return flow
}

// benchmarkProse builds a deterministic Latin-dominant manuscript with
// regular CJK paragraphs. Paragraph prefixes keep the document realistic
// rather than letting identical paragraphs mask cache behavior.
func benchmarkProse(size int) string {
	const cjk = "潮水退去以后，编辑器的节奏仍然清晰。文字在版心里自然流动，" +
		"而每一次靠近都应当安静、准确。"
	fragments := []string{
		"Morning light crossed the room while the manuscript waited, " +
			"its paragraphs settling into a quiet measure of their own.",
		"A useful editor keeps the page still when the thought moves. " +
			"Small punctuation, café names, and the occasional “quoted phrase” " +
			"should remain part of the same visual rhythm.",
		"She moved the caret through the final sentence, paused at the comma, " +
			"and watched the wrapped line hold its place.",
		cjk,
		"The tide withdrew beyond the harbor wall; gulls crossed the pale sky, " +
			"and the long paragraph continued without asking for attention.",
	}
	var text strings.Builder
	text.Grow(size + 512)
	for index := 0; text.Len() < size; index++ {
		fmt.Fprintf(&text, "Paragraph %06d. %s\n", index, fragments[index%len(fragments)])
	}
	value := text.String()[:size]
	for len(value) > 0 && !utf8.RuneStart(value[len(value)-1]) {
		value = value[:len(value)-1]
	}
	if padding := size - len(value); padding > 0 {
		value += strings.Repeat("x", padding)
	}
	return value
}

func benchmarkSizes() []struct {
	name string
	size int
} {
	return []struct {
		name string
		size int
	}{
		{name: "8KiB", size: 8 << 10},
		{name: "64KiB", size: 64 << 10},
		{name: "256KiB", size: 256 << 10},
	}
}

func BenchmarkFirstLayout(b *testing.B) {
	style := testStyle()
	for _, fixture := range benchmarkSizes() {
		text := benchmarkProse(fixture.size)
		flow := benchmarkFlow(b, text)
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(fixture.size))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				cache := NewCache()
				benchmarkLayout = cache.Update(flow, benchmarkLayoutWidth, style)
			}
		})
	}
}

func BenchmarkRetainedLayout(b *testing.B) {
	style := testStyle()
	for _, fixture := range benchmarkSizes() {
		flow := benchmarkFlow(b, benchmarkProse(fixture.size))
		cache := NewCache()
		cache.Update(flow, benchmarkLayoutWidth, style)
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(fixture.size))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				benchmarkLayout = cache.Update(flow, benchmarkLayoutWidth, style)
			}
		})
	}
}

func BenchmarkTypingLastParagraph(b *testing.B) {
	const variants = 16
	style := testStyle()
	for _, fixture := range benchmarkSizes() {
		base := benchmarkProse(fixture.size)
		flows := make([]*surface.Flow, variants)
		for index := range flows {
			// Each variant keeps every paragraph except the last unchanged.
			flows[index] = benchmarkFlowText(b, base+strings.Repeat("x", index+1))
		}
		cache := NewCache()
		for _, flow := range flows {
			cache.Update(flow, benchmarkLayoutWidth, style)
		}
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(fixture.size))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				benchmarkLayout = cache.Update(
					flows[index%len(flows)],
					benchmarkLayoutWidth,
					style,
				)
			}
		})
	}
}

func BenchmarkCaretGeometry(b *testing.B) {
	style := testStyle()
	for _, fixture := range benchmarkSizes() {
		layout := NewCache().Update(
			benchmarkFlow(b, benchmarkProse(fixture.size)),
			benchmarkLayoutWidth,
			style,
		)
		paragraphs := layout.Paragraphs()
		caret := paragraphs[len(paragraphs)/2].StartUTF16() + 17
		_ = layout.Caret(model.UTF16Offset(caret))
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(fixture.size))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				rect := layout.Caret(model.UTF16Offset(caret))
				benchmarkFloat32 += rect.X + rect.Y + rect.H
			}
		})
	}
}

func BenchmarkSelectionGeometry(b *testing.B) {
	style := testStyle()
	for _, fixture := range benchmarkSizes() {
		layout := NewCache().Update(
			benchmarkFlow(b, benchmarkProse(fixture.size)),
			benchmarkLayoutWidth,
			style,
		)
		paragraphs := layout.Paragraphs()
		paragraph := paragraphs[len(paragraphs)/2]
		start := paragraph.StartUTF16()
		end := paragraph.ContentEndUTF16()
		selection := model.NewUTF16Range(
			model.UTF16Offset(start),
			model.UTF16Offset(end),
		)
		_ = layout.RangeRects(selection)
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(fixture.size))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				benchmarkSelectionRects = layout.RangeRects(selection)
			}
		})
	}
}
