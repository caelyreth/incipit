package motion

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

const frame = 16 * time.Millisecond

var epoch = time.Unix(1_700_000_000, 0)

func at(frames int) time.Time { return epoch.Add(time.Duration(frames) * frame) }

func TestScrollPhase(t *testing.T) {
	for _, test := range []struct {
		precise, momentum bool
		want              InputPhase
	}{
		{false, false, Notch},
		{true, false, Direct},
		{false, true, Momentum},
		{true, true, Momentum},
	} {
		if got := Phase(test.precise, test.momentum); got != test.want {
			t.Fatalf("Phase(%v, %v) = %v, want %v", test.precise, test.momentum, got, test.want)
		}
	}
}

func TestAxisScrollPreservesSurfaceOwnership(t *testing.T) {
	for _, test := range []struct {
		name     string
		position float32
		delta    float32
		phase    InputPhase
		policy   AxisPolicy
		want     AxisResult
		pull     bool
	}{
		{"owned notch", 0, 20, Notch, AxisPolicy{}, AxisResult{Position: 20, Handled: true}, false},
		{"container notch", 0, 20, Notch, AxisPolicy{Container: true}, AxisResult{Position: 0}, false},
		{"owned crossing", 90, 20, Direct, AxisPolicy{}, AxisResult{Position: 100, Handled: true}, true},
		{"container crossing", 90, 20, Direct, AxisPolicy{Container: true}, AxisResult{Position: 100, Handled: true}, true},
		{"container tolerance", 99.7, 1, Direct, AxisPolicy{Container: true, EdgeTolerance: 0.5}, AxisResult{Position: 99.7, Handled: true}, true},
		{"owned tolerance", 99.7, 0.1, Direct, AxisPolicy{}, AxisResult{Position: 99.8, Handled: true}, false},
		{"negative lower bound", -40, -20, Direct, AxisPolicy{}, AxisResult{Position: -50, Handled: true}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var elastic Elastic
			got := AxisScroll(&elastic, test.delta, test.position, -50, 100, test.phase, time.Time{}, 64, DefaultProfile, test.policy)
			if got.Handled != test.want.Handled || abs(got.Position-test.want.Position) > 0.001 {
				t.Fatalf("result = %+v, want %+v", got, test.want)
			}
			if (elastic.Offset() != 0) != test.pull {
				t.Fatalf("pull = %v, want pull %v", elastic.Offset(), test.pull)
			}
		})
	}
}

