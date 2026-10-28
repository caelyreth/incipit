package motion

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSpringConvergesWithoutOvershoot(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	spring := NewSpring(0, now)
	previous := float32(0)
	spring.Step(100, now, caretFrequency)
	for frame := 1; frame <= 100; frame++ {
		value := spring.Step(100, now.Add(time.Duration(frame)*16*time.Millisecond), caretFrequency)
		if value < previous || value > 100 {
			t.Fatalf("frame %d value %v after %v", frame, value, previous)
		}
		previous = value
	}
	if previous != 100 || spring.Moving() {
		t.Fatalf("spring did not settle: value=%v moving=%v", previous, spring.Moving())
	}
}

func TestRetargetIsContinuousAndPreservesVelocity(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	spring := NewSpring(0, now)
	spring.Step(100, now, caretFrequency)
	later := now.Add(40 * time.Millisecond)
	uninterrupted := spring
	before := uninterrupted.Step(100, later, caretFrequency)
	if got := spring.Step(60, later, caretFrequency); got != before {
		t.Fatalf("retarget jumped: got %v before %v", got, before)
	}
	after := spring.Step(60, later.Add(10*time.Millisecond), caretFrequency)
	if after <= before {
		t.Fatalf("retarget lost velocity: before %v after %v", before, after)
	}
	if got := spring.Step(60, now.Add(time.Second), caretFrequency); got != 60 {
		t.Fatalf("settled = %v", got)
	}
}

func TestSelectionTopologyReset(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	motion := New(now)
	selection := func(x, y, width float32) ui.Rect { return ui.Rect{X: x, Y: y, W: width, H: 34} }
	caret := selection(0, 0, 2)
	motion.Geometry(caret, []ui.Rect{selection(0, 0, 30)}, now, true)
	_, sampled := motion.Geometry(caret, []ui.Rect{selection(0, 0, 100)}, now, true)
	if sampled[0].W != 30 {
		t.Fatalf("same-top width did not animate: %#v", sampled)
	}
	targets := []ui.Rect{selection(0, 0, 100), selection(0, 34, 40)}
	_, sampled = motion.Geometry(caret, targets, now, true)
	if len(sampled) != 2 || sampled[0] != targets[0] || sampled[1] != targets[1] {
		t.Fatalf("new topology animated across rows: %#v", sampled)
	}
	relocated := []ui.Rect{selection(0, 68, 50)}
	_, sampled = motion.Geometry(caret, relocated, now, true)
	if len(sampled) != 1 || sampled[0] != relocated[0] {
		t.Fatalf("relocated topology animated: %#v", sampled)
	}
}

func TestBreathingHasSolidAndTransparentPhases(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	motion := New(now)
	if got := motion.CaretOpacity(now.Add(200 * time.Millisecond)); got != 1 {
		t.Fatalf("solid = %v", got)
	}
	if got := motion.CaretOpacity(now.Add(765 * time.Millisecond)); math.Abs(float64(got)) > 0.0001 {
		t.Fatalf("transparent = %v", got)
	}
	if got := motion.CaretOpacity(now.Add(1290 * time.Millisecond)); math.Abs(float64(got-1)) > 0.0001 {
		t.Fatalf("opaque = %v", got)
	}
}

func TestRevealAndViewportMotion(t *testing.T) {
	t.Parallel()
	if target, changed := RevealOffset(RevealOptions{
		ViewportTop: 0, ViewportHeight: 400, CaretY: 100, LineHeight: 34, CurrentOffset: 0,
	}); changed || target != 0 {
		t.Fatalf("visible caret moved viewport: %v,%v", target, changed)
	}
	if target, changed := RevealOffset(RevealOptions{
		ViewportTop: 0, ViewportHeight: 400, CaretY: 900, LineHeight: 34,
		CurrentOffset: 20,
	}); !changed || target != 542 {
		t.Fatalf("below-viewport caret target = %v,%v", target, changed)
	}
	if target, changed := RevealOffset(RevealOptions{
		ViewportTop: 0, ViewportHeight: 400, CaretY: 150, LineHeight: 34,
		CurrentOffset: 20, Typewriter: true,
	}); !changed || math.Abs(float64(target+42)) > 0.001 {
		t.Fatalf("typewriter target = %v,%v", target, changed)
	}
	now := time.Unix(0, 0)
	motion := New(now)
	motion.ScrollTo(0, 100, now)
	_, _ = motion.Scroll(now.Add(16*time.Millisecond), true)
	value, ok := motion.Scroll(now.Add(32*time.Millisecond), true)
	if !ok || value <= 0 || value >= 100 {
		t.Fatalf("scroll = %v,%v", value, ok)
	}
}
