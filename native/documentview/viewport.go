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
	line := e.Prose.LineHeight()
	// The product caret is centered in its line; reveal the whole line.
	lineTop := caret - (line-e.Prose.CaretHeight())/2
	// The reading space above the first line is that line's: revealing it
	// reveals the page's top. A caret within the first line's height of
	// the text's top is on it, however tall a heading makes it.
	if lineTop < e.TopInset+line {
		line, lineTop = line+lineTop, 0
	}
	target, changed := motion.RevealOffset(motion.RevealOptions{
		ViewportTop:    e.scroll,
		ViewportHeight: e.lastHeight,
		CaretY:         lineTop,
		LineHeight:     line,
		CurrentOffset:  e.scroll,
		Typewriter:     e.Typewriter,
		ManualOverride: now.Before(e.manualUntil),
	})
	if !changed {
		return
	}
	low, high := e.scrollRange(layout, e.lastHeight)
	target = max(low, min(target, high))
	if target == e.scroll && !e.Motion.Scrolling() {
		return
	}
	e.Motion.ScrollTo(e.scroll, target, now)
}

func (e *Element) advanceScroll(now time.Time) {
	if e == nil || e.Motion == nil {
		return
	}
	value, moving := e.Motion.Scroll(now, e.Animate)
	if moving {
		e.scroll = value
	}
}

func (e *Element) manualScroll(delta float32, now time.Time) {
	if e == nil || e.Motion == nil {
		return
	}
	e.Motion.CancelScroll()
	e.manualUntil = now.Add(manualScrollGrace)
	e.scroll += delta
	if e.current != nil {
		low, high := e.scrollRange(e.current, e.lastHeight)
		e.scroll = max(low, min(e.scroll, high))
	}
}
