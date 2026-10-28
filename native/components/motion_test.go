package components

import (
	"math"
	"testing"
)

func TestSmoothstepMatchesShorewardChromeCurve(t *testing.T) {
	for _, test := range []struct {
		input float32
		want  float32
	}{
		{-1, 0},
		{0, 0},
		{0.25, 0.15625},
		{0.5, 0.5},
		{0.75, 0.84375},
		{1, 1},
		{2, 1},
	} {
		if got := Smoothstep(test.input); math.Abs(float64(got-test.want)) > 0.0001 {
			t.Fatalf("Smoothstep(%v) = %v, want %v", test.input, got, test.want)
		}
	}
}
