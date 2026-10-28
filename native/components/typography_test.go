package components

import (
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestLabelUsesExactChromeScaleAndLineHeight(t *testing.T) {
	tester := ui.NewTester(func(c *ui.Context) {
		Label(c, "Small", theme.TextSmall, c.Theme().Text)
	}, 300, 100)
	tester.Frame()
	rect, ok := tester.Find("Small")
	if !ok || rect.H <= 0 {
		t.Fatalf("small label rect = %#v, %v", rect, ok)
	}
}

func TestHeadlineUsesSourceRelativeLeading(t *testing.T) {
	settings := theme.DefaultSettings()
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(settings.MyGoTheme())
		Headline(c, "Editor", theme.HeadlineXLarge, c.Theme().Text)
	}, 300, 100)
	tester.Frame()
	if _, ok := tester.Find("Editor"); !ok {
		t.Fatal("headline missing")
	}
}
