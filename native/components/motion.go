package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	MotionImmediate = 60 * time.Millisecond
	MotionQuick     = 120 * time.Millisecond
	MotionNormal    = 200 * time.Millisecond
	MotionPanel     = 220 * time.Millisecond
	MotionScene     = 450 * time.Millisecond
)

// Smoothstep is Neverland's bounded chrome interpolation:
// 3t^2 - 2t^3. It settles exactly at the deadline and never overshoots.
func Smoothstep(t float32) float32 {
	t = max(0, min(t, 1))
	return t * t * (3 - 2*t)
}

func ChromeAnimation(duration time.Duration) ui.Animation {
	return ui.Animation{Duration: duration, Ease: Smoothstep}
}
