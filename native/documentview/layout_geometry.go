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
	Layout  *textlayout.Layout
	YOffset float32
	ScrollY float32
}

var _ controller.Geometry = LayoutGeometry{}

func (g LayoutGeometry) BoundsForRange(
	text string,
	range16 controller.UTF16Range,
) (controller.Rect, controller.UTF16Range, bool) {
	if g.Layout == nil {
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
	rect, ok := g.Layout.BoundsForRange(aligned)
	if !ok {
		return controller.Rect{}, controller.UTF16Range{
			Start: int(aligned.Start),
			End:   int(aligned.End),
		}, false
	}
	return controller.Rect{
			X: rect.X,
			Y: rect.Y + g.YOffset - g.ScrollY,
			W: rect.W,
			H: rect.H,
		},
		controller.UTF16Range{Start: int(aligned.Start), End: int(aligned.End)},
		true
}

func (g LayoutGeometry) IndexForPoint(
	_ string,
	point controller.Point,
) (int, bool) {
	if g.Layout == nil {
		return 0, false
	}
	return int(g.Layout.Hit(ui.Point{
		X: point.X,
		Y: point.Y + g.ScrollY - g.YOffset,
	})), true
}
