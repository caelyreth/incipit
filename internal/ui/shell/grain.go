package shell

import (
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Grain is the material of the desk's ground: a tile baked once, a
// seamless mask of sparse specks, thicker and thinner as a slow cloud has
// them, drawn a pixel of the screen to a pixel of the tile, moving with the
// desk. The field's ground stays flat: grain under it, with its cards
// moving over, made the field lag. Its tone is only in how many specks
// there are, never in a faint fill, which eight bits of alpha would band.
// The theme gives it its ink: light specks in Starfield, dark in Daybreak.
type grain struct {
	// size is the tile's side, in pixels.
	size int
	// specks is the share of pixels that are a speck, on average; cloud
	// how far the cloud, of cells cells across the tile, thins and thickens
	// them, 0 for even.
	specks, cloud float64
	cells         int
	seed          uint32
}

// groundGrain is sparse, drifting in a slow cloud, on a tile large enough
// that its repeat does not show.
var groundGrain = grain{size: 512, specks: 0.05, cloud: 0.8, cells: 5, seed: 0x6e766c64}

var groundTile = sync.OnceValue(func() *ui.Bitmap { return groundGrain.bake() })

// bake draws g's tile: white, its alpha the grain, so the ink it is drawn
// in colors it.
func (g grain) bake() *ui.Bitmap {
	n := g.size
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	cell := float64(n) / float64(g.cells)
	for y := range n {
		for x := range n {
			density := g.specks * (1 + g.cloud*(2*cloud(float64(x)/cell, float64(y)/cell, g.cells, g.seed)-1))
			h := hash(uint32(x), uint32(y), g.seed)
			if float64(h&0xffff)/0x10000 >= density {
				continue
			}
			a := 0.4 + 0.6*float64(h>>16)/0x10000
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(math.Round(255 * a))})
		}
	}
	return ui.NewBitmap(img).Pixelated()
}

// cloud is smooth value noise at (x, y), in cells, repeating every period
// cells both ways, so the tile meets itself: 0 to 1.
func cloud(x, y float64, period int, seed uint32) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := smooth(x-x0), smooth(y-y0)
	at := func(i, j float64) float64 {
		p := uint32(period)
		return float64(hash(uint32(int(i))%p, uint32(int(j))%p, seed^0x9e3779b9)&0xffff) / 0xffff
	}
	top := at(x0, y0) + (at(x0+1, y0)-at(x0, y0))*fx
	bottom := at(x0, y0+1) + (at(x0+1, y0+1)-at(x0, y0+1))*fx
	return top + (bottom-top)*fy
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

// hash is a PCG hash of (x, y) under seed.
func hash(x, y, seed uint32) uint32 {
	pcg := func(v uint32) uint32 {
		s := v*747796405 + 2891336453
		w := ((s >> ((s >> 28) + 4)) ^ s) * 277803737
		return (w >> 22) ^ w
	}
	return pcg(x ^ pcg(y^pcg(seed)))
}

// grainInk is the ink the ground's grain is drawn in.
func grainInk(semantic theme.Theme) ui.Color {
	strength := float32(0.16)
	if semantic.Appearance == theme.Daybreak {
		strength = 0.2
	}
	return semantic.Text.Default.Alpha(strength)
}

// paintGrain tiles r, rounded by radius, with tile in ink, a tile's corner
// standing at at.
func paintGrain(p *ui.Painter, r ui.Rect, radius float32, tile *ui.Bitmap, ink ui.Color, at ui.Point) {
	if r.W <= 0 || r.H <= 0 || ink.A == 0 {
		return
	}
	scale := max(p.Scale(), 1)
	px, _ := tile.Size()
	side := float32(px) / scale
	from := func(edge, at float32) float32 {
		v := at + float32(math.Floor(float64((edge-at)/side)))*side
		return float32(math.Round(float64(v*scale))) / scale
	}
	x0, y0 := from(r.X, at.X), from(r.Y, at.Y)
	p.Clip(r, radius, func() {
		for y := y0; y < r.Y+r.H; y += side {
			for x := x0; x < r.X+r.W; x += side {
				p.ImageRegion(tile, ui.Rect{}, ui.Rect{X: x, Y: y, W: side, H: side}, ui.ImageOptions{Tint: ink})
			}
		}
	})
}
