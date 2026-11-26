package components

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Ink is the hand's mark on a page: a ring drawn around what is chosen,
// a line drawn under a title, a check. Each wavers as a pen does, the
// same way for the same seed, so a mark stays itself from frame to frame,
// and draws itself on as drawn goes from 0 to 1.

// wobble is a smooth, repeatable waver from -1 to 1 along t, for seed.
func wobble(seed, t float32) float32 {
	s := float64(seed)
	x := float64(t)
	return float32(0.55*math.Sin(x*1.7+s*2.3) + 0.3*math.Sin(x*3.9+s*5.1) + 0.15*math.Sin(x*7.3+s*1.3))
}

// InkRing draws a loop around box, a little larger than it, as a pen
// circles a word: it starts at the upper left, goes round once and a bit
// past where it began, and leans as a hand does. The loop is a
// superellipse hugging the box's corners, so it never cuts across what
// it circles. width
// is the pen's.
func InkRing(p *ui.Painter, box ui.Rect, ink ui.Color, width, seed, drawn float32) {
	if drawn <= 0 || box.W <= 0 || box.H <= 0 {
		return
	}
	const steps = 128
	const turn = 1.08 // once round and past the start
	cx, cy := box.X+box.W/2, box.Y+box.H/2
	rx, ry := box.W/2+width*3, box.H/2+width*3
	// 2 would be an ellipse, which cuts a card's corners; 4 and more
	// square the loop toward the box's corners, the more the wider it is.
	// A small mark, as a dot circled, keeps a round loop.
	n := 4 + 2*min(1, max(0, max(box.W, box.H)/min(box.W, box.H)-1)/2)
	if max(box.W, box.H) < 32 {
		n = 2
	}
	var path ui.Path
	start := float32(-2.4) + 0.15*wobble(seed, 0)
	for i := 0; i <= steps; i++ {
		u := float32(i) / steps
		a := float64(start + u*turn*2*math.Pi)
		c, s := math.Cos(a), math.Sin(a)
		ex := float32(math.Copysign(math.Pow(math.Abs(c), 2/float64(n)), c))
		ey := float32(math.Copysign(math.Pow(math.Abs(s), 2/float64(n)), s))
		// The loop opens as it ends, so its two ends pass each other.
		grow := 1 + 0.012*wobble(seed, u*6) + 0.03*u
		x := cx + (rx*grow+width*0.6*wobble(seed+7, u*5))*ex
		y := cy + (ry*grow+width*0.8*wobble(seed+3, u*4))*ey
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	path.Cap(ui.RoundCap).Join(ui.RoundJoin).Trim(0, Smoothstep(drawn))
	p.StrokePath(&path, width, ink)
}

// InkCheck draws a check in box, its short stroke then its long one.
func InkCheck(p *ui.Painter, box ui.Rect, ink ui.Color, width, drawn float32) {
	if drawn <= 0 {
		return
	}
	var path ui.Path
	path.MoveTo(box.X+box.W*0.12, box.Y+box.H*0.55)
	path.QuadTo(box.X+box.W*0.3, box.Y+box.H*0.68, box.X+box.W*0.4, box.Y+box.H*0.86)
	path.QuadTo(box.X+box.W*0.58, box.Y+box.H*0.4, box.X+box.W*0.92, box.Y+box.H*0.12)
	path.Cap(ui.RoundCap).Join(ui.RoundJoin).Trim(0, Smoothstep(drawn))
	p.StrokePath(&path, width, ink)
}

// InkStar draws a small four-pointed spark at (x, y), size across, as a
// margin doodle, twinkling as glow goes from 0 to 1.
func InkStar(p *ui.Painter, x, y, size float32, ink ui.Color, glow float32) {
	r := size / 2 * (0.8 + 0.2*glow)
	w := r * 0.22
	var path ui.Path
	path.MoveTo(x, y-r)
	path.QuadTo(x+w, y-w, x+r, y)
	path.QuadTo(x+w, y+w, x, y+r)
	path.QuadTo(x-w, y+w, x-r, y)
	path.QuadTo(x-w, y-w, x, y-r)
	path.Close()
	p.FillPath(&path, ink.Alpha((0.45+0.55*glow)*float32(ink.A)/255))
}
