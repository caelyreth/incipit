package documentview

import "time"

// ScrollY returns the retained vertical viewport offset.
func (e *Element) ScrollY() float32 {
	if e == nil {
		return 0
	}
	return e.scroll
}

// SetScrollY restores a page's retained viewport without animating it.
func (e *Element) SetScrollY(offset float32) {
	if e == nil {
		return
	}
	// The element clamps it into the mode's range when it next paints.
	e.scroll = offset
	e.overscroll.Reset()
	if e.Motion != nil {
		e.Motion.CancelScroll()
	}
	e.manualUntil = time.Time{}
}

// OverscrollY is the transient pull past the viewport edge. Unlike ScrollY,
// it is visual feedback and is never saved.
func (e *Element) OverscrollY() float32 {
	if e == nil {
		return 0
	}
	return e.overscroll.Offset()
}

// SwitchPage detaches the old input and cancels its transient presentation
// before a newly mounted document is painted. Retained shaping caches remain
// reusable by content identity.
func (e *Element) SwitchPage(offset float32) {
	if e == nil {
		return
	}
	if input, ok := e.Input.(interface{ Detach() }); ok {
		input.Detach()
	}
	e.Input = nil
	e.pageState = pageState{}
	e.pageEpoch++
	e.SetScrollY(offset)
	if e.Motion != nil {
		e.Motion.ResetGeometry()
	}
	e.Focus()
}
