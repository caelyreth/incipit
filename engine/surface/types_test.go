package surface

import (
	"math"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

func TestTimelineSamplingAndDeadline(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	opacity := TimedOpacity(now, 1100*time.Millisecond, 56, EnvelopePulse)
	if got := opacity.Sample(now, true); got != 0 {
		t.Fatalf("start = %v", got)
	}
	if got := opacity.Sample(now.Add(100*time.Millisecond), true); got <= 0 {
		t.Fatalf("pulse = %v", got)
	}
	if got := opacity.Sample(now, false); got <= 0 {
		t.Fatalf("reduced motion = %v", got)
	}
	if got := opacity.Sample(now.Add(1100*time.Millisecond), false); got != 0 {
		t.Fatalf("deadline = %v", got)
	}
	if deadline, ok := opacity.Deadline(); !ok || !deadline.Equal(now.Add(1100*time.Millisecond)) {
		t.Fatalf("deadline = %v,%v", deadline, ok)
	}
}

func TestFrameValidatesIdentityCoordinatesAndSteadyLifetimes(t *testing.T) {
	t.Parallel()
	flow := fixtureFlow(t)
	now := time.Unix(0, 0)
	decoration := Decoration{
		ID: DecorationID{Producer: "test", Local: 7},
		Primitive: GuidePrimitive(
			1,
			[]string{"Tab"},
			ColorMuted,
		),
		Opacity:    FixedOpacity(255),
		Visibility: VisibilityFocused,
		ExpiresAt:  &now,
	}
	if got := decoration.Sample(now, false); got != 0 {
		t.Fatalf("expired decoration = %v", got)
	}
	if _, err := NewFrame(
		flow,
		model.NewUTF16Range(0, 0),
		0,
		TextStyle{},
		[]Decoration{decoration, decoration},
	); err == nil {
		t.Fatal("duplicate decoration accepted")
	}
	invalid := decoration
	invalid.Primitive = FillPrimitive([]model.UTF16Range{model.NewUTF16Range(1, 2)}, ColorText, 4)
	if _, err := NewFrame(
		flow,
		model.NewUTF16Range(0, 0),
		0,
		TextStyle{},
		[]Decoration{invalid},
	); err == nil {
		t.Fatal("split-surrogate fill accepted")
	}
	invalid.Primitive = RulePrimitive(nil, ColorText, float32(math.NaN()))
	if _, err := NewFrame(
		flow,
		model.NewUTF16Range(0, 0),
		0,
		TextStyle{},
		[]Decoration{invalid},
	); err == nil {
		t.Fatal("NaN rule accepted")
	}
}

func TestEmptyFlowConversions(t *testing.T) {
	t.Parallel()
	flow := EmptyFlow(projection.NewVersion(0, 0))
	if flow.UTF16Len() != 0 {
		t.Fatalf("empty length = %v", flow.UTF16Len())
	}
	if byteOffset, ok := flow.ByteOfUTF16(0); !ok || byteOffset != 0 {
		t.Fatalf("empty byte = %d,%v", byteOffset, ok)
	}
	if unit, ok := flow.UTF16OfByte(0); !ok || unit != 0 {
		t.Fatalf("empty unit = %d,%v", unit, ok)
	}
}

// A pulse marks plainly and lets go soon: it starts and ends at nothing,
// peaks once early, is under half by halfway, and never jumps.
func TestPulseEnvelopeIsSmoothAndComplete(t *testing.T) {
	t.Parallel()
	if EnvelopePulse.At(0) != 0 || EnvelopePulse.At(1) != 0 {
		t.Fatalf("ends at %v and %v", EnvelopePulse.At(0), EnvelopePulse.At(1))
	}
	peak, previous := float32(0), float32(0)
	for step := 1; step <= 1000; step++ {
		value := EnvelopePulse.At(float32(step) / 1000)
		if math.Abs(float64(value-previous)) > 0.035 {
			t.Fatalf("steps by %v at %d‰", value-previous, step)
		}
		peak, previous = max(peak, value), value
	}
	if peak < 0.999 {
		t.Fatalf("peak %v", peak)
	}
	if EnvelopePulse.At(0.08) > 0.95 || EnvelopePulse.At(0.16) < 0.999 || EnvelopePulse.At(0.5) > 0.5 {
		t.Fatalf("%v at a twelfth, %v by a sixth, %v by halfway",
			EnvelopePulse.At(0.08), EnvelopePulse.At(0.16), EnvelopePulse.At(0.5))
	}
	// Played backwards, a pulse shows the same strengths in reverse.
	now := time.Unix(0, 0)
	forward := TimedOpacity(now, time.Second, 255, EnvelopePulse)
	backward := forward.Played(now, time.Second, 1, 0)
	for _, at := range []time.Duration{100, 300, 700} {
		f := forward.Sample(now.Add(at*time.Millisecond), true)
		b := backward.Sample(now.Add((1000-at)*time.Millisecond), true)
		if math.Abs(float64(f-b)) > 1e-4 {
			t.Fatalf("at %v: forward %v, backward %v", at, f, b)
		}
	}
}
