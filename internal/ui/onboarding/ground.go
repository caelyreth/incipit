package onboarding

import (
	_ "embed"
	"sync"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

//go:embed paper.kage
var paperSource string

// paper is the ground's shader and its uniforms, found once.
var paper = sync.OnceValue(func() *paperShader {
	s := shader.MustCompile(paperSource)
	return &paperShader{s: s, ground: s.Uniform("Ground"), line: s.Uniform("Line"), wash: s.Uniform("Wash"),
		light: s.Uniform("Light"), drift: s.Uniform("Drift")}
})

type paperShader struct {
	s                                *shader.Shader
	ground, line, wash, light, drift shader.Uniform
}

// paintGround fills r with laid paper in semantic's colors, read each
// frame: the window's background, its lines in the muted ink, its bleed
// Halo's. drift is how far the pages have turned. It moves nothing of its
// own: it is drawn again only in the frames something else asks for, a
// page turning or a theme changing.
func paintGround(p *ui.Painter, r ui.Rect, semantic theme.Theme, drift float32) {
	g := paper()
	line, wash, light := semantic.Text.Muted.Alpha(0.06), semantic.Accent.Muted.Alpha(0.55), float32(0.05)
	if semantic.Appearance == theme.Daybreak {
		line, wash, light = semantic.Text.Muted.Alpha(0.075), semantic.Accent.Muted.Alpha(0.8), 0.03
	}
	v := g.s.NewValues()
	v.Color(g.ground, semantic.Surface.Background)
	v.Color(g.line, line)
	v.Color(g.wash, wash)
	v.Float(g.light, light)
	v.Float(g.drift, drift)
	g.s.Draw(p, r, shader.Options{Values: v})
}
