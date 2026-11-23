package motion

import (
	"math"
	"testing"
	"time"
)

func TestSpringConvergesWithoutOvershoot(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	spring := NewSpring(0, now)
	previous := float32(0)
	spring.Step(100, now, 38)
	for frame := 1; frame <= 100; frame++ {
		value := spring.Step(100, now.Add(time.Duration(frame)*16*time.Millisecond), 38)
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
	spring.Step(100, now, 38)
	later := now.Add(40 * time.Millisecond)
	uninterrupted := spring
	before := uninterrupted.Step(100, later, 38)
	if got := spring.Step(60, later, 38); got != before {
		t.Fatalf("retarget jumped: got %v before %v", got, before)
	}
	after := spring.Step(60, later.Add(10*time.Millisecond), 38)
	if after <= before {
		t.Fatalf("retarget lost velocity: before %v after %v", before, after)
	}
	if got := spring.Step(60, now.Add(time.Second), 38); got != 60 {
		t.Fatalf("settled = %v", got)
	}
}

func TestUnitSpringSettlesWithoutAVisibleTerminalJump(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	spring := NewUnitSpring(0, now)
	spring.Step(1, now, 18)
	previous := float32(0)
	for frame := 1; frame <= 180; frame++ {
		value := spring.Step(1, now.Add(time.Duration(frame)*time.Second/60), 18)
		if value < previous || value > 1 {
			t.Fatalf("frame %d value %v after %v", frame, value, previous)
		}
		if value == 1 {
			if 1-previous > 0.01 {
				t.Fatalf("terminal jump from %v to %v", previous, value)
			}
			if spring.Moving() {
				t.Fatal("unit spring still moving at its target")
			}
			return
		}
		previous = value
	}
	t.Fatal("unit spring did not settle")
}

func TestUnitSpringMatchesScaledGeometrySpring(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	unit, scaled := NewUnitSpring(0, now), NewSpring(0, now)
	unit.Step(1, now, 14)
	scaled.Step(100, now, 14)
	for frame := 1; frame <= 120; frame++ {
		at := now.Add(time.Duration(frame) * time.Second / 60)
		actual := unit.Step(1, at, 14) * 100
		want := scaled.Step(100, at, 14)
		if math.Abs(float64(actual-want)) > 0.001 || unit.Moving() != scaled.Moving() {
			t.Fatalf("frame %d: unit=%v moving=%v, scaled=%v moving=%v", frame, actual, unit.Moving(), want, scaled.Moving())
		}
	}
}
