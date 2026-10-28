package theme

import "testing"

func TestChromeMetricsScaleFromOneUIFontSize(t *testing.T) {
	t.Parallel()
	controls := DefaultControls.Scaled(20)
	if controls.Height != 40 || controls.Icon != 22.5 || controls.Radius != 10 {
		t.Fatalf("controls = %#v", controls)
	}
	if got := Space16.DIPs(20); got != 20 {
		t.Fatalf("spacing = %v", got)
	}
	if got := TextDefault.Size(20); got != 17.5 {
		t.Fatalf("text = %v", got)
	}
	if got := TextDefault.LineHeight(20); got != 25 {
		t.Fatalf("line height = %v", got)
	}
	if NewProseStyle(20).LineHeight() <= DefaultControls.Height {
		t.Fatal("prose line height collided with chrome geometry")
	}
}

func TestReferenceLayoutKeepsChromeSpacing(t *testing.T) {
	if DefaultLayout.Titlebar != 40 ||
		DefaultLayout.Status != 32 ||
		DefaultLayout.ReadingTop != 64 ||
		DefaultLayout.TitleBottom != 24 ||
		DefaultLayout.ReadingBottom != 150 ||
		DefaultLayout.Tab != 30 ||
		DefaultLayout.DockOffset != 20 {
		t.Fatalf("layout = %#v", DefaultLayout)
	}
}
