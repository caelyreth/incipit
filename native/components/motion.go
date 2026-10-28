package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	MotionImmediate = 60 * time.Millisecond
	MotionQuick     = 120 * time.Millisecond
	MotionNormal    = 200 * time.Millisecond
	MotionPanel     = 240 * time.Millisecond
	MotionScene     = 450 * time.Millisecond
)

// Decelerate is the panel curve: a cubic ease-out that moves immediately and
// settles softly, so sidebar and chrome changes answer the click at once.
func Decelerate(t float32) float32 {
	t = max(0, min(t, 1))
	u := 1 - t
	return 1 - u*u*u
}

// Smoothstep is Neverland's bounded chrome interpolation:
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
	return element.AnimateWith("hover", target, duration, Smoothstep)
}

func ChromeAnimation(duration time.Duration) ui.Animation {
	return ui.Animation{Duration: duration, Ease: Smoothstep}
}
