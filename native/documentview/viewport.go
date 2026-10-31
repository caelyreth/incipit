package documentview

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

const manualScrollGrace = 650 * time.Millisecond

// revealCaret brings the caret's whole line box into view.
func (e *Element) revealCaret(layout *textlayout.Layout, box ui.Rect, now time.Time) {
	if e == nil || layout == nil || e.lastHeight <= 0 {
		return
	}
	line, lineTop := max(box.H, e.Prose.LineHeight()), box.Y
	// The reading space above the first line is that line's: revealing it
	// reveals the page's top. A caret within the first line's height of
	// the text's top is on it, however tall larger text makes it.
	if lineTop < e.TopInset+line {
		line, lineTop = line+lineTop, 0
	}
	viewportHeight := e.lastHeight
	if !e.Typewriter {
		// The floating writing dock covers the bottom inset. Reveal only
		// within the room above it, not merely inside the component box.
		viewportHeight = max(0, viewportHeight-e.BottomInset)
	}
	target, changed := motion.RevealOffset(motion.RevealOptions{
		ViewportTop:    e.scroll,
		ViewportHeight: viewportHeight,
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

func (e *Element) manualScroll(delta float32, now time.Time, precise, momentum bool) {
	if e == nil || e.Motion == nil {
		return
	}
	e.Motion.CancelScroll()
	e.manualUntil = now.Add(manualScrollGrace)
	style := components.OverscrollEditor
	input := scrollfx.Direct
	switch {
	case momentum:
		input = scrollfx.Momentum
	case !precise:
		input = scrollfx.Notch
	}
	if e.overscroll.Offset() != 0 {
		style.Add(&e.overscroll, -delta, e.lastHeight, e.UIFontSize, now, input)
		return
	}
	next := e.scroll + delta
	if e.current != nil {
		low, high := e.scrollRange(e.current, e.lastHeight)
		switch {
		case next < low:
			style.Add(&e.overscroll, low-next, e.lastHeight, e.UIFontSize, now, input)
			next = low
		case next > high:
			style.Add(&e.overscroll, high-next, e.lastHeight, e.UIFontSize, now, input)
			next = high
		}
	}
	e.scroll = next
}
