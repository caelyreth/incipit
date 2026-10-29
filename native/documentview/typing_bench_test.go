package documentview

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type typingBenchmarkClock struct{}

func (typingBenchmarkClock) Now() time.Time { return time.Unix(1_700_000_000, 0) }

func typingBenchmarkProse(size int) string {
	const cjk = "潮水退去以后，编辑器的节奏仍然清晰。文字在版心里自然流动。"
	var text strings.Builder
	text.Grow(size + 256)
	for index := 0; text.Len() < size; index++ {
		fmt.Fprintf(
			&text,
			"Paragraph %06d. Long prose keeps the page calm while every keystroke updates the document. %s\n",
			index,
			cjk,
		)
	}
	value := text.String()[:size]
	for len(value) > 0 && !utf8.RuneStart(value[len(value)-1]) {
		value = value[:len(value)-1]
	}
	return value + strings.Repeat("x", size-len(value))
}

func BenchmarkTypingFrameEndToEnd(b *testing.B) {
	for _, fixture := range []struct {
		name string
		size int
	}{
		{name: "8KiB", size: 8 << 10},
		{name: "64KiB", size: 64 << 10},
		{name: "256KiB", size: 256 << 10},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			document, err := controller.NewTextDocument(
				typingBenchmarkProse(fixture.size),
				typingBenchmarkClock{},
			)
			if err != nil {
				b.Fatal(err)
			}
			end := model.UTF16Offset(document.TextLengthUTF16())
			if err := document.Session().SelectProjected(end, end); err != nil {
				b.Fatal(err)
			}
			document.Refresh()
			bridge := NewBridge(document, nil, typingBenchmarkClock{}.Now)
			settings := theme.DefaultSettings()
			element := NewElement(
				bridge,
				settings.Semantic(),
				theme.NewProseStyle(settings.ProseFontSize),
				typingBenchmarkClock{}.Now(),
			)
			element.Animate = false
			tester := ui.NewTester(func(c *ui.Context) {
				element.View(c)
			}, 700, 900)
			tester.Frame()

			b.ReportAllocs()
			b.SetBytes(int64(fixture.size))
			b.ResetTimer()
			for b.Loop() {
				document.ReplaceText(nil, "x")
				tester.Frame()
			}
		})
	}
}
