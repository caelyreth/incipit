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

// Rows a selection gains grow out of the selection as drawn: each starts
// as the drawn row nearest it, then settles at its own place.
func TestSelectionRowsGrowOutOfTheDrawnSelection(t *testing.T) {
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
	targets := []ui.Rect{selection(0, 0, 100), selection(0, 34, 40), selection(0, 68, 70)}
	_, sampled = motion.Geometry(caret, targets, now, true)
	if len(sampled) != 3 || sampled[1] != sampled[0] || sampled[2] != sampled[0] {
		t.Fatalf("new rows did not start from the drawn row: %#v", sampled)
	}
	later := now
	for range 120 {
		later = later.Add(16 * time.Millisecond)
		_, sampled = motion.Geometry(caret, targets, later, true)
	}
	for index := range targets {
		if sampled[index] != targets[index] {
			t.Fatalf("rows did not settle: %#v", sampled)
		}
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
	}); !changed || math.Abs(float64(target+33)) > 0.001 {
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

func TestSelectionFadesInOnlyWhenAppearing(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	motion := New(now)
	row := func(width float32) ui.Rect { return ui.Rect{W: width, H: 34} }
	caret := ui.Rect{W: 2, H: 25}
	motion.Geometry(caret, nil, now, true)
	if got := motion.SelectionOpacity(); got != 1 || motion.Moving() {
		t.Fatalf("empty selection opacity = %v moving = %v", got, motion.Moving())
	}

	motion.Geometry(caret, []ui.Rect{row(30)}, now, true)
	if got := motion.SelectionOpacity(); got != 0 || !motion.Moving() {
		t.Fatalf("appearing opacity = %v moving = %v", got, motion.Moving())
	}
	half := now.Add(SelectionFade / 2)
	motion.Geometry(caret, []ui.Rect{row(60)}, half, true)
	if got := motion.SelectionOpacity(); got <= 0.5 || got >= 1 {
		t.Fatalf("ease-out midpoint opacity = %v", got)
	}
	motion.Geometry(caret, []ui.Rect{row(60), {Y: 34, W: 20, H: 34}}, half, true)
	if got := motion.SelectionOpacity(); got >= 1 {
		t.Fatalf("growing across rows restarted or ended the fade: %v", got)
	}
	done := now.Add(SelectionFade)
	motion.Geometry(caret, []ui.Rect{row(60), {Y: 34, W: 20, H: 34}}, done, true)
	if got := motion.SelectionOpacity(); got != 1 {
		t.Fatalf("settled opacity = %v", got)
	}

	// Direct manipulation is never faded.
	motion.Geometry(caret, nil, done, true)
	motion.Geometry(caret, []ui.Rect{row(30)}, done, false)
	if got := motion.SelectionOpacity(); got != 1 {
		t.Fatalf("direct selection opacity = %v", got)
	}
}

// Frames stop between the caret settling and the wake deadline, so breathing
// must begin exactly at that deadline or the next frame jumps.
func TestCaretWakeCoincidesWithBreathingStart(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	motion := New(now)
	deadline, ok := motion.CaretWakeAt(now)
	if !ok {
		t.Fatal("no wake after activity")
	}
	if got := motion.CaretOpacity(deadline.Add(-time.Millisecond)); got != 1 {
		t.Fatalf("caret breathed before its wake: %v", got)
	}
	if got := motion.CaretOpacity(deadline); math.Abs(float64(got-1)) > 0.0001 {
		t.Fatalf("caret jumped at its wake: %v", got)
	}
	if got := motion.CaretOpacity(deadline.Add(16 * time.Millisecond)); got >= 1 || got < 0.99 {
		t.Fatalf("breathing did not ease out of its wake: %v", got)
	}
}

func TestFocusRisesHardAndLandsSoftly(t *testing.T) {
	t.Parallel()
	if Focus(0) != 0 || Focus(1) != 1 {
		t.Fatalf("endpoints = %v, %v", Focus(0), Focus(1))
	}
	previous := float32(0)
	for step := 1; step <= 100; step++ {
		value := Focus(float32(step) / 100)
		if value < previous || value > 1 {
			t.Fatalf("Focus is not monotonic without overshoot at %d: %v", step, value)
		}
		previous = value
	}
	// Most of the travel happens early; the last quarter barely moves.
	if Focus(0.25) < 0.65 {
		t.Fatalf("Focus rises too gently: %v", Focus(0.25))
	}
	if landing := 1 - Focus(0.75); landing > 0.05 {
		t.Fatalf("Focus lands too hard: %v remaining", landing)
	}
}

func TestViewportGlideHandsOverWithoutJumping(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	motion := New(now)
	motion.ScrollTo(0, 100, now)
	midway, _ := motion.Scroll(now.Add(ViewportGlide/4), true)
	motion.ScrollTo(midway, 300, now.Add(ViewportGlide/4))
	if handed, _ := motion.Scroll(now.Add(ViewportGlide/4), true); handed != midway {
		t.Fatalf("retarget jumped from %v to %v", midway, handed)
	}
	end, _ := motion.Scroll(now.Add(ViewportGlide/4+2*ViewportGlide), true)
	if end != 300 || motion.Scrolling() {
		t.Fatalf("glide settled at %v, scrolling=%v", end, motion.Scrolling())
	}
}

// An interrupted glide keeps the speed it had: the page's velocity is
// continuous across a new destination, not reset to a fresh hard rise.
func TestViewportGlideKeepsItsSpeedAcrossANewDestination(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	frame := 250 * time.Microsecond
	speed := func(m *Motion, at time.Time) float32 {
		before, _ := m.Scroll(at.Add(-frame), true)
		after, _ := m.Scroll(at, true)
		return (after - before) / float32(frame.Seconds())
	}
	motion := New(now)
	motion.ScrollTo(0, 100, now)
	switchAt := now.Add(ViewportGlide / 2)
	was := speed(motion, switchAt)
	here, _ := motion.Scroll(switchAt, true)
	motion.ScrollTo(here, 400, switchAt)
	after, _ := motion.Scroll(switchAt.Add(frame), true)
	is := (after - here) / float32(frame.Seconds())
	if diff := is - was; diff > was*0.1+20 || diff < -(was*0.1+20) {
		t.Fatalf("speed jumped from %v to %v DIP/s at the new destination", was, is)
	}
	// From rest a glide still rises hard.
	fresh := New(now)
	fresh.ScrollTo(0, 100, now)
	if first, _ := fresh.Scroll(now.Add(ViewportGlide/10), true); first < 30 {
		t.Fatalf("a glide from rest crept to %v in its first tenth", first)
	}
}
