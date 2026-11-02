package documentview

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

const manualScrollGrace = 650 * time.Millisecond

// revealSettle is how long the caret's line is followed after it is
// revealed, as a row or a caption an edit opened comes in.
const revealSettle = 450 * time.Millisecond

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

// followCaret keeps the caret's line in view, in either mode, by one
// rule: the page moves for what the writer does, never for what the page
// does on its own. It reveals the line as its text or selection changes,
// the viewport resizes or the mode changes, and follows it while what that
// change opened settles, as a new row growing; typewriter mode centers it
// where the plain mode keeps it in view. Room the page opens by the
// pointer, as a table's for a row, moves the line and is no reason to
// scroll: in typewriter mode the line stays off center until the writer
// next acts. Nothing moves under a held pointer, or while the pointer
// stays by a control it used; what changed meanwhile is revealed after.
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
	if e.pointer != pointerIdle || e.pointerHold || e.carry != nil {
		e.followUntil = time.Time{}
		return
	}
	switch {
	case e.revealPending || resized || modeChanged:
		e.revealPending = false
		e.followUntil = now.Add(revealSettle)
	case moved && now.Before(e.followUntil) && e.liftTo == 0:
	default:
		return
	}
	// Entering typewriter mode glides to the centered line from where the
	// page is and at the speed it is moving.
	e.revealCaret(layout, line, now)
}

// applyLift scrolls the page by what the room opening about the pointer
// asks since the last frame, so what is about the pointer stays under it,
// carrying a glide under way along.
func (e *Element) applyLift() {
	if delta := e.liftTo - e.lift; delta != 0 {
		e.lift = e.liftTo
		e.scroll += delta
		if e.Motion != nil {
			e.Motion.Shift(delta)
		}
	}
}

// pagePointer is where the pointer is in the page's content, and whether
// it is over the page: the column, and the margins beside it, where the
// page's grips and gutters are.
func (e *Element) pagePointer(box ui.Element) (ui.Point, bool) {
	x, y, inside := box.PointerPosition()
	if !inside && e.current != nil && y >= 0 && y < e.lastHeight &&
		x >= -e.Margin && x <= e.current.Width()+e.Margin {
		inside = true
	}
	return e.contentPoint(x, y), inside
}

// control notes a control set over the page at r, in the element's space.
func (e *Element) control(r ui.Rect) {
	e.controls = append(e.controls, r)
}

// onControl reports whether a point of the element is on a control set
// over the page, whose presses are its own.
func (e *Element) onControl(x, y float32) bool {
	for _, control := range e.controls {
		if contains(control, ui.Point{X: x, Y: y}) {
			return true
		}
	}
	return false
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
	selecting := e.pointer == pointerSelecting && (bridge == nil || bridge.Cells == nil)
	if !selecting && e.carry == nil || e.current == nil || e.Motion == nil {
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
	if !selecting {
		return
	}
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
	low, high := float32(math.Inf(-1)), float32(math.Inf(1))
	if e.current != nil {
		low, high = e.scrollRange(e.current, e.lastHeight)
	}
	e.scroll = elasticScroll(&e.overscroll, e.scroll, delta, low, high, e.lastHeight, e.UIFontSize, scrollfx.Phase(precise, momentum))
}

// elasticScroll scrolls from position by delta within low and high, the
// page's and a wide table's alike: what would pass an edge pulls it,
// elastic, a viewport's share at most, and what passes back through the
// edge scrolls on. It returns where it scrolls to.
func elasticScroll(elastic *scrollfx.Elastic, position, delta, low, high, viewport, uiFontSize float32, input scrollfx.InputPhase) float32 {
	return components.OverscrollEditor.Scroll(elastic, position, delta, low, high, viewport, uiFontSize, input)
}
