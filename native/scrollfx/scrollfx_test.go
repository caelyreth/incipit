package scrollfx

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

const frame = 16 * time.Millisecond

var epoch = time.Unix(1_700_000_000, 0)

func at(frames int) time.Time { return epoch.Add(time.Duration(frames) * frame) }

func TestElasticResistsHoldsAndReturns(t *testing.T) {
	var elastic Elastic
	elastic.Step(at(0), true)
	elastic.Add(-1000, 64, Direct, DefaultProfile)
	first, moving := elastic.Step(at(1), true)
	if !moving || first >= 0 || first <= -64 {
		t.Fatalf("first pull = %v, moving %v", first, moving)
	}
	elastic.Add(-1000, 64, Direct, DefaultProfile)
	pulled, _ := elastic.Step(at(2), true)
	if pulled >= first || pulled <= -64 {
		t.Fatalf("second pull = %v after %v", pulled, first)
	}
	if moved, further := first, pulled-first; -further >= -moved {
		t.Fatalf("the second pull moved as far as the first: %v then %v", moved, further)
	}
	// Held, it stays put between events.
	if held, _ := elastic.Step(at(4), true); held != pulled {
		t.Fatalf("the held edge moved to %v from %v", held, pulled)
	}
	settled := false
	for n := 5; n < 120; n++ {
		value, active := elastic.Step(at(n), true)
		if value < pulled-0.01 {
			t.Fatalf("the return went farther out: %v", value)
		}
		if value > 0.5 {
			t.Fatalf("the return swung visibly past the edge: %v", value)
		}
		if !active {
			if value != 0 {
				t.Fatalf("settled away from its edge: %v", value)
			}
			settled = true
			break
		}
	}
	if !settled {
		t.Fatal("the edge did not settle")
	}
}

func TestPullingBackRetracesAndHandsTheRestToTheContent(t *testing.T) {
	var elastic Elastic
	elastic.Step(at(0), true)
	elastic.Add(-30, 64, Direct, DefaultProfile)
	out := elastic.Offset()
	elastic.Add(-20, 64, Direct, DefaultProfile)
	elastic.Add(20, 64, Direct, DefaultProfile)
	if back := elastic.Offset(); abs(back-out) > 0.001 {
		t.Fatalf("pulling back did not retrace: %v, was %v", back, out)
	}
	if rest := elastic.Add(50, 64, Direct, DefaultProfile); abs(rest-20) > 0.001 || elastic.Offset() != 0 {
		t.Fatalf("crossing the edge left %v with the edge at %v", rest, elastic.Offset())
	}
}

func TestOverscrollOnlyTakesInputBeyondAnEdge(t *testing.T) {
	var state ui.ScrollState
	state.MaxY = 100
	state.Y = 40
	var over Overscroll
	over.Step(at(0), true)
	direct := func(dy float32) ui.InputEvent {
		return ui.InputEvent{Kind: ui.InputScroll, DY: dy, Precise: true}
	}
	if over.Handle(direct(20), &state, Vertical, 64, DefaultProfile) {
		t.Fatal("scroll with room was taken from the container")
	}
	state.Y = 100
	if !over.Handle(direct(20), &state, Vertical, 64, DefaultProfile) {
		t.Fatal("scroll beyond the end was not taken")
	}
	_, start, moving := over.Step(at(1), true)
	if start >= 0 || !moving {
		t.Fatalf("end pull = %v moving %v", start, moving)
	}
	over.Handle(direct(-5), &state, Vertical, 64, DefaultProfile)
	if _, nearer, _ := over.Step(at(2), true); nearer <= start || nearer >= 0 {
		t.Fatalf("pull back did not follow the input: %v then %v", start, nearer)
	}
	if !over.Handle(direct(-100), &state, Vertical, 64, DefaultProfile) {
		t.Fatal("pull back through the edge was not taken")
	}
	if _, y := over.Offset(); y != 0 {
		t.Fatalf("pull back stopped at %v", y)
	}
	if state.Y >= 100 || state.Y <= 0 {
		t.Fatalf("the content did not take the rest of the move: %v", state.Y)
	}
}

