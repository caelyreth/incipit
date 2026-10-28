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
			"guide",
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
