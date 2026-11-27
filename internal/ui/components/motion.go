package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Motion takes one of four durations, by how much moves: a press, a tint
// or a hover; a control; a panel, a popover, a fold or a column; the
// scene. Timers that wait for the hand, as a dwell or a double press,
// are not motion and keep their own.
const (
	MotionQuick  = 90 * time.Millisecond
	MotionNormal = 200 * time.Millisecond
	MotionPanel  = 280 * time.Millisecond
	MotionScene  = 420 * time.Millisecond
	// settleBounce is how far a Settle goes past its place and back: a
	// damping ratio of 0.75, about three hundredths of the way.
	settleBounce = 0.25
)

// MotionEnabled is the app's effective motion policy for a window: visible
// windows keep animating even when unfocused or inactive so blur-driven
// transitions run smoothly; only disabled motion, a hidden window, or OS
// Reduce Motion suppresses animations.
func MotionEnabled(c *ui.Context, enabled bool) bool {
	return c != nil && enabled && !c.WindowHidden() && !c.Preferences().ReduceMotion
}

// Smoothstep is Incipit's bounded chrome interpolation:
// 3t^2 - 2t^3. It settles exactly at the deadline and never overshoots.
func Smoothstep(t float32) float32 {
	t = max(0, min(t, 1))
	return t * t * (3 - 2*t)
}

const (
	// HoverIn is how fast a hover tint answers the pointer; HoverOut how
	// gently it lets go, so sweeping down a list leaves a soft trail rather
	// than flicker. Both stay quicker than any selection motion.
	HoverIn  = MotionQuick
	HoverOut = MotionNormal
)

// Settle is how what travels moves, its place, size or scale: a spring
// that leaves at once and comes to its place with a small, magnetic
// overshoot, about settle long. A new target on the way takes over at the
// value's speed, so what is interrupted carries on from where it is.
func Settle(settle time.Duration) ui.Animation {
	return ui.Animation{Spring: ui.SpringOptions{Duration: settle, Bounce: settleBounce}}
}

// Glide is how what fades, tints or grows in place moves: a spring that
// never overshoots, as a color or an opacity past its end means nothing.
func Glide(settle time.Duration) ui.Animation {
	return ui.Animation{Spring: ui.SpringOptions{Duration: settle}}
}

// SettleEase is t of the way along a Settle as a curve, for what arrives
// a short way on a clock of its own, as the field's cards coming in:
// quick to leave, a little past its place, and back at 1.
func SettleEase(t float32) float32 {
	t = max(0, min(t, 1))
	const c1 = 0.9
	u := t - 1
	return 1 + (c1+1)*u*u*u + c1*u*u
}

// GlideEase is t of the way along a Glide as a curve, for what travels
// far on a clock of its own: quick to leave and landing softly, never
// past its place, where a settle's overshoot would carry it a long way.
func GlideEase(t float32) float32 {
	t = max(0, min(t, 1))
	u := 1 - t
	return 1 - u*u*u
}

// Follow returns element's value under key gliding to target.
func Follow(element ui.Element, key any, target float32, settle time.Duration) float32 {
	return element.AnimateTo(key, target, Glide(settle))
}

// HoverAmount is element's hover tint from 0 to 1, eased in quickly and
// out gently when animate is set.
func HoverAmount(element ui.Element, hovered, animate bool) float32 {
	target := float32(0)
	if hovered {
		target = 1
	}
	if !animate {
		return target
	}
	duration := HoverOut
	if hovered {
		duration = HoverIn
	}
	return Follow(element, "hover", target, duration)
}

// rowSink is the most a row pressed narrows by, in DIPs, half of it each
// side: well within the press a drag starts past.
const rowSink = 3

// PressSink sinks a row while it is pressed, eased when animate is set, as
// the field's rows and the settings' choices give under the pointer. A
// row sinks as far whatever its width: a ratio would sink a list's divider,
// as wide as the list, past where the press is taken for a drag, and its
// click would be lost.
func PressSink(el ui.Element, animate bool) {
	press := float32(0)
	if el.Pressed() {
		press = 1
	}
	if animate {
		press = Follow(el, "press", press, MotionQuick)
	}
	if press > 0.001 {
		w := max(el.LayoutBounds().W, 1)
		sink := 1 - min(0.012, rowSink/w)*press
		el.Scale(sink, sink)
	}
}
