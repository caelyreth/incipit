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
	e.scroll = max(0, offset)
	if e.Motion != nil {
		e.Motion.CancelScroll()
	}
	e.manualUntil = time.Time{}
}