// Scroll events need not carry a time: the hold must come from the frames.
func TestAGestureWithoutTimesHoldsTheEdge(t *testing.T) {
	var state ui.ScrollState
	state.MaxY, state.Y = 100, 100
	var over Overscroll
	over.Step(at(0), true)
	previous := float32(0)
	for n := 1; n <= 20; n++ {
		over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: 6, Precise: true}, &state, Vertical, 64, DefaultProfile)
		_, y, _ := over.Step(at(n), true)
		if y > previous {
			t.Fatalf("frame %d sprang back under the fingers: %v after %v", n, y, previous)
		}
		previous = y
	}
}

// A run of wheel notches at an edge swings it smoothly: no frame jumps.
func TestWheelNotchesSwingWithoutJumping(t *testing.T) {
	var state ui.ScrollState
	state.MaxY, state.Y = 100, 100
	var over Overscroll
	over.Step(at(0), true)
	notch := ui.InputEvent{Kind: ui.InputScroll, DY: 40}
	if !over.Handle(notch, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("a notch at the end was not taken")
	}
	if _, y := over.Offset(); y != 0 {
		t.Fatalf("a notch jumped the edge to %v", y)
	}
	previous, deepest := float32(0), float32(0)
	for n := 1; n <= 90; n++ {
		if n%4 == 0 && n <= 40 {
			over.Handle(notch, &state, Vertical, 64, DefaultProfile)
		}
		_, y, _ := over.Step(at(n), true)
		if jump := abs(y - previous); jump > 6 {
			t.Fatalf("frame %d jumped %v DIPs", n, jump)
		}
		if y < -64 {
			t.Fatalf("frame %d went past the limit: %v", n, y)
		}
		previous, deepest = y, min(deepest, y)
	}
	if deepest > -4 {
		t.Fatalf("the notches hardly moved the edge: %v", deepest)
	}
	if previous != 0 {
		t.Fatalf("the edge did not settle: %v", previous)
	}
}

func TestPreciseScrollSplitsAtTheEdge(t *testing.T) {
	var state ui.ScrollState
	state.Y = 90
	state.MaxY = 100
	var over Overscroll
	event := ui.InputEvent{Kind: ui.InputScroll, DY: 20, Precise: true}
	if !over.Handle(event, &state, Vertical, 64, DefaultProfile) {
		t.Fatal("precise overshoot was not taken")
	}
	if state.Y != 100 {
		t.Fatalf("content stopped at %v, want its end", state.Y)
	}
	if _, y := over.Offset(); y >= 0 {
		t.Fatalf("remaining travel did not pull past the end: %v", y)
	}
}

func TestMomentumPushesOncePerGesture(t *testing.T) {
	var state ui.ScrollState
	state.MaxY, state.Y = 100, 100
	var over Overscroll
	over.Step(at(0), true)
	momentum := ui.InputEvent{Kind: ui.InputScroll, DY: 30, Precise: true, Momentum: true}
	over.Handle(momentum, &state, Vertical, 64, DefaultProfile)
	deepest := float32(0)
	for n := 1; n <= 60; n++ {
		over.Handle(momentum, &state, Vertical, 64, DefaultProfile)
		_, y, _ := over.Step(at(n), true)
		deepest = min(deepest, y)
		if n > 20 && y < deepest/2 {
			t.Fatalf("continued inertia pushed the returning edge again at frame %d: %v", n, y)
		}
	}
	if deepest > -4 {
		t.Fatalf("inertia hardly pushed the edge: %v", deepest)
	}
	// A new gesture, though it starts inside the content, pushes again.
	state.Y = 60
	over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: 10, Precise: true}, &state, Vertical, 64, DefaultProfile)
	state.Y = 100
	over.Handle(momentum, &state, Vertical, 64, DefaultProfile)
	if _, y, _ := over.Step(at(62), true); y >= 0 {
		t.Fatalf("a new gesture's inertia did not push the edge: %v", y)
	}
}

func TestMomentumReleasesAHeldEdge(t *testing.T) {
	var state ui.ScrollState
	state.MaxY, state.Y = 100, 100
	var over Overscroll
	over.Step(at(0), true)
	over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: 20, Precise: true}, &state, Vertical, 64, DefaultProfile)
	if !over.y.held {
		t.Fatal("direct gesture did not hold the edge")
	}
	over.Handle(ui.InputEvent{Kind: ui.InputScroll, DY: 4, Precise: true, Momentum: true}, &state, Vertical, 64, DefaultProfile)
	if over.y.held {
		t.Fatal("momentum kept the edge pinned after the gesture ended")
	}
}
