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
		{At: 4, Node: 9, Media: "m", Width: 200, Height: 100, Caption: 1},
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

// A figure with no caption takes no room for its line, and one opening
// takes its line's room as far as it has opened.
func TestAClosedCaptionTakesNoRoom(t *testing.T) {
	place := func(caption float32) (PlacedFigure, Paragraph) {
		flow, err := testFlow(t, "one\n\uFFFC\nthree", 0).WithFigures([]surface.Figure{
			{At: 4, Node: 9, Media: "m", Width: 200, Height: 100, Caption: caption},
		})
		if err != nil {
			t.Fatal(err)
		}
		layout := NewCache().Update(flow, 400, testStyle())
		return layout.Figures()[0], layout.Paragraphs()[2]
	}
	figure, closed := place(0)
	if below := closed.Top() - closed.gap - (figure.Box.Y + figure.Box.H); below != 0 {
		t.Fatalf("a closed caption keeps %v below its figure", below)
	}
	_, half := place(0.5)
	_, open := place(1)
	if !(closed.Top() < half.Top() && half.Top() < open.Top()) {
		t.Fatalf("the line after sits at %v, %v and %v as the caption opens", closed.Top(), half.Top(), open.Top())
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

// Up and Down step a row at a time across a figure, whatever room it
// takes, its caption open or closed; a closed caption shows no selection.
func TestRowsStepAcrossAFigure(t *testing.T) {
	for _, caption := range []float32{0, 1} {
		flow, err := testFlow(t, "one\n￼\nthree", 0).WithFigures([]surface.Figure{
			{At: 4, Node: 9, Media: "m", Width: 200, Height: 100, Caption: caption},
		})
		if err != nil {
			t.Fatal(err)
		}
		layout := NewCache().Update(flow, 400, testStyle())
		x := layout.Caret(0).X
		if got := layout.RowNear(6, -1, x); got != 4 && got != 5 {
			t.Fatalf("caption %v: Up from below the figure reached %v, not its line", caption, got)
		}
		if got := layout.RowNear(5, -1, x); got != 0 {
			t.Fatalf("caption %v: Up from the figure's line reached %v", caption, got)
		}
		if got := layout.RowNear(0, 1, x); got != 4 && got != 5 {
			t.Fatalf("caption %v: Down to the figure reached %v", caption, got)
		}
		if got := layout.RowNear(5, 1, x); got != 6 {
			t.Fatalf("caption %v: Down from the figure's line reached %v", caption, got)
		}
		// The caption's row, from the figure's foot to the gap above the
		// next line.
		top := layout.Figures()[0].Box.Y + layout.Figures()[0].Box.H
		bottom := layout.Paragraphs()[2].Top() - layout.Paragraphs()[2].gap
		shown := false
		for _, rect := range layout.RangeRects(model.NewUTF16Range(0, 11)) {
			if rect.Y < bottom && rect.Y+rect.H > top && rect.H > 0 {
				shown = true
			}
		}
		if shown != (caption == 1) {
			t.Fatalf("caption %v: the caption's row shows selected: %v", caption, shown)
		}
	}
}
