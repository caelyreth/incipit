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
	if e.Motion != nil {
		e.Motion.CancelScroll()
	}
	e.manualUntil = time.Time{}
}

// SwitchPage resets presentation identity before a newly mounted document is
// painted. Retained shaping caches remain reusable by content identity.
func (e *Element) SwitchPage(offset float32) {
	if e == nil {
		return
	}
	e.SetScrollY(offset)
	if e.Motion != nil {
		e.Motion.ResetGeometry()
	}
	e.pointerAnchor = 0
	e.pointer = pointerIdle
	e.current = nil
	e.Focus()
}
