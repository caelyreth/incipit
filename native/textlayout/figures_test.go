package textlayout

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

func TestAFigureIsSetAtItsSizeWithinTheColumn(t *testing.T) {
	for _, test := range []struct {
		name   string
		figure surface.Figure
		want   ui.Rect
	}{
		{"its own size, centered", surface.Figure{Width: 200, Height: 100}, ui.Rect{X: 100, W: 200, H: 100}},
		{"at the start", surface.Figure{Width: 200, Height: 100, Align: surface.FigureStart}, ui.Rect{W: 200, H: 100}},
		{"no wider than the column", surface.Figure{Width: 1600, Height: 800}, ui.Rect{W: 400, H: 200}},
		{"a share of the column", surface.Figure{Width: 100, Height: 100, Span: 0.5}, ui.Rect{X: 100, W: 200, H: 200}},
		{"no taller than its bound", surface.Figure{Width: 100, Height: 1000}, ui.Rect{X: 175, W: 50, H: 500}},
		{"never too small to see", surface.Figure{Width: 4, Height: 4}, ui.Rect{X: 188, W: 24, H: 24}},
	} {
		if got := figureBox(test.figure, 400); got != test.want {
			t.Errorf("%s: %+v, want %+v", test.name, got, test.want)
		}
	}
}

// A figure takes its room above its caption's line, so the line and what
// follows move down by it, and a click in it finds it.
func TestAFigureTakesItsRoomAboveItsLine(t *testing.T) {
	plain := NewCache().Update(testFlow(t, "one\ntwo\nthree", 0), 400, testStyle())
	flow, err := testFlow(t, "one\ntwo\nthree", 0).WithFigures([]surface.Figure{
		{At: 4, Node: 9, Media: "m", Width: 200, Height: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	layout := NewCache().Update(flow, 400, testStyle())
	figures := layout.Figures()
	if len(figures) != 1 {
		t.Fatalf("figures = %+v", figures)
	}
	figure := figures[0]
	if figure.Line != model.NewUTF16Range(4, 7) || figure.Box.W != 200 || figure.Box.H != 100 || figure.Box.X != 100 {
		t.Fatalf("figure = %+v", figure)
	}
	before, after := plain.Paragraphs(), layout.Paragraphs()
	if after[0].Top() != before[0].Top() {
		t.Fatal("the line above the figure moved")
	}
	if after[1].Top() < figure.Box.Y+figure.Box.H || after[1].Top() <= before[1].Top()+100 {
		t.Fatalf("the caption at %v does not clear the figure %+v", after[1].Top(), figure.Box)
	}
	if figure.Box.Y <= before[0].Top()+before[0].Height() {
		t.Fatal("the figure touches the line above it")
	}
	if layout.Height() <= plain.Height()+100 {
		t.Fatal("the page did not grow by the figure")
	}
	hit, ok := layout.FigureAt(ui.Point{X: 150, Y: figure.Box.Y + 50})
	if !ok || hit.Node != 9 {
		t.Fatalf("a click in the figure found %+v, %v", hit, ok)
	}
	if _, ok := layout.FigureAt(ui.Point{X: 20, Y: figure.Box.Y + 50}); ok {
		t.Fatal("a click beside the figure found it")
	}
	// The caption centers under a centered figure.
	caption := layout.Caret(5)
	if caption.X < 150 {
		t.Fatalf("the caption starts at %v, not centered", caption.X)
	}
}

func TestFiguresStartLinesInOrder(t *testing.T) {
	flow := testFlow(t, "one\ntwo", 0)
	for _, figures := range [][]surface.Figure{
		{{At: 2, Width: 1, Height: 1}},
		{{At: 4, Width: 1, Height: 1}, {At: 0, Width: 1, Height: 1}},
		{{At: 4, Width: 0, Height: 1}},
	} {
		if _, err := flow.WithFigures(figures); err == nil {
			t.Fatalf("figures %+v were taken", figures)
		}
	}
}
