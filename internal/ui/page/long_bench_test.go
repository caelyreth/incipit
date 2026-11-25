package page

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/vocab"
)

const longParagraph = "Some **marked** text, a #tag and `code`, and plain words after it to fill a line or two. "

// longPage is a page of n paragraphs, or one paragraph n sentences long.
func longPage(n int, oneParagraph bool) string {
	if oneParagraph {
		return strings.Repeat(longParagraph, n)
	}
	return strings.Repeat(longParagraph+"\n\n", n)
}

// A keystroke in the middle of a long page, laid out as the page does
// for a frame: a letter typed, then taken back, so the page stays as long.
func BenchmarkAKeystrokeOnALongPage(b *testing.B) {
	for _, c := range []struct {
		name string
		n    int
		one  bool
	}{{"paragraphs=2000", 2000, false}, {"paragraphs=20000", 20000, false}, {"oneParagraph=16KiB", 16 << 10 / len(longParagraph), true}, {"oneParagraph=64KiB", 64 << 10 / len(longParagraph), true}} {
		b.Run(c.name, func(b *testing.B) {
			el := newBenchElement(b, longPage(c.n, c.one))
			el.viewport.W, el.viewport.H = 900, 600
			now := time.Unix(200, 0)
			el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, now)
			if c.one {
				for range c.n * 8 {
					el.Editor.Do(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward}, now)
				}
			} else {
				for range c.n / 2 {
					el.Editor.Do(vocab.Move{Unit: vocab.ByParagraph, Dir: vocab.Forward}, now)
				}
			}
			el.relayout()
			for i := 0; b.Loop(); i++ {
				now = now.Add(time.Second)
				if i%2 == 0 {
					el.Editor.Do(vocab.Type{Text: string(rune('a' + i/2%26))}, now)
				} else {
					el.Editor.Do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}, now)
				}
				el.relayout()
			}
		})
	}
}

// What an open, laid-out page holds per byte of its text.
func BenchmarkTheMemoryOfAnOpenPage(b *testing.B) {
	for _, c := range []struct {
		name string
		n    int
		one  bool
	}{{"paragraphs=256KiB", 256 << 10 / (len(longParagraph) + 2), false}, {"oneParagraph=256KiB", 256 << 10 / len(longParagraph), true}} {
		b.Run(c.name, func(b *testing.B) {
			text := longPage(c.n, c.one)
			var held float64
			for b.Loop() {
				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				el := newBenchElement(b, text)
				el.viewport.W, el.viewport.H = 900, 600
				el.relayout()
				runtime.GC()
				runtime.ReadMemStats(&after)
				held = float64(after.HeapAlloc-before.HeapAlloc) / float64(len(text))
				runtime.KeepAlive(el)
			}
			b.ReportMetric(held, "B/byte")
		})
	}
}
