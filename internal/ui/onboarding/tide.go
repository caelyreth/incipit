package onboarding

import (
	_ "embed"
	"sync"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// tideSource is the tide's shader: the paper the walkthrough is set on.
//
//go:embed tide.kage
var tideSource string

// tide is the compiled shader and its uniforms, found once.
type tide struct {
	s                                   *shader.Shader
	time, drift, ground, contour, grain shader.Uniform
	vignette                            shader.Uniform
	washes                              [3]shader.Uniform
}

var tideShader = sync.OnceValue(func() *tide {
	s := shader.MustCompile(tideSource)
	return &tide{
		s: s, time: s.Uniform("Time"), drift: s.Uniform("Drift"), ground: s.Uniform("Ground"),
		contour: s.Uniform("Contour"), grain: s.Uniform("Grain"), vignette: s.Uniform("Vignette"),
		washes: [3]shader.Uniform{s.Uniform("Wash0"), s.Uniform("Wash1"), s.Uniform("Wash2")},
	}
})

// tideFrameRate is how often the tide moves: it drifts slowly enough that
// half the display's rate reads as smooth, at half the work.
const tideFrameRate = 30

// paintTide fills r with the tide in semantic's colors: its ground the
// window's, its washes three of the families' surfaces, its contours the
// muted ink. seconds is how long it has run and drift how far the pages
// have moved it; still, it stands as it is.
func paintTide(p *ui.Painter, r ui.Rect, semantic theme.Theme, seconds float64, drift float32, animate bool) {
	t := tideShader()
	v := t.s.NewValues()
	light := semantic.Appearance == theme.Daybreak
	wash, contour, grain, vignette := float32(0.75), float32(0.2), float32(0.035), float32(0.55)
	if light {
		wash, contour, grain, vignette = 0.9, 0.16, 0.03, 0.2
	}
	v.Float(t.time, float32(seconds))
	v.Vec2(t.drift, drift*0.42, -drift*0.27)
	v.Color(t.ground, semantic.Surface.Background)
	for i, family := range []theme.Accent{theme.Afterglow, theme.Regolith, theme.Halo} {
		v.Color(t.washes[i], semantic.Families[family].Surface.Alpha(wash))
	}
	v.Color(t.contour, semantic.Text.Muted.Alpha(contour))
	v.Float(t.grain, grain)
	v.Float(t.vignette, vignette)
	t.s.Draw(p, r, shader.Options{Values: v})
	if animate {
		p.AnimationFrameRate(tideFrameRate)
	}
}
