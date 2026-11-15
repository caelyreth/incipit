package lab

import (
	"image"
	"math"
	"slices"

	"github.com/egoist/mygo/ui"
)

// Lightness is a color's OKLab lightness, from 0 for black to 1 for white:
// the scale the lab's ΔL figures are hundredths of.
func Lightness(c ui.Color) float64 {
	l, _, _ := oklch(c.R, c.G, c.B)
	return l
}

// tinted is the chroma above which a pixel counts as colored: above the
// near-neutral surfaces' (0.005) and the faint fills of today's (0.026).
const tinted = 0.03

// hueBin is the width of the bins a hue is counted in, in degrees.
const hueBin = 30

// enough is the share of a region's colored pixels a hue must hold to
// count as one of its hues, so the fringe of an antialiased glyph does
// not count as a hue of its own.
const enough = 0.08

// Reading is how a region of a rendered window spends its color: the
// share of its pixels that are colored, and the hues they fall in, each
// the middle of its 30° bin, most pixels first.
type Reading struct {
	Area float64
	Hues []int
}

// Measure reads the color of r in img, as the color budget counts it.
func Measure(img image.Image, r image.Rectangle) Reading {
	r = r.Intersect(img.Bounds())
	total := r.Dx() * r.Dy()
	if total == 0 {
		return Reading{}
	}
	colored := 0
	bins := map[int]int{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			_, chroma, hue := oklch(uint8(cr>>8), uint8(cg>>8), uint8(cb>>8))
			if chroma <= tinted {
				continue
			}
			colored++
			bins[int(math.Mod(hue+hueBin/2, 360))/hueBin*hueBin]++
		}
	}
	var hues []int
	for hue, n := range bins {
		if float64(n) >= enough*float64(colored) {
			hues = append(hues, hue)
		}
	}
	slices.SortFunc(hues, func(a, b int) int { return bins[b] - bins[a] })
	return Reading{Area: float64(colored) / float64(total), Hues: hues}
}

// oklch is an sRGB color's OKLab lightness, its chroma and its hue in
// degrees.
func oklch(r8, g8, b8 uint8) (l, c, h float64) {
	r, g, b := linear(r8), linear(g8), linear(b8)
	lc := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	mc := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	sc := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	l = 0.2104542553*lc + 0.7936177850*mc - 0.0040720468*sc
	a := 1.9779984951*lc - 2.4285922050*mc + 0.4505937099*sc
	bb := 0.0259040371*lc + 0.7827717662*mc - 0.8086757660*sc
	return l, math.Hypot(a, bb), math.Mod(math.Atan2(bb, a)*180/math.Pi+360, 360)
}

func linear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
