package theme

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// contrast is the WCAG 2 contrast ratio of two opaque colors.
func contrast(a, b ui.Color) float64 {
	luminance := func(c ui.Color) float64 {
		channel := func(v uint8) float64 {
			x := float64(v) / 255
			if x <= 0.03928 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
	}
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// A chosen row stands out from the panel by about as much as a family's
// surface on it stands out from the row, so neither vanishes.
func TestAChosenRowSitsMidwayToTheSurfaces(t *testing.T) {
	for _, appearance := range []Appearance{Starfield, Daybreak} {
		settings := DefaultSettings()
		settings.Appearance = appearance
		th := settings.Semantic()
		row := contrast(th.Surface.Chosen, th.Surface.Panel)
		for accent, family := range th.Families {
			on := contrast(family.Surface, th.Surface.Chosen)
			if row < 1.08 || on < 1.08 {
				t.Errorf("appearance %d, family %d: row %.2f from the panel, surface %.2f from the row", appearance, accent, row, on)
			}
		}
	}
}
