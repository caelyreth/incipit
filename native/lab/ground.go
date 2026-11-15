package lab

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// Ground is the candidate for the weight at the foot of the library: a
// horizon in ordered dither, its ink growing denser toward the foot, the
// line it rises from rolling a little across the pane. Density is how
// much of the band is inked, from 0 for none to 1, the figure a week of
// writing would set; it is a still texture, painted once a frame like
// any fill, so it costs nothing at rest.
type Ground struct {
	Density float32
	Ink     ui.Color
	// Seed sets where the horizon rolls, so each notebook keeps its own.
	Seed float32
}

// groundDots is the size of one dot of the dither, in DIPs.
const groundDots = 2

// groundShader inks a dot where the 8×8 Bayer threshold at it falls under
// the band's density there. The dots are whole device pixels, so the
// texture stays crisp at any scale. Below the rolling horizon the density
// rises to the foot; above it nothing is inked.
var groundShader = shader.MustCompile(`package main

var Ink vec4
var Density float
var Seed float
var Dots float

func bayer2(a vec2) float {
	a = floor(a)
	return fract(a.x/2 + a.y*a.y*0.75)
}

func bayer8(a vec2) float {
	return (bayer2(0.25*a)*0.25+bayer2(0.5*a))*0.25 + bayer2(a)
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	side := max(1, floor(Dots*pixelScale()+0.5))
	cell := floor(dst.xy / side)
	x := pos.x / max(size.x, 1)
	horizon := 0.30 + 0.10*sin(x*5.1+Seed) + 0.05*sin(x*13.3+Seed*2.7)
	t := clamp((pos.y/max(size.y, 1)-horizon)/max(1-horizon, 0.001), 0, 1)
	d := Density * t * t * (3 - 2*t)
	if bayer8(cell) < d {
		return vec4(Ink.rgb*Ink.a, Ink.a)
	}
	return vec4(0)
}
`)

// Paint draws the ground in r.
func (g Ground) Paint(p *ui.Painter, r ui.Rect) {
	groundShader.Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Ink":     g.Ink,
		"Density": g.Density,
		"Seed":    g.Seed,
		"Dots":    float32(groundDots),
	}})
}
