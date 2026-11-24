package page

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/manifest"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

func newBenchElement(b *testing.B, content string) *Element {
	b.Helper()
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		b.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		b.Fatal(err)
	}
	ed := NewEditor(engine.New(snap, prog, engine.Options{}), 1)
	el := NewElement(ed)
	el.Theme = theme.DefaultSettings().Semantic()
	el.Prose = theme.DefaultSettings().Prose()
	el.Editor.Do(vocab.Paste{Text: content}, time.Unix(100, 0))
	return el
}

func BenchmarkThemeTransitionStep(b *testing.B) {
	var body strings.Builder
	for i := range 12 {
		fmt.Fprintf(&body, "# Heading %d\n\nPlain *emphasis* and **bold** with `code` plus #tag%d and math $x^2+y^2=z_%d$.\n\n", i+1, i, i)
	}
	el := newBenchElement(b, body.String())
	starfield := theme.DefaultSettings()
	daybreak := starfield
	daybreak.Appearance = theme.Daybreak
	var tr theme.Transition
	start := time.Unix(1_700_000_000, 0)
	tr.Jump(starfield.Semantic())
	tr.Retarget(daybreak.Semantic(), start)
	stepA, _ := tr.At(daybreak, start.Add(80*time.Millisecond))
	stepB, _ := tr.At(daybreak, start.Add(160*time.Millisecond))
	steps := [2]theme.Theme{stepA, stepB}

	tt := ui.NewTester(el.View, 900, 700)
	tt.Frame()
	startRelayouts := el.Editor.relayouts

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		el.Theme = steps[i&1]
		tt.Frame()
	}
	b.StopTimer()
	b.ReportMetric(float64(el.Editor.relayouts-startRelayouts)/float64(b.N), "relayouts/op")
}

func BenchmarkFormulaColdTypeset(b *testing.B) {
	lc := newLayoutCache()
	samples := []struct {
		id      doc.ID
		src     string
		dialect profile.MathDialect
		display bool
	}{
		{1, `\frac{a+b}{\sqrt{x^2+y^2}}`, profile.DialectTeX, false},
		{2, `\sum_{k=1}^n k^2 = \frac{n(n+1)(2n+1)}{6}`, profile.DialectTeX, true},
		{3, `frac(a+b, sqrt(x^2+y^2))`, profile.DialectTypst, false},
		{4, `sum_(k=1)^n k^2 = frac(n(n+1)(2n+1), 6)`, profile.DialectTypst, true},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		formulaCache.Lock()
		clear(formulaCache.boxes)
		formulaCache.Unlock()
		clear(lc.good)
		for _, s := range samples {
			_ = lc.typeset(s.id, s.src, s.dialect, s.display, 16, 480)
		}
	}
}

func BenchmarkDialectPillLayout(b *testing.B) {
	el := newBenchElement(b, "hello")
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		d := profile.DialectTeX
		if i%2 == 1 {
			d = profile.DialectTypst
		}
		_, _, _ = el.dialectPillLayout(d)
	}
}
