package documentview

import (
	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

// LayoutGeometry adapts a retained MyGo text layout to the controller's
// geometry contract. It uses the visible controller text for range validation.
type LayoutGeometry struct {
	Layout *textlayout.Layout
	// YOffset is where the layout's top lies in the unscrolled viewport.
	YOffset float32
	ScrollY float32
	// Caret is the caret geometry painted for the current layout snapshot.
	// Paint and the native input method must share one presentation, including
	// the hint position used by an empty figure caption.
	Caret *CaretPresentation
}

// CaretPresentation is the painted caret geometry for one collapsed
// selection. Rect uses the layout's unscrolled coordinates.
type CaretPresentation struct {
	Text      string
	Selection controller.UTF16Range
	Rect      controller.Rect
}

var _ controller.Geometry = LayoutGeometry{}
var _ controller.VerticalGeometry = LayoutGeometry{}

func (g LayoutGeometry) coordinates() pageCoordinates {
	return pageCoordinates{top: g.YOffset, scroll: g.ScrollY}
}

func (g LayoutGeometry) BoundsForRange(
	text string,
	range16 controller.UTF16Range,
) (controller.Rect, controller.UTF16Range, bool) {
	if g.Layout == nil {
		return controller.Rect{}, range16, false
	}
	if g.Layout.Flow().Text() != text {
		return controller.Rect{}, range16, false
	}
	aligned, _, ok := model.AlignUTF16RangeForNativeQuery(
		text,
		model.NewUTF16Range(
			model.UTF16Offset(range16.Start),
			model.UTF16Offset(range16.End),
		),
	)
	if !ok {
		return controller.Rect{}, range16, false
	}
	if g.Caret != nil && range16 == g.Caret.Selection &&
		g.Caret.Text == text && range16.Empty() {
		rect := g.Caret.Rect
		rect.Y = g.coordinates().viewportY(rect.Y)
		return rect, range16, true
	}
	rect, ok := g.Layout.BoundsForRange(aligned)
	if !ok {
		return controller.Rect{}, controller.UTF16Range{
			Start: int(aligned.Start),
			End:   int(aligned.End),
		}, false
	}
	return controller.Rect{
			X: rect.X,
			Y: g.coordinates().viewportY(rect.Y),
			W: rect.W,
			H: rect.H,
		},
		controller.UTF16Range{Start: int(aligned.Start), End: int(aligned.End)},
		true
}

func (g LayoutGeometry) IndexForPoint(
	text string,
	point controller.Point,
) (int, bool) {
	if g.Layout == nil || g.Layout.Flow().Text() != text {
		return 0, false
	}
	return int(g.Layout.Hit(g.coordinates().contentPoint(ui.Point{
		X: point.X,
		Y: point.Y,
	}))), true
}

func (g LayoutGeometry) RowIndex(
	text string,
	caret int,
	direction controller.VerticalDirection,
	x float32,
) (int, bool) {
	if g.Layout == nil || g.Layout.Flow().Text() != text || caret < 0 ||
		!model.UTF16OffsetIsBoundary(text, model.UTF16Offset(caret)) {
		return 0, false
	}
	return int(g.Layout.RowNear(model.UTF16Offset(caret), int(direction), x)), true
}

func (g LayoutGeometry) CaretLine(text string, caret int) (controller.Rect, bool) {
	if g.Layout == nil || g.Layout.Flow().Text() != text || caret < 0 ||
		!model.UTF16OffsetIsBoundary(text, model.UTF16Offset(caret)) {
		return controller.Rect{}, false
	}
	line, ok := g.Layout.CaretLine(model.UTF16Offset(caret))
	if !ok {
		return controller.Rect{}, false
	}
	return controller.Rect{
		X: line.X,
		Y: g.coordinates().viewportY(line.Y),
		W: line.W,
		H: line.H,
	}, true
}
