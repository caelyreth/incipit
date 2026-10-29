package components

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestButtonsUseSourceGeometryAndActions(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	var clicked []string
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		Button{
			Key: "medium", Label: "Medium",
			Action: func() { clicked = append(clicked, "medium") },
		}.View(c, semantic)
		Button{
			Key: "compact", Label: "Compact", Size: CompactButton, Style: SubtleButton,
			Action: func() { clicked = append(clicked, "compact") },
		}.View(c, semantic)
	}, 400, 160)
	tester.Frame()
	medium, ok := tester.Find("Medium")
	if !ok || medium.H != theme.DefaultControls.Height {
		t.Fatalf("medium button = %#v, %v", medium, ok)
	}
	compact, ok := tester.Find("Compact")
	if !ok || compact.H != theme.DefaultControls.InlineHeight {
		t.Fatalf("compact button = %#v, %v", compact, ok)
	}
	if err := tester.Click("Medium"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Compact"); err != nil {
		t.Fatal(err)
	}
	if len(clicked) != 2 || clicked[0] != "medium" || clicked[1] != "compact" {
		t.Fatalf("actions = %#v", clicked)
	}
}

func TestDisabledButtonDoesNotAct(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	acted := false
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		Button{
			Key: "disabled", Label: "Disabled", Disabled: true,
			Action: func() { acted = true },
		}.View(c, semantic)
	}, 300, 100)
	tester.Frame()
	if err := tester.Click("Disabled"); err != nil {
		t.Fatal(err)
	}
	if acted {
		t.Fatal("disabled button ran its action")
	}
}

func TestButtonTooltipShowsLabelAndLiveBinding(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		Button{
			Key: "bold", Label: "Bold",
			Tooltip: CommandTooltip("Bold", "⌘B"),
		}.View(c, semantic)
	}, 320, 140)
	tester.Frame()
	bounds, ok := tester.Find("Bold")
	if !ok {
		t.Fatal("button missing")
	}
	tester.Move(bounds.X+bounds.W/2, bounds.Y+bounds.H/2)
	tester.Advance(700 * time.Millisecond)
	if !tester.HasText("Bold  ⌘B") {
		t.Fatalf("tooltip texts = %#v", tester.Texts())
	}
}
