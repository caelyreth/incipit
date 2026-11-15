package lab

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A region's color is the share of its pixels that are colored, and the
// hues that hold enough of them: a speck of another hue is not a hue.
func TestMeasureCountsColoredAreaAndItsHues(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			c := color.RGBA{0x16, 0x16, 0x16, 0xff}
			switch {
			case y < 25:
				c = color.RGBA{0xdc, 0x87, 0x89, 0xff} // Afterglow, hue ≈ 18°, in the bin about 30°
			case y == 99 && x < 2:
				c = color.RGBA{0x7d, 0xad, 0x96, 0xff} // a speck of Life
			}
			img.Set(x, y, c)
		}
	}
	got := Measure(img, img.Bounds())
	if got.Area < 0.25 || got.Area > 0.2503 {
		t.Fatalf("area %v", got.Area)
	}
	if !slices.Equal(got.Hues, []int{30}) {
		t.Fatalf("hues %v", got.Hues)
	}
	if empty := Measure(img, image.Rect(0, 30, 100, 90)); empty.Area != 0 || len(empty.Hues) != 0 {
		t.Fatalf("a grey region reads %+v", empty)
	}
}

// Cosmofi's derivation leaves its ladder inverted, as the audit measured;
// the quiet scheme keeps it in order by night and by day.
func TestTheQuietSchemeKeepsTheLadderInOrder(t *testing.T) {
	if got := Breaks(theme.Cosmofi().Starfield, theme.Starfield); len(got) != 1 {
		t.Fatalf("Cosmofi's night breaks %v", got)
	}
	if got := Breaks(theme.Cosmofi().Daybreak, theme.Daybreak); len(got) != 3 {
		t.Fatalf("Cosmofi's day breaks %v", got)
	}
	for _, appearance := range []theme.Appearance{theme.Starfield, theme.Daybreak} {
		if got := Breaks(quiet.For(appearance), appearance); len(got) != 0 {
			t.Fatalf("the quiet scheme breaks %v in appearance %v", got, appearance)
		}
	}
}

// What is chosen in the lab is handed to the app.
func TestTheLabHandsItsChoicesToTheApp(t *testing.T) {
	var applied []Choice
	settings := theme.DefaultSettings()
	l := &Lab{
		Choice:   Choice{Accent: theme.Halo, FollowGroup: true},
		Apply:    func(c Choice) { applied = append(applied, c) },
		Settings: func() theme.Settings { return settings },
		Colors:   &theme.Transition{},
	}
	tester := ui.NewTester(l.View, 860, 900)
	tester.Frame()
	tester.Click("Quiet")
	tester.Frame()
	tester.Click("Daybreak")
	tester.Frame()
	tester.Click("Unripe")
	tester.Frame()
	if len(applied) != 3 {
		t.Fatalf("applied %+v", applied)
	}
	if last := applied[2]; last.Scheme != 1 || last.Appearance != theme.Daybreak ||
		last.Accent != theme.Unripe || last.FollowGroup {
		t.Fatalf("the last choice is %+v", last)
	}
}
