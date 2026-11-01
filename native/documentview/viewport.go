package documentview

import (
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

const manualScrollGrace = 650 * time.Millisecond

const (
	// edgeRate is how fast a drag past the page's edge scrolls it, in DIPs
	// a second for each DIP past, and edgeFastest the most.
	edgeRate    = 12
	edgeFastest = 2400
)

// revealKey is what the caret is revealed for: the text and the selection.
type revealKey struct {
	text      string
	selection controller.UTF16Range
	reversed  bool
}

func (e *Element) revealKeyOf(controllerValue *controller.DocumentController) revealKey {
	selection := controllerValue.Selection()
	return revealKey{text: controllerValue.Text(), selection: selection.Range, reversed: selection.Reversed}
}

// followCaret reveals the caret's line as its text or selection changes,
// the viewport resizes or typewriter mode turns on, and in typewriter mode
// as its line moves at all, but never under a held pointer: what a press
// or a drag changes is revealed as it lets go. Room opening by the caret,
// as a table's for a row, moves its line and is no reason to scroll.
func (e *Element) followCaret(layout *textlayout.Layout, line ui.Rect, caretY, height float32, key revealKey, now time.Time) {
	modeChanged := e.lastTypewriter != e.Typewriter
	moved := e.lastCaretY != caretY
	resized := e.lastViewportH != height
	e.lastCaretY, e.lastViewportH, e.lastTypewriter = caretY, height, e.Typewriter
	if key != e.revealed {
		e.revealed, e.revealPending = key, true
	}
	if modeChanged && !e.Typewriter {
		// Leaving typewriter mode redirects whatever glide is under way,
		// at its speed, into the plain range, rather than letting it
		// reach a centered offset and then snapping into range.
		low, high := e.scrollRange(layout, height)
		e.Motion.Settle(e.scroll, low, high, now)
		return
	}
	if e.pointer != pointerIdle {
		return
	}
	if e.revealPending || resized || modeChanged || e.Typewriter && moved {
		e.revealPending = false
		// Entering typewriter mode glides to the centered line from where
		// the page is and at the speed it is moving.
		e.revealCaret(layout, line, now)
	}
}

// stepViewport brings the page's scroll and its pull past an edge to now,
// and reports whether either still moves. Stepped again at the same time
// it holds, so the view and the painting of one frame agree.
func (e *Element) stepViewport(now time.Time, animate bool) bool {
	_, pulling := e.overscroll.Step(now, animate)
	e.advanceScroll(now)
	return pulling || e.Motion != nil && e.Motion.Scrolling()
}

// followDragEdge scrolls the page while a drag selecting text is held past
// the room it reads in, above the top or down by the writing dock, faster
// the further past, the selection's head following the pointer over what
// comes into view.
func (e *Element) followDragEdge(c *ui.Context) {
	bridge, _ := e.Input.(*Bridge)
	if e.pointer != pointerSelecting || e.current == nil || e.Motion == nil || bridge != nil && bridge.Cells != nil {
		e.edgeAt = time.Time{}
		return
	}
	zone := e.Prose.LineHeight() / 2
	bottom := e.lastHeight
	if !e.Typewriter {
		bottom -= e.BottomInset
	}
	past := float32(0)
	switch y := e.pointerAt.Y; {
	case y < zone:
		past = y - zone
	case y > bottom-zone:
		past = y - (bottom - zone)
	}
	if past == 0 {
		e.edgeAt = time.Time{}
		return
	}
	now := c.Now()
	c.AnimationFrame()
	if e.edgeAt.IsZero() {
		e.edgeAt = now
		return
	}
	elapsed := float32(now.Sub(e.edgeAt).Seconds())
	e.edgeAt = now
	speed := max(-edgeFastest, min(past*edgeRate, edgeFastest))
	low, high := e.scrollRange(e.current, e.lastHeight)
	next := max(low, min(e.scroll+speed*elapsed, high))
	if next == e.scroll {
		return
	}
	e.Motion.CancelScroll()
	e.scroll = next
	if head := e.hit(e.pointerAt.X, e.pointerAt.Y); head != e.pointerHead {
		e.pointerHead = head
		_ = e.Input.Select(e.pointerAnchor, head, now)
	}
}

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
	e.overscroll.Observe(input)
	if e.overscroll.Offset() != 0 {
		// What passes back through the edge scrolls the page.
		delta = -style.Add(&e.overscroll, -delta, e.lastHeight, e.UIFontSize, input)
		if delta == 0 {
			return
		}
	}
	next := e.scroll + delta
	if e.current != nil {
		low, high := e.scrollRange(e.current, e.lastHeight)
		push := func(beyond float32) {
			if input == scrollfx.Momentum {
				// Inertia pushes by its speed, the whole event.
				beyond = -delta
			}
			style.Add(&e.overscroll, beyond, e.lastHeight, e.UIFontSize, input)
		}
		switch {
		case next < low:
			push(low - next)
			next = low
		case next > high:
			push(high - next)
			next = high
		}
	}
	e.scroll = next
}
