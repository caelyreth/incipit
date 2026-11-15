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

// lighter reports whether a is lighter than b.
func lighter(a, b ui.Color) bool {
	return contrast(a, ui.RGB(0, 0, 0)) > contrast(b, ui.RGB(0, 0, 0))
}

// The ladder keeps its order in both appearances: a chosen row is louder
// than a row under the pointer, both visible on the panel, and the page and
// what is raised stand lighter than the panel, never sinking into it.
func TestTheLadderKeepsItsOrder(t *testing.T) {
	for _, appearance := range []Appearance{Starfield, Daybreak} {
		settings := DefaultSettings()
		settings.Appearance = appearance
		th := settings.Semantic()
		chosen := contrast(th.Surface.Chosen, th.Surface.Panel)
		hover := contrast(th.Element.Hover, th.Surface.Panel)
		if chosen <= hover || hover < 1.03 {
			t.Errorf("appearance %d: chosen %.3f from the panel, hover %.3f", appearance, chosen, hover)
		}
		if !lighter(th.Surface.Page, th.Surface.Panel) || !lighter(th.Surface.Elevated, th.Surface.Panel) {
			t.Errorf("appearance %d: the page or what is raised sinks into the panel", appearance)
		}
		for accent, family := range th.Families {
			if on := contrast(family.Surface, th.Surface.Page); on < 1.08 {
				t.Errorf("appearance %d, family %d: the surface is %.2f from the page", appearance, accent, on)
			}
		}
	}
}
