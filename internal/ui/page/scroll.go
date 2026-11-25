package page

import (
	"time"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/motion"
	"github.com/egoist/mygo/ui"
)

func (el *Element) wheelAcross(x, y float32, ev ui.InputEvent, dx float32) bool {
	pl := el.Editor.layout
	if pl == nil {
		return false
	}
	for i := len(pl.units) - 1; i >= 0; i-- {
		if u := pl.units[i]; u.kind != nil && u.kind.wheel(el, u, x, y, ev, dx) {
			return true
		}
	}
	return false
}

// relayout lays the page out at the element's width.

func (el *Element) ScrollY() float32 {
	if el.restoreScroll != nil {
		return *el.restoreScroll
	}
	return el.scroll
}

// RestoreScroll places the page at a saved offset after its next layout.
// Applying it after layout clamps it to the page's current extent.
func (el *Element) RestoreScroll(y float32) { el.restoreScroll = &y }

func (el *Element) applyRestoredScroll(pl *pageLayout) {
	if el.restoreScroll == nil || el.viewport.W <= 0 || el.viewport.H <= 0 {
		return
	}
	low, high := el.scrollRange(pl)
	el.scroll = max(low, min(*el.restoreScroll, high))
	el.restoreScroll = nil
	el.glide.Cancel()
	el.overscroll.Reset()
	// The restored offset was saved at this viewport height. Mark the
	// height as followed so the next view does not mistake opening the
	// page for a resize and reveal the caret over the saved position.
	el.typewriter, el.viewH = el.Typewriter, el.viewport.H
}

// Still lays the page out at width as a showcase stage draws a story,
// without taking input: the text at the whole width from its top. It
// returns the text's extent: its widest line and its height.

func (el *Element) offset() float32 { return el.scroll - el.overscroll.Offset() }

// scrollRange is how far the page scrolls. In typewriter mode room above
// and below lets the first and last lines sit at the center like every
// other line, so offsets may be negative; otherwise the page scrolls over
// what overflows.
func (el *Element) scrollRange(pl *pageLayout) (float32, float32) {
	if pl == nil || el.viewport.H <= 0 {
		return 0, 0
	}
	if el.Typewriter {
		line := el.Prose.LineHeight()
		half := (el.viewport.H - line) / 2
		first := el.Top - half
		return first, max(first, el.Top+pl.height-line-half)
	}
	return 0, max(0, el.Top+pl.height+el.Bottom-el.viewport.H)
}

// follow keeps the caret's line in view by one rule: the page moves for
// what the writer does, never on its own. It reveals the line after an
// intent, a resize or a change of mode; typewriter mode centers it. What
// the pointer does is a glance (Editor.Glance), which moves no view: the
// next intent of the keyboard reveals the caret where it left it.
// Leaving the mode coasts whatever glide is under way into the plain
// range, at its speed, rather than finishing it and snapping.
func (el *Element) follow(pl *pageLayout, now time.Time) {
	ed := el.Editor
	modeChanged := el.typewriter != el.Typewriter
	resized := el.viewH != el.viewport.H
	el.typewriter, el.viewH = el.Typewriter, el.viewport.H
	if modeChanged && !el.Typewriter {
		low, high := el.scrollRange(pl)
		el.glide.Settle(el.scroll, low, high, now)
		ed.reveal = false
		return
	}
	if !ed.reveal && !resized && !modeChanged {
		return
	}
	ed.reveal = false
	if pc, ok := pl.lookup(ed.Selection().Head); ok && pc.unit.kind != nil {
		pc.unit.kind.reveal(el, pc, now)
	}
	el.revealCaret(pl, now)
}

// revealCaret glides the caret's whole line into view, from where the
// page is and at the speed it moves.
func (el *Element) revealCaret(pl *pageLayout, now time.Time) {
	if el.viewport.H <= 0 {
		return
	}
	r, ok := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if !ok {
		return
	}
	line, y := max(r.H, el.Prose.LineHeight()), el.Top+r.Y
	if !el.Typewriter && y < el.Top+line {
		// The room above the first line is that line's: revealing it
		// reveals the page's top.
		line, y = line+y, 0
	}
	// The line stays clear of the fades at the view's edges, the dock's
	// among them; typewriter mode centers it in the whole view.
	from, view := float32(0), el.viewport.H
	if !el.Typewriter {
		top, bottom := el.edgeFades()
		from, view = top, max(view-top-bottom, line)
	}
	target, changed := motion.Reveal(el.scroll+from, view, y, line, el.Typewriter, now.Before(el.held))
	target -= from
	if !changed {
		return
	}
	low, high := el.scrollRange(pl)
	target = max(low, min(target, high))
	if target == el.scroll && !el.glide.Moving() {
		return
	}
	el.glide.To(el.scroll, target, now)
	el.revealing = true
}

// stepViewport brings the glide and the pull past an edge to now, and
// reports whether either still moves.
func (el *Element) stepViewport(now time.Time, animate bool) bool {
	_, pulling := el.overscroll.Step(now, animate)
	// A range that shrank past the page, as text deleted at its end,
	// leaves it astray: it glides back in once the hand has let it be,
	// rather than leaving the next scroll to find itself past the edge.
	low, high := el.scrollRange(el.Editor.layout)
	if high < el.ranged[1] && el.scroll > high || low > el.ranged[0] && el.scroll < low {
		el.astray = true
	}
	el.ranged = [2]float32{low, high}
	if el.astray && (el.glide.Moving() || el.scroll >= low && el.scroll <= high) {
		el.astray = false
	}
	if el.astray && !pulling {
		if now.Before(el.held) {
			return true
		}
		el.astray = false
		el.glide.Settle(el.scroll, low, high, now)
	}
	var gliding bool
	el.scroll, gliding = el.glide.Step(el.scroll, now, animate)
	return pulling || gliding
}

// scrollBy scrolls by hand: it stops a glide, holds the page against the
// caret a moment, and pulls elastic past an edge.
func (el *Element) scrollBy(ev ui.InputEvent, now time.Time) {
	el.glide.Cancel()
	el.held = now.Add(manualHold)
	low, high := el.scrollRange(el.Editor.layout)
	el.scroll = components.OverscrollEditor.Scroll(&el.overscroll, el.scroll, ev.DY, low, high, el.viewport.H, el.UIFontSize, ev)
}

// --- animation ------------------------------------------------------------

// stepCaret moves the drawn caret toward the selection head's place; it
// reports whether it still moves.
