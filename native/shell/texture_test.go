package shell

import (
	"image/color"
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestTextureTilesAreDeterministicAndSubtle(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seed    uint64
		tint    color.NRGBA
		maxA    uint8
		density uint64
	}{
		{"starfield", 0x9e3779b97f4a7c15, color.NRGBA{R: 218, G: 224, B: 238}, 7, 30},
		{"daybreak", 0xbf58476d1ce4e5b9, color.NRGBA{R: 68, G: 62, B: 54}, 8, 35},
	} {
		a := buildTextureTile(textureTileSize, tc.seed, tc.tint, tc.maxA, tc.density)
		b := buildTextureTile(textureTileSize, tc.seed, tc.tint, tc.maxA, tc.density)
		if bnds := a.Bounds(); bnds.Dx() != textureTileSize || bnds.Dy() != textureTileSize {
			t.Fatalf("%s bounds = %v, want %dx%d", tc.name, bnds, textureTileSize, textureTileSize)
		}
		nonZero := 0
		for y := 0; y < textureTileSize; y++ {
			for x := 0; x < textureTileSize; x++ {
				pa, pb := a.NRGBAAt(x, y), b.NRGBAAt(x, y)
				if pa != pb {
					t.Fatalf("%s not deterministic at (%d,%d): %v vs %v", tc.name, x, y, pa, pb)
				}
				if pa.A > 10 {
					t.Fatalf("%s pixel alpha %d exceeds subtle ceiling at (%d,%d)", tc.name, pa.A, x, y)
				}
				if pa.A > 0 {
					nonZero++
				}
			}
		}
		if nonZero == 0 {
			t.Fatalf("%s produced empty grain tile", tc.name)
		}
	}
}

func TestPaintTextureTilesAcrossArbitraryRect(t *testing.T) {
	for _, app := range []theme.Appearance{theme.Starfield, theme.Daybreak} {
		settings := theme.DefaultSettings()
		settings.Appearance = app
		sem := settings.Semantic()
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(sem.Surface.Background).Draw(func(p *ui.Painter, r ui.Rect) {
				paintTexture(p, r, app)
			})
		}, 300, 300)
		tt.Frame()
		img := tt.Image()
		first := img.RGBAAt(0, 0)
		var modMain, modEdge bool
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				c := img.RGBAAt(x, y)
				if c != first {
					if x >= textureTileSize || y >= textureTileSize {
						modEdge = true
					} else {
						modMain = true
					}
				}
			}
		}
		if !modMain || !modEdge {
			t.Fatalf("%v texture did not tile across 300x300 (modMain=%v modEdge=%v)", app, modMain, modEdge)
		}
	}
}

func TestRootBackgroundTextureHonorsSetting(t *testing.T) {
	render := func(textureOn bool) *ui.Tester {
		s := &State{Settings: theme.DefaultSettings()}
		s.Appearance.Texture = textureOn
		tt := ui.NewTester(s.View, 300, 180)
		tt.Frame()
		return tt
	}

	flat := render(false).Image()
	textured := render(true).Image()
	base := flat.RGBAAt(0, 0)
	var differed int
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			if got := flat.RGBAAt(x, y); got != base {
				t.Fatalf("expected uniform corner backdrop with Texture=false at (%d,%d): %v vs %v", x, y, got, base)
			}
			if textured.RGBAAt(x, y) != base {
				differed++
			}
		}
	}
	if differed == 0 {
		t.Fatal("expected Texture=true on shell root to apply subtle grain to corner backdrop")
	}
}
