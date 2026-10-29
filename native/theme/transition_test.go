package theme

import (
	"testing"
	"time"
)

// sameColors compares only colors; labels always name the target.
func sameColors(a, b Theme) bool {
	a.Appearance, a.AccentFamily = b.Appearance, b.AccentFamily
	return a == b
}

func TestTransitionCarriesAChangeAndSettles(t *testing.T) {
	t.Parallel()
	starfield, daybreak := DefaultSettings(), DefaultSettings()
	daybreak.Appearance = Daybreak
	start := time.Unix(1_700_000_000, 0)
	var colors Transition
	if got, moving := colors.At(starfield, start); !sameColors(got, starfield.Semantic()) || moving {
		t.Fatal("an untouched transition must show its settings, still")
	}
	colors.Retarget(daybreak.Semantic(), start)
	mid, moving := colors.At(daybreak, start.Add(ThemeTransition/3))
	if !moving || sameColors(mid, starfield.Semantic()) || sameColors(mid, daybreak.Semantic()) {
		t.Fatal("partway through a change shows neither end")
	}
	from, to := starfield.Semantic().Surface.Background.R, daybreak.Semantic().Surface.Background.R
	if got := mid.Surface.Background.R; got <= from || got >= to {
		t.Fatalf("background red partway = %d, want between %d and %d", got, from, to)
	}
	if settled, moving := colors.At(daybreak, start.Add(3*ThemeTransition)); moving ||
		!sameColors(settled, daybreak.Semantic()) {
		t.Fatal("the change never settled")
	}
	colors.Jump(starfield.Semantic())
	if got, moving := colors.At(starfield, start.Add(3*ThemeTransition)); moving ||
		!sameColors(got, starfield.Semantic()) {
		t.Fatal("a jump did not show its theme at once")
	}
}

// Flipping back mid-change neither jumps nor reverses instantly: the colors
// keep their momentum for a moment, then ease back.
func TestReversingAChangeKeepsItsMomentum(t *testing.T) {
	t.Parallel()
	starfield, daybreak := DefaultSettings(), DefaultSettings()
	daybreak.Appearance = Daybreak
	start := time.Unix(1_700_000_000, 0)
	var colors Transition
	colors.Jump(starfield.Semantic())
	colors.Retarget(daybreak.Semantic(), start)
	flip := start.Add(ThemeTransition / 4)
	before, _ := colors.At(daybreak, flip)
	colors.Retarget(starfield.Semantic(), flip)
	same, _ := colors.At(starfield, flip)
	if !sameColors(same, before) {
		t.Fatal("reversing a change jumped")
	}
	// Still brightening just after the flip, toward Daybreak, before
	// turning back.
	later, _ := colors.At(starfield, flip.Add(8*time.Millisecond))
	if later.Surface.Background.R <= before.Surface.Background.R {
		t.Fatalf("the reversal snapped direction: %d then %d",
			before.Surface.Background.R, later.Surface.Background.R)
	}
	if settled, moving := colors.At(starfield, flip.Add(3*ThemeTransition)); moving ||
		!sameColors(settled, starfield.Semantic()) {
		t.Fatal("the reversal never settled")
	}
}
