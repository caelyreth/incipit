package documentview

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

var benchmarkPaintImage image.Rectangle

func documentBenchmarkProse(size int) string {
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

func documentBenchmarkFlow(tb testing.TB, text string) *surface.Flow {
	tb.Helper()
	flow, err := surface.NewFlow(
		projection.NewVersion(1, 1),
		text,
		[]surface.StyleRun{{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: surface.TextStyle{},
		}},
	)
	if err != nil {
		tb.Fatal(err)
	}
	return flow
}

type benchmarkDocumentView struct{ state *State }

func (v benchmarkDocumentView) view(c *ui.Context) {
	ui.Box(c).Fill().Background(v.state.Theme.Surface.Page).
		Draw(func(p *ui.Painter, rect ui.Rect) {
			Paint(p, rect, v.state)
		})
}

func BenchmarkHeadlessPaint(b *testing.B) {
	for _, fixture := range []struct {
		name string
		size int
	}{
		{name: "8KiB", size: 8 << 10},
		{name: "64KiB", size: 64 << 10},
		{name: "256KiB", size: 256 << 10},
	} {
		text := documentBenchmarkProse(fixture.size)
		flow := documentBenchmarkFlow(b, text)
		caret := flow.UTF16Len()
		frame, err := surface.NewFrame(
			flow,
			model.EmptyUTF16Range(caret),
			caret,
			surface.TextStyle{},
			nil,
		)
		if err != nil {
			b.Fatal(err)
		}
		settings := theme.DefaultSettings()
		prose := theme.NewProseStyle(settings.ProseFontSize)
		semantic := settings.Semantic()
		layout := textlayout.NewCache().Update(
			flow,
			700,
			textlayout.NewStyle(prose, semantic),
		)
		paragraphs := layout.Paragraphs()
		mid := paragraphs[len(paragraphs)/2]
		state := &State{
			Flow:    flow,
			Frame:   frame,
			Layout:  layout,
			Motion:  motion.New(time.Unix(1_700_000_000, 0)),
			Theme:   semantic,
			Prose:   prose,
			ScrollY: mid.Top(),
			Focused: true,
		}
		tester := ui.NewTester((benchmarkDocumentView{state: state}).view, 700, 900)
		tester.Frame()
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(fixture.size))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				tester.Frame()
				benchmarkPaintImage = tester.Image().Rect
			}
		})
	}
}
