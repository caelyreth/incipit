package shell

import (
	"image"
	"image/color"
	"sync"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

const textureTileSize = 256

var (
	starfieldTextureTile = sync.OnceValue(func() *ui.Bitmap {
		return ui.NewBitmap(buildTextureTile(textureTileSize, 0x9e3779b97f4a7c15, color.NRGBA{R: 218, G: 224, B: 238}, 7, 30))
	})
	daybreakTextureTile = sync.OnceValue(func() *ui.Bitmap {
		return ui.NewBitmap(buildTextureTile(textureTileSize, 0xbf58476d1ce4e5b9, color.NRGBA{R: 68, G: 62, B: 54}, 8, 35))
	})
)

func buildTextureTile(size int, seed uint64, tint color.NRGBA, maxAlpha uint8, densityPct uint64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	state := seed
	next := func() uint64 {
		state += 0x9e3779b97f4a7c15
		z := state
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		return z ^ (z >> 31)
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			roll := next()
			if roll%100 >= densityPct {
				continue
			}
			a := 1 + uint8((roll>>8)%uint64(maxAlpha))
			img.SetNRGBA(x, y, color.NRGBA{R: tint.R, G: tint.G, B: tint.B, A: a})
		}
	}
	return img
}

func textureBitmap(appearance theme.Appearance) *ui.Bitmap {
	if appearance == theme.Daybreak {
		return daybreakTextureTile()
	}
	return starfieldTextureTile()
}

func paintTexture(p *ui.Painter, r ui.Rect, appearance theme.Appearance) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	bmp := textureBitmap(appearance)
	if bmp == nil {
		return
	}
	tile := float32(textureTileSize)
	for y := r.Y; y < r.Y+r.H; y += tile {
		h := min(tile, r.Y+r.H-y)
		for x := r.X; x < r.X+r.W; x += tile {
			w := min(tile, r.X+r.W-x)
			p.ImageRegion(bmp, ui.Rect{W: w, H: h}, ui.Rect{X: x, Y: y, W: w, H: h}, ui.ImageOptions{})
		}
	}
}
