package textlayout

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func benchmarkFlow(b *testing.B, text string) *surface.Flow {
	b.Helper()
	runs := []surface.StyleRun{}
	if text != "" {
		runs = append(runs, surface.StyleRun{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: surface.TextStyle{},
		})
	}
	flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, runs)
	if err != nil {
		b.Fatal(err)
	}
	return flow
}

func BenchmarkParagraphRetentionTyping(b *testing.B) {
	base := strings.Repeat("A stable paragraph for the retained layout benchmark.\n", 256)
	cache := NewCache()
	style := testStyle()
	flow := benchmarkFlow(b, base)
	_ = cache.Update(flow, 700, style)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = cache.Update(flow, 700, style)
	}
}

func BenchmarkIncrementalParagraphShape(b *testing.B) {
	base := strings.Repeat("A stable paragraph for the retained layout benchmark.\n", 256)
	cache := NewCache()
	style := testStyle()
	_ = cache.Update(benchmarkFlow(b, base), 700, style)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		flow := benchmarkFlow(b, base+"x")
		_ = cache.Update(flow, 700, style)
	}
}

func BenchmarkFirstShapeEightK(b *testing.B) {
	text := strings.Repeat("Unicode 中文 🦀 prose line.\n", 300)
	style := testStyle()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		cache := NewCache()
		_ = cache.Update(benchmarkFlow(b, text), 700, style)
	}
}

func BenchmarkFirstShape128K(b *testing.B) {
	text := strings.Repeat("Unicode 中文 🦀 prose line.\n", 4800)
	style := testStyle()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		cache := NewCache()
		_ = cache.Update(benchmarkFlow(b, text), 700, style)
	}
}
