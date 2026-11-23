package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

type selectionFixture struct {
	selected int
	track    *SelectionTrack
	tops     [3]float32
	at       [3]bool
}

func (f *selectionFixture) view(c *ui.Context) {
	semantic := theme.DefaultSettings().Semantic()
	c.SetTheme(theme.DefaultSettings().MyGoTheme())
	list := ui.Column(c).Width(240).Gap(2)
	f.track = TrackSelection(list, NavigationSelection(c, semantic), true)
	list.Children(func() {
		for index := range f.tops {
			row := ui.Row(c).Key(fmt.Sprintf("row-%d", index)).Label(fmt.Sprintf("Row %d", index)).
				WidthPercent(100).Height(32)
			if f.selected == index {
				f.track.Mark(row)
			}
			f.tops[index] = row.LayoutBounds().Y
			f.at[index] = f.track.At(row)
		}
	})
}

func TestSelectionTrackAtFollowsTheVisibleHighlight(t *testing.T) {
	fixture := &selectionFixture{selected: 0}
	tester := ui.NewTester(fixture.view, 300, 200)
	tester.Frame()
	tester.Advance(time.Second)
	tester.Frame()
	if !fixture.at[0] || fixture.at[2] {
		t.Fatalf("highlight positions = %v", fixture.at)
	}
	fixture.selected = 2
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	tester.Frame()
	if fixture.at[2] {
		t.Fatal("highlight reported arrival while it was still gliding")
	}
	tester.Advance(time.Second)
	tester.Frame()
	if !fixture.at[2] || fixture.at[0] {
		t.Fatalf("highlight positions after moving = %v", fixture.at)
	}
}

func (f *selectionFixture) rowY(index int) float32 {
	return f.tops[index] - f.tops[0]
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

func TestHoverEasesInQuicklyAndOutGently(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	view := func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		ui.Column(c).Width(240).Padding(20, 20, 20, 20).Children(func() {
			row := ui.Row(c).Key("row").Label("Hover me").WidthPercent(100).Height(32)
			hover := HoverAmount(row, row.Hovered(), true)
			row.Background(blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
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
	tester.Advance(400 * time.Millisecond)
	full := sample()
	if full == rest {
		t.Fatal("hover never tinted the row")
	}
	if partway == rest || partway == full {
		t.Fatalf("hover snapped instead of easing in: rest=%d mid=%d full=%d", rest, partway, full)
	}
	tester.Move(float32(x), 115)
	tester.Advance(100 * time.Millisecond)
	if leaving := sample(); leaving == rest || leaving == full {
		t.Fatalf("hover did not ease out: rest=%d mid=%d full=%d", rest, leaving, full)
	}
	tester.Advance(500 * time.Millisecond)
	if gone := sample(); gone != rest {
		t.Fatalf("hover tint remained: %d, rest %d", gone, rest)
	}
}
