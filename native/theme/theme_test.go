package theme

import "testing"

func TestProseMetricsKeepReferenceProportionsAndClamp(t *testing.T) {
	t.Parallel()
	prose := NewProseStyle(19)
	if prose.FontSize != 19 || prose.LineHeight() != 34 || prose.CaretHeight() != 25 ||
		prose.CaretOffsetY() != 1 ||
		prose.Column() != 700 {
		t.Fatalf("default prose = %#v", prose)
	}
	if got := NewProseStyle(4).FontSize; got != MinProseFontSize {
		t.Fatalf("minimum = %v", got)
	}
	if got := NewProseStyle(90).FontSize; got != MaxProseFontSize {
		t.Fatalf("maximum = %v", got)
	}
	if got := NewProseStyle(26).CaretOffsetY(); got <= 1 || got > 1.5 {
		t.Fatalf("scaled caret offset = %v", got)
	}
}

func TestMyGoThemeCarriesSemanticMaterial(t *testing.T) {
	t.Parallel()
	settings := DefaultSettings()
	semantic := settings.Semantic()
	native := settings.MyGoTheme()
	if semantic.Text.Default != native.Text || semantic.Surface.Background != native.Background {
		t.Fatalf("semantic theme did not reach MyGo")
	}
	if native.FontSize != 16 || native.Spacing != 4 || native.Radius != 8 {
		t.Fatalf("metrics = font %v spacing %v radius %v", native.FontSize, native.Spacing, native.Radius)
	}
}

// A scheme is what the appearances paint with; without one they paint
// Cosmofi's.
func TestASchemeGivesTheAppearancesTheirPalettes(t *testing.T) {
	t.Parallel()
	settings := DefaultSettings()
	if settings.Palette() != Cosmofi().Starfield {
		t.Fatal("the default is not Cosmofi's night")
	}
	night := Cosmofi().Starfield
	night.Page = RGB(0x101010)
	day := Cosmofi().Daybreak
	day.Page = RGB(0xfefefe)
	settings.Scheme = &Scheme{Name: "Probe", Starfield: night, Daybreak: day}
	if got := settings.Semantic().Surface.Page; got != RGB(0x101010) {
		t.Fatalf("the night page is %v", got)
	}
	settings.Appearance = Daybreak
	if got := settings.Semantic().Surface.Page; got != RGB(0xfefefe) {
		t.Fatalf("the day page is %v", got)
	}
	if got := settings.Semantic().Accent.OnAccent; got != day.Background {
		t.Fatalf("on the accent is %v, not the day's ground", got)
	}
}
