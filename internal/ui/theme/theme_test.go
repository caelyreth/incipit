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

func TestMathFontEmbedded(t *testing.T) {
	if len(MathFont()) == 0 {
		t.Fatal("missing embedded math font")
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

func TestMyGoThemeTurnsDarkHalfwayThroughABlend(t *testing.T) {
	t.Parallel()
	light := DefaultSettings()
	light.Appearance = Daybreak
	dark := light
	dark.Appearance = Starfield
	// Daybreak's settings painting a blend a third of the way from
	// Starfield: still dark.
	blend := light.Semantic()
	from, to := dark.Semantic().Surface.Background, light.Semantic().Surface.Background
	blend.Surface.Background = from.Mix(to, 0.3)
	if !light.MyGoThemeFor(blend).Dark {
		t.Error("a third of the way to Daybreak, the controls turned light")
	}
	blend.Surface.Background = from.Mix(to, 0.7)
	if dark.MyGoThemeFor(blend).Dark {
		t.Error("most of the way to Daybreak, the controls stayed dark")
	}
}
