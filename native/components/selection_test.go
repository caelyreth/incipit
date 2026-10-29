package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type selectionFixture struct {
	selected int
	track    *SelectionTrack
	tops     [3]float32
}

func (f *selectionFixture) view(c *ui.Context) {
	semantic := theme.DefaultSettings().Semantic()
	c.SetTheme(theme.DefaultSettings().MyGoTheme())
	list := ui.Column(c).Width(240).Gap(2)
	f.track = TrackSelection(list, NavigationSelection(c, semantic), true)
	list.Children(func() {
		for index := range f.tops {
			row := NavigationRow{
				Key: fmt.Sprintf("row-%d", index), Label: fmt.Sprintf("Row %d", index),
				Selected: f.selected == index, Track: f.track, FullWidth: true,
			}.View(c, semantic)
			f.tops[index] = row.LayoutBounds().Y
		}
	})
}

func (f *selectionFixture) rowY(index int) float32 {
	return f.tops[index] - f.tops[0]
}

func TestSelectionTrackGlidesFadesAndReappearsInPlace(t *testing.T) {
	fixture := &selectionFixture{selected: 0}
	tester := ui.NewTester(fixture.view, 300, 200)
	tester.Frame()
	tester.Advance(time.Second)
	if got := fixture.track.Opacity(); got != 1 {
		t.Fatalf("initial highlight opacity = %v", got)
	}
	if got := fixture.track.Rect().Y; got != fixture.rowY(0) {
		t.Fatalf("initial highlight y = %v, want %v", got, fixture.rowY(0))
	}

	// Moving: the one highlight glides between rows.
	fixture.selected = 2
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	if y := fixture.track.Rect().Y; y <= fixture.rowY(0) || y >= fixture.rowY(2) {
		t.Fatalf("highlight jumped instead of gliding: y=%v between %v and %v",
			y, fixture.rowY(0), fixture.rowY(2))
	}
	if got := fixture.track.Opacity(); got != 1 {
		t.Fatalf("highlight faded while moving: %v", got)
	}
	tester.Advance(time.Second)
	if got := fixture.track.Rect().Y; got != fixture.rowY(2) {
		t.Fatalf("highlight settled at %v, want %v", got, fixture.rowY(2))
	}

	// Leaving: it fades out where it was.
	fixture.selected = -1
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	if got := fixture.track.Opacity(); got <= 0 || got >= 1 {
		t.Fatalf("highlight did not fade out: %v", got)
	}
	if got := fixture.track.Rect().Y; got != fixture.rowY(2) {
		t.Fatalf("fading highlight moved: %v", got)
	}
	tester.Advance(time.Second)
	if got := fixture.track.Opacity(); got > 0.01 {
		t.Fatalf("highlight remained after fading out: %v", got)
	}

	// Appearing: it fades in at the new row rather than sliding there.
	fixture.selected = 1
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	if got := fixture.track.Rect().Y; got != fixture.rowY(1) {
		t.Fatalf("appearing highlight slid in from %v, want %v", got, fixture.rowY(1))
	}
	if got := fixture.track.Opacity(); got <= 0 || got >= 1 {
		t.Fatalf("highlight did not fade in: %v", got)
	}
}

func TestSelectionTrackReselectMidFadeSlidesFromWhereItIs(t *testing.T) {
	fixture := &selectionFixture{selected: 0}
	tester := ui.NewTester(fixture.view, 300, 200)
	tester.Frame()
	tester.Advance(time.Second)
	fixture.selected = -1
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	fading := fixture.track.Opacity()
	fixture.selected = 2
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	if y := fixture.track.Rect().Y; y <= fixture.rowY(0) || y >= fixture.rowY(2) {
		t.Fatalf("partly visible highlight did not slide: y=%v", y)
	}
	if got := fixture.track.Opacity(); got <= fading {
		t.Fatalf("reselected highlight kept fading: %v -> %v", fading, got)
	}
}

