package documentview

import (
	"time"

	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
)

const manualScrollGrace = 650 * time.Millisecond

func (e *Element) revealCaret(layout *textlayout.Layout, caret float32, now time.Time) {
	if e == nil || layout == nil || e.lastHeight <= 0 {
		return
	}
	target, changed := motion.RevealOffset(motion.RevealOptions{
		ViewportTop:    e.scroll,
		ViewportHeight: e.lastHeight,
		CaretY:         caret,
		LineHeight:     e.Prose.LineHeight(),
		CurrentOffset:  e.scroll,
		Typewriter:     e.Typewriter,
		ManualOverride: now.Before(e.manualUntil),
	})
	if !changed {
		return
	}
	contentHeight := layout.Height() + e.contentTop() + e.BottomInset
	maxOffset := max(0, contentHeight-e.lastHeight)
	target = max(0, min(target, maxOffset))
	e.Motion.ScrollTo(e.scroll, target, now)
}

func (e *Element) advanceScroll(now time.Time) {
	if e == nil || e.Motion == nil {
		return
	}
	value, moving := e.Motion.Scroll(now, e.Animate)
	if moving {
		e.scroll = max(0, value)
	}
}

func (e *Element) manualScroll(delta float32, now time.Time) {
	if e == nil || e.Motion == nil {
		return
	}
	e.Motion.CancelScroll()
	e.manualUntil = now.Add(manualScrollGrace)
	e.scroll = max(0, e.scroll+delta)
}
