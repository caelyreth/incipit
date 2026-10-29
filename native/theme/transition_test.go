package theme

import (
	"testing"
	"time"
)

func TestBlendMixesEveryColorBetweenThemes(t *testing.T) {
	t.Parallel()
	ink, paper := DefaultSettings(), DefaultSettings()
	paper.Appearance = Paper
	from, to := ink.Semantic(), paper.Semantic()
	if got := Blend(from, to, 0); !sameColors(got, from) {
		t.Fatal("blending nothing changed the theme")
	}
	if got := Blend(from, to, 1); got != to {
		t.Fatal("blending fully missed the target")
	}
	half := Blend(from, to, 0.5)
	for name, pair := range map[string][3]uint8{
		"background": {from.Surface.Background.R, half.Surface.Background.R, to.Surface.Background.R},
		"text":       {from.Prose.Text.R, half.Prose.Text.R, to.Prose.Text.R},
		"accent":     {from.Accent.Foreground.G, half.Accent.Foreground.G, to.Accent.Foreground.G},
	} {
		low, high := min(pair[0], pair[2]), max(pair[0], pair[2])
		if pair[1] <= low || pair[1] >= high {
			t.Errorf("%s halfway = %d, want between %d and %d", name, pair[1], pair[0], pair[2])
		}
	}
}

// sameColors compares what Blend mixes; labels always name the target.
func sameColors(a, b Theme) bool {
	a.Appearance, a.AccentFamily = b.Appearance, b.AccentFamily
	return a == b
}

func TestTransitionCarriesAChangeAndResumesFromWhereItIs(t *testing.T) {
	t.Parallel()
	ink, paper := DefaultSettings(), DefaultSettings()
	paper.Appearance = Paper
	start := time.Unix(1_700_000_000, 0)
	var colors Transition
	if got, moving := colors.At(ink, start); got != ink.Semantic() || moving {
		t.Fatal("an untouched transition must show its settings, still")
	}
	colors.Retarget(paper.Semantic(), start)
	mid, moving := colors.At(paper, start.Add(ThemeTransition/2))
	if !moving || mid == ink.Semantic() || mid == paper.Semantic() {
		t.Fatal("halfway through a change shows neither end")
	}
	// Switching back mid-change starts from what is showing, not a snap.
	colors.Retarget(ink.Semantic(), start.Add(ThemeTransition/2))
	if back, _ := colors.At(ink, start.Add(ThemeTransition/2)); !sameColors(back, mid) {
		t.Fatal("reversing a change jumped")
	}
	if settled, moving := colors.At(ink, start.Add(2*ThemeTransition)); moving || settled != ink.Semantic() {
		t.Fatal("the change never settled")
	}
	colors.Jump(paper.Semantic())
	if got, moving := colors.At(paper, start.Add(2*ThemeTransition)); moving || got != paper.Semantic() {
		t.Fatal("a jump did not show its theme at once")
	}
}
