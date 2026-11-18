package components

import (
	"math"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// DashedOutline strokes a dashed outline width wide just inside r, its
// corners rounded by radius. A border dashed by side breaks its rounded
// corners where two sides' dashes meet; here each corner is the middle of
// a dash that bends round it, and each side has whole dashes and gaps
// between, stretched a little to fit.
func DashedOutline(p *ui.Painter, r ui.Rect, radius, width float32, color ui.Color) {
	inset := width / 2
	x, y, w, h := r.X+inset, r.Y+inset, r.W-width, r.H-width
	radius = max(0, min(radius-inset, w/2, h/2))
	if w <= 0 || h <= 0 {
		return
	}
	dash, gap := 4*width, 3*width
	// The corners' centers, clockwise from the top left, and the straight
	// length of the side after each.
	corners := [4][2]float32{
		{x + radius, y + radius}, {x + w - radius, y + radius},
		{x + w - radius, y + h - radius}, {x + radius, y + h - radius},
	}
	sides := [4]float32{w - 2*radius, h - 2*radius, w - 2*radius, h - 2*radius}
	for side := range 4 {
		// A run goes from the middle of one corner to the middle of the
		// next, beginning and ending in half a dash.
		from := float64(225+90*side) * math.Pi / 180
		length := sides[side] + float32(math.Pi)*radius/2
		n := max(1, float32(math.Round(float64(length/(dash+gap)))))
		scale := length / (n * (dash + gap))
		var run ui.Path
		start := corners[side]
		end := corners[(side+1)%4]
		arc(&run, start, radius, from, from+math.Pi/4, true)
		arc(&run, end, radius, from+math.Pi/4, from+math.Pi/2, false)
		p.StrokePath(run.Cap(ui.ButtCap).Dash(dash*scale/2, dash*scale, gap*scale), width, color)
	}
}

// arc adds a circular arc of radius about center from angle a0 to a1, in
// radians clockwise on the screen, as one cubic: begin starts the path
// there, else a line joins it.
func arc(path *ui.Path, center [2]float32, radius float32, a0, a1 float64, begin bool) {
	point := func(a float64) (float32, float32) {
		return center[0] + radius*float32(math.Cos(a)), center[1] + radius*float32(math.Sin(a))
	}
	x0, y0 := point(a0)
	x1, y1 := point(a1)
	if begin {
		path.MoveTo(x0, y0)
	} else {
		path.LineTo(x0, y0)
	}
	if radius <= 0 {
		return
	}
	k := radius * float32(4.0/3.0*math.Tan((a1-a0)/4))
	path.CubeTo(
		x0-k*float32(math.Sin(a0)), y0+k*float32(math.Cos(a0)),
		x1+k*float32(math.Sin(a1)), y1-k*float32(math.Cos(a1)),
		x1, y1,
	)
}

// DropFrame is where something dragged would land: a dashed outline in the
// accent over its faint tint, as a list's drop place and the page's show.
// shown fades it in and out.
func DropFrame(p *ui.Painter, r ui.Rect, radius, shown float32, t theme.Theme) {
	if r.W <= 0.5 || r.H <= 0.5 || shown <= 0 {
		return
	}
	p.Fill(r, t.Accent.Hint.Alpha(shown), radius)
	DashedOutline(p, r, radius, 1, t.Accent.Foreground.Alpha(0.65*shown))
}
