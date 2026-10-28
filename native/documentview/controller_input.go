package documentview

import (
	"github.com/caelyreth/neverland/app/controller"
	"github.com/egoist/mygo/ui"
)

// NativeTextClient adapts the UI-neutral controller input contract to MyGo's
// TextInputClient. The controller keeps the authoritative text, selection,
// marked range, and policy. This adapter only converts geometry/range types.
type NativeTextClient struct {
	Controller *controller.DocumentController
}

var _ ui.TextInputClient = (*NativeTextClient)(nil)

func (c *NativeTextClient) TextForRange(r ui.TextInputRange) (string, ui.TextInputRange) {
	if c == nil || c.Controller == nil {
		return "", ui.TextInputRange{}
	}
	text, actual := c.Controller.TextForRange(controller.UTF16Range{Start: r.Start, End: r.End})
	return text, ui.TextInputRange{Start: actual.Start, End: actual.End}
}

func (c *NativeTextClient) Selection() ui.TextInputSelection {
	if c == nil || c.Controller == nil {
		return ui.TextInputSelection{}
	}
	selection := c.Controller.Selection()
	return ui.TextInputSelection{
		Range: ui.TextInputRange{
			Start: selection.Range.Start,
			End:   selection.Range.End,
		},
		Reversed: selection.Reversed,
	}
}

func (c *NativeTextClient) MarkedRange() (ui.TextInputRange, bool) {
	if c == nil || c.Controller == nil {
		return ui.TextInputRange{}, false
	}
	marked, ok := c.Controller.MarkedRange()
	if !ok {
		return ui.TextInputRange{}, false
	}
	return ui.TextInputRange{Start: marked.Start, End: marked.End}, true
}

func (c *NativeTextClient) ReplaceText(r *ui.TextInputRange, text string) {
	if c == nil || c.Controller == nil {
		return
	}
	var converted *controller.UTF16Range
	if r != nil {
		value := controller.UTF16Range{Start: r.Start, End: r.End}
		converted = &value
	}
	c.Controller.ReplaceText(converted, text)
}

func (c *NativeTextClient) SetMarkedText(
	r *ui.TextInputRange,
	text string,
	selected ui.TextInputRange,
) {
	if c == nil || c.Controller == nil {
		return
	}
	var converted *controller.UTF16Range
	if r != nil {
		value := controller.UTF16Range{Start: r.Start, End: r.End}
		converted = &value
	}
	c.Controller.SetMarkedText(
		converted,
		text,
		controller.UTF16Range{Start: selected.Start, End: selected.End},
	)
}

func (c *NativeTextClient) UnmarkText() {
	if c != nil && c.Controller != nil {
		c.Controller.UnmarkText()
	}
}

func (c *NativeTextClient) BoundsForRange(
	r ui.TextInputRange,
) (ui.Rect, ui.TextInputRange, bool) {
	if c == nil || c.Controller == nil {
		return ui.Rect{}, ui.TextInputRange{}, false
	}
	bounds, actual, ok := c.Controller.BoundsForRange(
		controller.UTF16Range{Start: r.Start, End: r.End},
	)
	if !ok {
		return ui.Rect{}, ui.TextInputRange{}, false
	}
	return ui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: bounds.H},
		ui.TextInputRange{Start: actual.Start, End: actual.End},
		true
}

func (c *NativeTextClient) IndexForPoint(point ui.Point) (int, bool) {
	if c == nil || c.Controller == nil {
		return 0, false
	}
	return c.Controller.IndexForPoint(controller.Point{X: point.X, Y: point.Y})
}
