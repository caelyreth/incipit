package onboarding

import (
	_ "embed"
	"sync"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

//go:embed halftone.kage
var halftoneSource string

// halftone is the ground's shader and its uniforms, found once.
var halftone = sync.OnceValue(func() *groundShader {
	s := shader.MustCompile(halftoneSource)
	return &groundShader{s: s, ground: s.Uniform("Ground"), dot: s.Uniform("Dot"), cell: s.Uniform("Cell"), drift: s.Uniform("Drift")}
})

type groundShader struct {
	s                        *shader.Shader
	ground, dot, cell, drift shader.Uniform
}

// paintGround fills r with the halftone ground in semantic's colors: the
// window's background, its dots in Halo's ink. drift is how far the pages
// have turned. It moves nothing of its own: it is drawn again only in the
// frames something else asks for, a page turning or a theme changing.
func paintGround(p *ui.Painter, r ui.Rect, semantic theme.Theme, drift float32) {
	g := halftone()
	dot := semantic.Accent.Foreground.Alpha(0.2)
	if semantic.Appearance == theme.Daybreak {
		dot = semantic.Accent.Foreground.Alpha(0.24)
	}
	v := g.s.NewValues()
	v.Color(g.ground, semantic.Surface.Background)
	v.Color(g.dot, dot)
	v.Float(g.cell, 8)
	v.Float(g.drift, drift)
	g.s.Draw(p, r, shader.Options{Values: v})
}
