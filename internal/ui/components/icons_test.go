package components

import "testing"

func TestSourceIconsLoadAtOneOpticalSize(t *testing.T) {
	seen := make(map[any]IconName, len(AllIcons))
	for _, name := range AllIcons {
		icon := name.SVG()
		if icon == nil {
			t.Fatalf("%s did not load", name)
		}
		if width, height := icon.Size(); width != 24 || height != 24 {
			t.Fatalf("%s size = %vx%v", name, width, height)
		}
		if previous, duplicate := seen[icon]; duplicate {
			t.Fatalf("%s and %s share one SVG", previous, name)
		}
		seen[icon] = name
	}
}

func TestGroupIconsLoad(t *testing.T) {
	for _, name := range GroupIcons {
		if icon := name.SVG(); icon == nil {
			t.Fatalf("%s did not load", name)
		}
	}
}
