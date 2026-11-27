package components

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSmoothstepMatchesIncipitChromeCurve(t *testing.T) {
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

func TestMotionEnabledCombinesWindowAndDesktopPolicy(t *testing.T) {
	enabled := true
	var got bool
	tester := ui.NewTester(func(c *ui.Context) {
		got = MotionEnabled(c, enabled)
		ui.Box(c).Fill()
	}, 100, 100)
	tester.Frame()
	if !got {
		t.Fatal("active window with motion enabled did not animate")
	}
	tester.SetWindowActive(false)
	if !got {
		t.Fatal("unfocused visible window stopped animating")
	}
	tester.SetWindowActive(true)
	tester.SetWindowHidden(true)
	tester.Frame()
	if got {
		t.Fatal("hidden window animated")
	}
	tester.SetWindowHidden(false)
	enabled = false
	tester.Frame()
	if got {
		t.Fatal("disabled motion animated")
	}
	enabled = true
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()
	if got {
		t.Fatal("reduced motion animated")
	}
}
