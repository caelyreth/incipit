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
	// MotionSettle is how long a panel, a fold or the chrome takes to settle
	// on its spring: a spring's tail is long and soft, so it reads about as
	// quick as MotionPanel's curve.
	MotionSettle = 340 * time.Millisecond
	// MotionMode is how long a window takes to go from one way of laying
	// out to another: the scenes crossing, what they share flying across,
	// and what floats over both moving with them (scene.go).
	MotionMode = 420 * time.Millisecond
	// MotionDock is how long a dock takes to grow, shrink or move as its
	// groups come and go: in step with the scene changing around it.
	MotionDock = MotionMode
	// bounce is how far past its end a Bouncy spring goes, as a share.
	bounce = 0.2
)

// MotionEnabled is the app's effective motion policy for a window. MyGo
// handles reduced motion in its own animations, while custom painting also
// needs the window's active state and the app's preference.
func MotionEnabled(c *ui.Context, enabled, active bool) bool {
	return c != nil && enabled && active && !c.Preferences().ReduceMotion
}

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

// Spring is how every state-driven value in the app moves: a spring that
// arrives in about settle without overshooting. Changing its target while
// it moves takes over at the value's current speed, so a toggle pressed
// again mid-flight, or any state change that interrupts another, carries
// on smoothly from where things are instead of restarting a curve.
func Spring(settle time.Duration) ui.Animation {
	return ui.Animation{Spring: ui.SpringOptions{Duration: settle}}
}

// Bouncy is a spring that settles in about settle, going a little past
// its end and back: for what moves of its own accord and should feel
// alive doing it, as a dock growing or a column coming in, not for what
// follows the hand.
func Bouncy(settle time.Duration) ui.Animation {
	return ui.Animation{Spring: ui.SpringOptions{Duration: settle, Bounce: bounce}}
}

// Follow returns element's value under key following target on Spring.
func Follow(element ui.Element, key any, target float32, settle time.Duration) float32 {
	return element.AnimateTo(key, target, Spring(settle))
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
