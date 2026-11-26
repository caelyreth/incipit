package onboarding

import (
	_ "embed"
	"math"
	"sync"
	"time"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The tide is the walkthrough's ground, in two passes. The current, noise
// bending noise, holds nothing sharp, so it is drawn small into a canvas
// (tideScale of the window across), and only as often as it visibly
// moves (tideRate); the paper spreads it over the window each frame,
// one sample and a grain a pixel, which costs next to nothing.

//go:embed tide.kage
var tideSource string

//go:embed paper.kage
var paperSource string

const (
	// tideScale is the current's canvas across the window: a sixth.
	tideScale = 6
	// tideRate is how often a second the current moves: it drifts slowly
	// enough that a dozen steps a second read as smooth.
	tideRate = 12
)

// shaders are the tide's two passes and their uniforms, found once.
type shaders struct {
	tide, paper                *shader.Shader
	time, drift, ground, grain shader.Uniform
	vignette                   shader.Uniform
	washes                     [3]shader.Uniform
}

var tideShaders = sync.OnceValue(func() *shaders {
	t, p := shader.MustCompile(tideSource), shader.MustCompile(paperSource)
	return &shaders{
		tide: t, paper: p, time: t.Uniform("Time"), drift: t.Uniform("Drift"), ground: t.Uniform("Ground"),
		washes: [3]shader.Uniform{t.Uniform("Wash0"), t.Uniform("Wash1"), t.Uniform("Wash2")},
		grain:  p.Uniform("Grain"), vignette: p.Uniform("Vignette"),
	}
})

// tide is the current's canvas and what it was last drawn with, so it
// is drawn again only when the tide has moved or its colors changed.
type tide struct {
	canvas *ui.Canvas
	at     float64
	drift  float32
	ground ui.Color
	washes [3]ui.Color
}

// paint fills r with the tide in semantic's colors: the paper the
// window's ground, washed in the accent's family. seconds is how long it
// has run, drift how far the pages have moved it; still, it stands.
func (td *tide) paint(p *ui.Painter, r ui.Rect, semantic theme.Theme, seconds float64, drift float32, animate bool) {
	sh := tideShaders()
	light := semantic.Appearance == theme.Daybreak
	halo := semantic.Families[theme.Halo]
	washes := [3]ui.Color{halo.Surface.Alpha(0.85), halo.Muted.Alpha(0.35), halo.Foreground.Alpha(0.1)}
	grain, vignette := float32(0.035), float32(0.5)
	if light {
		washes = [3]ui.Color{halo.Surface.Alpha(0.9), halo.Muted.Alpha(0.22), halo.Foreground.Alpha(0.08)}
		grain, vignette = 0.03, 0.18
	}
	w := max(1, int(math.Ceil(float64(r.W*p.Scale()/tideScale))))
	h := max(1, int(math.Ceil(float64(r.H*p.Scale()/tideScale))))
	if !animate {
		seconds = 0
	}
	// The current steps tideRate times a second, whatever the display's.
	step := math.Floor(seconds*tideRate) / tideRate
	fresh := td.canvas == nil
	if !fresh {
		cw, ch := td.canvas.Size()
		fresh = cw != w || ch != h
	}
	if fresh {
		td.canvas = ui.NewCanvas(w, h)
	}
	// Moving, it draws again at its next step alone, whatever else moves
	// meanwhile (the drift between pages, a theme's colors): they catch up
	// there. Still, it draws when its colors change.
	recolored := semantic.Surface.Background != td.ground || washes != td.washes
	if fresh || step != td.at || !animate && (recolored || drift != td.drift) {
		td.at, td.drift, td.ground, td.washes = step, drift, semantic.Surface.Background, washes
		v := sh.tide.NewValues()
		v.Float(sh.time, float32(step))
		v.Vec2(sh.drift, drift*0.42, -drift*0.27)
		v.Color(sh.ground, semantic.Surface.Background)
		for i, c := range washes {
			v.Color(sh.washes[i], c)
		}
		td.canvas.Draw(p, func(cp *ui.Painter) {
			sh.tide.Draw(cp, ui.Rect{W: float32(w), H: float32(h)}, shader.Options{Values: v})
		})
	}
	v := sh.paper.NewValues()
	v.Float(sh.grain, grain)
	v.Float(sh.vignette, vignette)
	sh.paper.Draw(p, r, shader.Options{Values: v, Images: [4]*ui.Bitmap{td.canvas.Bitmap()}})
	if animate {
		next := time.Duration((step + 1.0/tideRate - seconds) * float64(time.Second))
		p.After(max(next, time.Millisecond))
	}
}