func TestSegmentedSharesOneSlidingHighlight(t *testing.T) {
	selected := "Starfield"
	var track *SelectionTrack
	view := func(c *ui.Context) {
		semantic := theme.DefaultSettings().Semantic()
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		options := make([]SegmentOption, 0, 2)
		for _, label := range []string{"Starfield", "Daybreak"} {
			label := label
			options = append(options, SegmentOption{
				Key: "option-" + label, Label: label, Selected: selected == label,
				Action: func() { selected = label },
			})
		}
		group := Segmented{Key: "theme", Label: "Theme", Options: options, Animate: true}.View(c, semantic)
		track = *ui.Local(group, selectionKey{}, func() *SelectionTrack { return nil })
	}
	tester := ui.NewTester(view, 300, 100)
	tester.Frame()
	tester.Advance(time.Second)
	start := track.Rect().X
	if err := tester.Click("Daybreak"); err != nil {
		t.Fatal(err)
	}
	tester.Advance(48 * time.Millisecond)
	if x := track.Rect().X; x <= start || track.Opacity() != 1 {
		t.Fatalf("segment highlight did not slide toward Daybreak: x=%v from %v opacity=%v",
			x, start, track.Opacity())
	}
}

func TestSegmentedArrowsMoveFocusAndChoose(t *testing.T) {
	selected := "Starfield"
	view := func(c *ui.Context) {
		semantic := theme.DefaultSettings().Semantic()
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		options := make([]SegmentOption, 0, 2)
		for _, label := range []string{"Starfield", "Daybreak"} {
			label := label
			options = append(options, SegmentOption{
				Key: "option-" + label, Label: label, Selected: selected == label,
				Action: func() { selected = label },
			})
		}
		Segmented{Key: "theme", Label: "Theme", Options: options}.View(c, semantic)
	}
	tester := ui.NewTester(view, 300, 100)
	tester.Frame()
	tester.Key(0, ui.KeyTab)
	if !tester.Focused("Starfield") {
		t.Fatal("Tab did not enter the chosen segment")
	}
	tester.Key(0, ui.KeyRight)
	if selected != "Daybreak" || !tester.Focused("Daybreak") {
		t.Fatalf("Right chose %q with focus on Daybreak=%v", selected, tester.Focused("Daybreak"))
	}
}

func TestNavigationRowHoverEasesInQuicklyAndOutGently(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	view := func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		ui.Column(c).Width(240).Padding(20, 20, 20, 20).Children(func() {
			NavigationRow{Key: "row", Label: "Hover me", FullWidth: true, Animate: true}.View(c, semantic)
		})
	}
	tester := ui.NewTester(view, 300, 120)
	tester.Frame()
	row, ok := tester.Find("Hover me")
	if !ok {
		t.Fatal("row missing")
	}
	// A point inside the row but clear of its label's ink.
	x, y := int(row.X+row.W-6), int(row.Y+row.H/2)
	sample := func() int {
		c := tester.Image().RGBAAt(x, y)
		return int(c.R) + int(c.G) + int(c.B)
	}
	rest := sample()
	tester.Move(float32(x), float32(y))
	tester.Advance(40 * time.Millisecond)
	partway := sample()
	tester.Advance(120 * time.Millisecond)
	full := sample()
	if full == rest {
		t.Fatal("hover never tinted the row")
	}
	if partway == rest || partway == full {
		t.Fatalf("hover snapped instead of easing in: rest=%d 40ms=%d full=%d", rest, partway, full)
	}
	tester.Move(float32(x), 115)
	tester.Advance(100 * time.Millisecond)
	if leaving := sample(); leaving == rest || leaving == full {
		t.Fatalf("hover did not ease out: rest=%d 100ms=%d full=%d", rest, leaving, full)
	}
	tester.Advance(200 * time.Millisecond)
	if gone := sample(); gone != rest {
		t.Fatalf("hover tint remained: %d, rest %d", gone, rest)
	}
}
