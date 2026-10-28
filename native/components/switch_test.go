package components

import (
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestSwitchGeometryAndToggle(t *testing.T) {
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
	rect, ok := tester.Find("Enable pairs")
	if !ok || rect.W != theme.DefaultControls.Height+14 {
		t.Fatalf("switch rect = %#v, %v", rect, ok)
	}
	if err := tester.Click("Enable pairs"); err != nil {
		t.Fatal(err)
	}
	if !selected {
		t.Fatal("switch did not report its new value")
	}
}
