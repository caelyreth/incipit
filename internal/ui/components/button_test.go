package components

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/ui/theme"
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

// A button with no words of its own takes a hint naming it and its key,
// once the pointer rests; one whose words say it takes none.
func TestOnlyWordlessButtonsTakeHints(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		ui.Row(c).Gap(40).Children(func() {
			IconButton{Key: "bold", Label: "Bold", Binding: "⌘B", Icon: IconClose.SVG()}.View(c, semantic)
			Button{Key: "save", Label: "Save"}.View(c, semantic)
		})
	}, 320, 140)
	tester.Frame()
	for _, probe := range []struct {
		name string
		hint bool
	}{{"Bold", true}, {"Save", false}} {
		bounds, ok := tester.Find(probe.name)
		if !ok {
			t.Fatalf("%s missing", probe.name)
		}
		tester.Move(bounds.X+bounds.W/2, bounds.Y+bounds.H/2)
		tester.Advance(time.Second)
		if got := tester.HasText("⌘B"); got != probe.hint {
			t.Fatalf("%s hint shown %v, want %v: %#v", probe.name, got, probe.hint, tester.Texts())
		}
		tester.Move(0, 0)
		tester.Advance(time.Second)
	}
}

func TestSwitchToggles(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	selected := false
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		Switch{
			Key: "switch", Label: "Enable pairs", Selected: selected,
			Action: func(value bool) { selected = value },
		}.View(c, semantic)
	}, 160, 80)
	tester.Frame()
	if _, ok := tester.Find("Enable pairs"); !ok {
		t.Fatal("switch is missing")
	}
	if err := tester.Click("Enable pairs"); err != nil {
		t.Fatal(err)
	}
	if !selected {
		t.Fatal("switch did not report its new value")
	}
}