func TestElasticResistsHoldsAndReturns(t *testing.T) {
	var elastic Elastic
	elastic.Step(at(0), true)
	elastic.Add(-200, 64, Direct, DefaultProfile)
	first, moving := elastic.Step(at(1), true)
	if !moving || first >= 0 || first <= -64 {
		t.Fatalf("first pull = %v, moving %v", first, moving)
	}
	elastic.Add(-200, 64, Direct, DefaultProfile)
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
	for n := 1; n <= 150; n++ {
		if n%4 == 0 && n <= 40 {
			over.Handle(notch, &state, Vertical, 64, DefaultProfile)
		}
		_, y, _ := over.Step(at(n), true)
		// No faster than the notches move the content, 10 DIPs a frame.
		if jump := abs(y - previous); jump > 10 {
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
	// Inertia at 30 DIPs a frame comes in at about 1900 DIPs a second.
	momentum := ui.InputEvent{Kind: ui.InputScroll, DY: 30, Precise: true, Momentum: true, Time: at(0)}
	over.Handle(momentum, &state, Vertical, 64, DefaultProfile)
	deepest := float32(0)
	for n := 1; n <= 60; n++ {
		momentum.Time = at(n)
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
	momentum.Time = at(61)
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

// owned scrolls an owning viewport over 0 to 1000 by events, one a frame
// from frame from, and steps it; it reports where the content is drawn
// each frame, the edge's offset taken off, for frames more after.
func owned(e *Elastic, position float32, from int, deltas []float32, phase InputPhase, frames int) (float32, []float32) {
	var drawn []float32
	for n := range frames {
		if n < len(deltas) {
			position = AxisScroll(e, deltas[n], position, 0, 1000, phase, at(from+n), 600, DefaultProfile, AxisPolicy{}).Position
		}
		e.Step(at(from+n), true)
		drawn = append(drawn, position-e.Offset())
	}
	return position, drawn
}

// fling is inertia from speed DIPs a frame, slowing as a touchpad's does.
func fling(speed float32) []float32 {
	var out []float32
	for v := speed; abs(v) > 0.5; v *= 0.94 {
		out = append(out, v)
	}
	return out
}

// The edge takes the speed the content met it at: a faster fling carries
// it farther, never to its limit, and it comes back without crossing.
func TestTheEdgeTakesTheSpeedItCameAt(t *testing.T) {
	deepest := func(speed float32) float32 {
		var e Elastic
		e.Step(at(0), true)
		_, drawn := owned(&e, 200, 1, fling(-speed), Momentum, 200)
		low := float32(0)
		for n, d := range drawn {
			low = min(low, d)
			if n > 0 && d < 0 && d < drawn[n-1] && low < d-0.01 {
				t.Fatalf("speed %v: the edge went out again at frame %d", speed, n)
			}
			if d > 0.01 && n > 0 && drawn[n-1] <= 0 {
				t.Fatalf("speed %v: the return crossed the edge to %v", speed, d)
			}
		}
		if last := drawn[len(drawn)-1]; last != 0 {
			t.Fatalf("speed %v: the edge settled at %v", speed, last)
		}
		return -low
	}
	slow, fast := deepest(15), deepest(60)
	if slow < 2 || fast <= slow*1.5 || fast >= 300 {
		t.Fatalf("a slow fling carried the edge %v, a fast one %v, of 600", slow, fast)
	}
}

// Fingers that pull an edge and then pull back through it hold it no
// more: letting go throws nothing past the edge the other way, and the
// fling they let go with scrolls the content.
func TestPullingBackThroughTheEdgeThrowsNothingPast(t *testing.T) {
	var e Elastic
	e.Step(at(0), true)
	position, _ := owned(&e, 0, 1, []float32{-15, -15, -15, -15}, Direct, 4)
	position, drawn := owned(&e, position, 5, []float32{30, 30, 30, 30}, Direct, 30)
	for n, d := range drawn[4:] {
		if d < drawn[3]-0.01 {
			t.Fatalf("frame %d after letting go went back to %v from %v", n, d, drawn[3])
		}
	}
	after, _ := owned(&e, position, 40, fling(30), Momentum, 60)
	if after <= position+100 {
		t.Fatalf("the fling away from the edge scrolled %v to %v", position, after)
	}
}

// A position the range left behind, as content that shrank under it,
// scrolls back in from where it is, and pulls no edge.
func TestAPositionPastTheRangeScrollsBackIn(t *testing.T) {
	var e Elastic
	got := AxisScroll(&e, -10, 1300, 0, 1000, Direct, at(0), 80, DefaultProfile, AxisPolicy{})
	if got.Position != 1290 || e.Offset() != 0 {
		t.Fatalf("scrolled to %v, the edge at %v", got.Position, e.Offset())
	}
}

// A wheel turned back from an edge it swung out stops the swing: the edge
// goes no farther out.
func TestTurningTheWheelBackStopsTheSwing(t *testing.T) {
	var e Elastic
	e.Step(at(0), true)
	position, _ := owned(&e, 0, 1, []float32{-120}, Notch, 1)
	out := e.Offset()
	owned(&e, position, 2, []float32{40}, Notch, 1)
	if e.Offset() > out+0.01 {
		t.Fatalf("the edge went on out to %v from %v", e.Offset(), out)
	}
}
