package documentview

import (
	"testing"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
)

func TestLayoutGeometryAppliesTitleOffsetAndScroll(t *testing.T) {
	style := textlayout.NewStyle(
		theme.NewProseStyle(theme.DefaultSettings().ProseFontSize),
		theme.DefaultSettings().Semantic(),
	)
	text := "one\ntwo"
	flow, err := surface.NewFlow(
		projection.NewVersion(1, 1),
		text,
		[]surface.StyleRun{{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: surface.TextStyle{},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	layout := textlayout.NewCache().Update(flow, 400, style)
	contentTop := float32(72)
	scroll := style.Prose.LineHeight()
	geometry := LayoutGeometry{Layout: layout, YOffset: contentTop, ScrollY: scroll}

	bounds, _, ok := geometry.BoundsForRange(text, controller.UTF16Range{Start: 4, End: 4})
	if !ok {
		t.Fatal("bounds unavailable")
	}
	want := layout.Caret(4).Y + contentTop - scroll
	if bounds.Y != want {
		t.Fatalf("bounds Y = %v, want %v", bounds.Y, want)
	}
	firstBottom := layout.Paragraphs()[0].Height()
	if got, ok := geometry.IndexForPoint(text, controller.Point{
		X: 0,
		Y: contentTop - scroll + firstBottom + 1,
	}); !ok || got != 4 {
		t.Fatalf("scrolled hit = %d, %v", got, ok)
	}
	line, ok := geometry.CaretLine(text, 4)
	if !ok {
		t.Fatal("caret line unavailable")
	}
	shaped, ok := layout.CaretLine(4)
	if !ok {
		t.Fatal("retained caret line unavailable")
	}
	if line.Y != shaped.Y+contentTop-scroll || line.H != shaped.H {
		t.Fatalf("caret line = %#v", line)
	}
}
