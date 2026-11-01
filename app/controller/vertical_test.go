package controller

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
)

type wrapRow struct {
	top     float32
	height  float32
	offsets []int
	xs      []float32
}

type wrapGeometry struct {
	text      string
	rows      []wrapRow
	lastPoint Point
}

func (g *wrapGeometry) BoundsForRange(
	_ string,
	range16 UTF16Range,
) (Rect, UTF16Range, bool) {
	if range16.Start < 0 || range16.End < range16.Start {
		return Rect{}, range16, false
	}
	return Rect{X: float32(range16.Start), W: 1, H: 1}, range16, true
}

func (g *wrapGeometry) IndexForPoint(_ string, point Point) (int, bool) {
	g.lastPoint = point
	if point.Y < 0 {
		point.Y = 0
	}
	for _, row := range g.rows {
		if point.Y < row.top || point.Y >= row.top+row.height {
			continue
		}
		best := row.offsets[0]
		distance := float32(0)
		for index, x := range row.xs {
			candidate := x - point.X
			if candidate < 0 {
				candidate = -candidate
			}
			if index == 0 || candidate < distance {
				best, distance = row.offsets[index], candidate
			}
		}
		return best, true
	}
	return len(g.text), true
}

func (g *wrapGeometry) RowIndex(text string, caret int, direction VerticalDirection, x float32) (int, bool) {
	for index, row := range g.rows {
		if !slices.Contains(row.offsets, caret) {
			continue
		}
		switch next := index + int(direction); {
		case next < 0:
			return g.IndexForPoint(text, Point{X: x, Y: row.top + row.height/2})
		case next >= len(g.rows):
			return len(g.text), true
		default:
			return g.IndexForPoint(text, Point{X: x, Y: g.rows[next].top + g.rows[next].height/2})
		}
	}
	return 0, false
}

func (g *wrapGeometry) CaretLine(_ string, caret int) (Rect, bool) {
	for _, row := range g.rows {
		for index, offset := range row.offsets {
			if offset == caret {
				return Rect{X: row.xs[index], Y: row.top, W: 1, H: row.height}, true
			}
		}
	}
	return Rect{}, false
}

func TestVerticalMovementTracksWrappedRowsAndPreferredColumn(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abcdef")
	geometry := &wrapGeometry{
		text: "abcdef",
		rows: []wrapRow{
			{top: 0, height: 10, offsets: []int{0, 1, 2}, xs: []float32{0, 10, 20}},
			{top: 10, height: 10, offsets: []int{3, 4, 5}, xs: []float32{0, 7, 14}},
		},
	}
	controller.SetGeometry(geometry)
	mustSelect(t, controller, 2, 2)

	if err := controller.MoveVertical(VerticalDown, false); err != nil {
		t.Fatal(err)
	}
	if controller.Caret() != 5 || geometry.lastPoint != (Point{X: 20, Y: 15}) {
		t.Fatalf("down caret=%d point=%#v", controller.Caret(), geometry.lastPoint)
	}

	if err := controller.MoveVertical(VerticalUp, false); err != nil {
		t.Fatal(err)
	}
	if controller.Caret() != 2 || geometry.lastPoint != (Point{X: 20, Y: 5}) {
		t.Fatalf("preferred-x up caret=%d point=%#v", controller.Caret(), geometry.lastPoint)
	}

	if err := controller.MoveVertical(VerticalUp, false); err != nil {
		t.Fatal(err)
	}
	if controller.Caret() != 2 {
		t.Fatalf("top boundary caret=%d", controller.Caret())
	}
}

func TestVerticalSelectionAndPreferredColumnReset(t *testing.T) {
	t.Parallel()
	controller := newController(t, "abcdef")
	geometry := &wrapGeometry{
		text: "abcdef",
		rows: []wrapRow{
			{top: 0, height: 10, offsets: []int{0, 1, 2}, xs: []float32{0, 10, 20}},
			{top: 10, height: 10, offsets: []int{3, 4, 5}, xs: []float32{0, 7, 14}},
		},
	}
	controller.SetGeometry(geometry)

	mustSelect(t, controller, 1, 1)
	if err := controller.MoveVertical(VerticalDown, true); err != nil {
		t.Fatal(err)
	}
	if selection := controller.Selection(); selection.Range != (UTF16Range{Start: 1, End: 4}) ||
		selection.Reversed {
		t.Fatalf("extended selection = %#v", selection)
	}
	mustSelect(t, controller, 4, 4)
	if err := controller.MoveVertical(VerticalUp, true); err != nil {
		t.Fatal(err)
	}
	if selection := controller.Selection(); selection.Range != (UTF16Range{Start: 1, End: 4}) ||
		!selection.Reversed || controller.Caret() != 1 {
		t.Fatalf("reverse extended selection = %#v caret=%d", selection, controller.Caret())
	}

	mustSelect(t, controller, 2, 2)
	if err := controller.MoveVertical(VerticalDown, false); err != nil {
		t.Fatal(err)
	}
	if geometry.lastPoint.X != 20 {
		t.Fatalf("captured vertical column = %v", geometry.lastPoint.X)
	}
	if _, err := controller.Execute(Command{ID: string(editor.TextDocumentStart.ID())}); err != nil {
		t.Fatal(err)
	}
	if err := controller.MoveVertical(VerticalDown, false); err != nil {
		t.Fatal(err)
	}
	if controller.Caret() != 3 || geometry.lastPoint.X != 0 {
		t.Fatalf("reset column caret=%d point=%#v", controller.Caret(), geometry.lastPoint)
	}
}
