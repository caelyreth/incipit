package scrollfx

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestElasticResistsAndReturnsWithVelocity(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var elastic Elastic
	elastic.Add(-1000, 64, now, Direct, DefaultProfile)
	first, moving := elastic.Step(now, true)
	if !moving || first >= 0 || -first >= 65 {
		t.Fatalf("first pull = %v, moving %v", first, moving)
	}
	elastic.Add(-1000, 64, now.Add(16*time.Millisecond), Direct, DefaultProfile)
	pulled, _ := elastic.Step(now.Add(16*time.Millisecond), true)
	if pulled > first || -pulled >= 65 {
		t.Fatalf("second pull = %v after %v", pulled, first)
	}
	var previous = pulled
	reversed := false
	for frame := 1; frame <= 120; frame++ {
		at := now.Add(16 * time.Millisecond * time.Duration(frame+1))
		value, active := elastic.Step(at, true)
		if value < -66 || value > 6 {
			t.Fatalf("recoil left its physical bounds: %v", value)
		}
		if value > previous {
			reversed = true
		}
		previous = value
		if !active {
			if value != 0 {
				t.Fatalf("settled away from its edge: %v", value)
			}
			break
		}
	}
	if !reversed {
		t.Fatal("the stretched surface returned without recoiling")
	}
}

func TestOverscrollOnlyTakesInputBeyondAnEdge(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var state ui.ScrollState
	state.MaxY = 100
	state.Y = 40
	var over Overscroll
	if over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: 20, Time: now}, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("scroll with room was taken from the container")
	}
	state.Y = 100
	if !over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: 20, Time: now}, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("scroll beyond the end was not taken")
	}
	_, start, moving := over.Step(now, true)
	if start >= 0 || !moving {
		t.Fatalf("end pull = %v moving %v", start, moving)
	}
	if !over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: -5, Time: now.Add(16 * time.Millisecond)}, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("small pull back toward the edge was not taken")
	}
	if _, nearer, _ := over.Step(now.Add(16*time.Millisecond), true); nearer <= start || nearer >= 0 {
		t.Fatalf("pull back did not follow the input: %v then %v", start, nearer)
	}
	if !over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: -100, Time: now.Add(32 * time.Millisecond)}, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("pull past the edge was not taken")
	}
	if x, y, _ := over.Step(now.Add(32*time.Millisecond), true); x != 0 || y != 0 {
		t.Fatalf("pull back stopped at %v,%v", x, y)
	}
}

func TestElasticHoldsWithTheGestureInsteadOfFightingIt(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var elastic Elastic
	previous := float32(0)
	for frame := range 8 {
		at := now.Add(time.Duration(frame) * 16 * time.Millisecond)
		elastic.Add(-10, 64, at, Direct, DefaultProfile)
		value, moving := elastic.Step(at, true)
		if !moving {
			t.Fatalf("frame %d stopped holding the pull", frame)
		}
		if value > previous {
			t.Fatalf("frame %d reversed the gesture: %v after %v", frame, value, previous)
		}
		previous = value
	}
	for frame := range 8 {
		at := now.Add(128*time.Millisecond + time.Duration(frame)*16*time.Millisecond)
		elastic.Add(4, 64, at, Direct, DefaultProfile)
		value, _ := elastic.Step(at, true)
		if value < previous {
			t.Fatalf("reverse frame %d moved farther out: %v after %v", frame, value, previous)
		}
		previous = value
	}
}

func TestNotchStartsItsReturnImmediately(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var elastic Elastic
	elastic.Add(-32, 64, now, Notch, DefaultProfile)
	if elastic.held {
		t.Fatal("a wheel notch held the edge as a touchpad gesture does")
	}
	first := elastic.Offset()
	next, moving := elastic.Step(now.Add(16*time.Millisecond), true)
	if !moving || abs(next) >= abs(first) {
		t.Fatalf("wheel return did not begin: %v then %v", first, next)
	}
}

func TestPreciseScrollSplitsAtTheEdge(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var state ui.ScrollState
	state.Y = 90
	state.MaxY = 100
	var over Overscroll
	event := ui.InputEvent{Kind: ui.InputScroll, DY: 20, Precise: true, Time: now}
	if !over.Handle(event, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("precise overshoot was not taken")
	}
	if state.Y != 100 {
		t.Fatalf("content stopped at %v, want its end", state.Y)
	}
	if _, y, _ := over.Step(now, true); y >= 0 {
		t.Fatalf("remaining travel did not pull past the end: %v", y)
	}
}

func TestMomentumReleasesWithoutHoldingTheEdge(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var state ui.ScrollState
	state.Y = 100
	state.MaxY = 100
	var over Overscroll
	direct := ui.InputEvent{
		Kind: ui.InputScroll, DY: 20, Precise: true, Time: now,
	}
	if !over.Handle(direct, &state, Vertical, 64, DefaultProfile) || !over.y.held {
		t.Fatal("direct gesture did not hold the edge")
	}
	momentum := ui.InputEvent{
		Kind: ui.InputScroll, DY: 4, Precise: true, Momentum: true,
		Time: now.Add(16 * time.Millisecond),
	}
	if !over.Handle(momentum, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("momentum was not taken at the edge")
	}
	if over.y.held {
		t.Fatal("momentum kept the surface pinned after the gesture ended")
	}
	afterFirst := over.y.Offset()
	later := momentum
	later.Time = later.Time.Add(16 * time.Millisecond)
	later.DY = 20
	if !over.Handle(later, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("continued momentum was not consumed at the edge")
	}
	if over.y.Offset() < afterFirst {
		t.Fatal("continued momentum stretched the band again")
	}
	for frame := range 60 {
		event := momentum
		event.Time = now.Add(time.Duration(frame+2) * 16 * time.Millisecond)
		over.Handle(event, &state, Vertical, 64, DefaultProfile)
		_, y, _ := over.Step(event.Time, true)
		if y < afterFirst-1 {
			t.Fatalf("momentum restretched the returned band at frame %d: %v", frame, y)
		}
	}
}
