package components

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/caelyreth/neverland/assets"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The brand mark is the profile of the application icon, drawn from its
// own vector (assets.Profile) rather than a picture of it, so it is crisp
// at any size and takes any ink.

// brandShape is one filled path of the mark: its data, read again at the
// size it is drawn (pathData), and its fill rule.
type brandShape struct {
	data    string
	evenOdd bool
}

var brandArtwork = sync.OnceValues(func() (brandArt, error) { return parseBrand(assets.Profile) })

type brandArt struct {
	w, h   float32
	shapes []brandShape
}

//go:embed soak.kage
var soakSource string

// soak is the shader the mark spreads in with, as ink into paper.
var soak = sync.OnceValues(func() (*shader.Shader, shader.Uniform) {
	s := shader.MustCompile(soakSource)
	return s, s.Uniform("Drawn")
})

// BrandAspect is the mark's width over its height.
func BrandAspect() float32 {
	art, err := brandArtwork()
	if err != nil || art.h == 0 {
		return 1
	}
	return art.w / art.h
}

// BrandMark is the mark as a view keeps it: its curves read once at the
// size it is drawn, so each frame fills the same shapes, whose masks are
// kept; and, while it spreads onto the page, a canvas the spreading reads.
type BrandMark struct {
	paths  []*ui.Path
	px     float32
	canvas *ui.Canvas
}

// Paint draws the mark centered in box in ink. drawn, from 0 to 1, is how
// far it has spread onto the page, as ink soaks into paper; 1 is the mark
// as the icon has it.
func (b *BrandMark) Paint(p *ui.Painter, box ui.Rect, ink ui.Color, drawn float32) {
	art, err := brandArtwork()
	if err != nil || art.w == 0 || box.W <= 0 || box.H <= 0 || drawn <= 0 {
		return
	}
	s := min(box.W/art.w, box.H/art.h)
	at := ui.Rect{X: box.X + (box.W-art.w*s)/2, Y: box.Y + (box.H-art.h*s)/2, W: art.w * s, H: art.h * s}
	w := max(1, int(math.Ceil(float64(at.W*p.Scale()))))
	h := max(1, int(math.Ceil(float64(at.H*p.Scale()))))
	// The curves are read at the size they are drawn, in pixels, which
	// their masks are cut at, rather than drawn small and scaled.
	if px := float32(w) / art.w; px != b.px {
		b.px, b.paths = px, b.paths[:0]
		for _, shape := range art.shapes {
			path, err := pathData(shape.data, px)
			if err != nil {
				continue
			}
			if shape.evenOdd {
				path.FillRule(ui.EvenOdd)
			}
			b.paths = append(b.paths, path)
		}
	}
	fill := func(fp *ui.Painter) {
		for _, path := range b.paths {
			fp.FillPath(path, ink)
		}
	}
	if drawn >= 1 {
		b.canvas = nil
		k := 1 / p.Scale()
		p.Transform(ui.Scaling(k, k).Then(ui.Translation(at.X, at.Y)), 0, 0, func() { fill(p) })
		return
	}
	if b.canvas == nil {
		b.canvas = ui.NewCanvas(w, h)
	} else if cw, ch := b.canvas.Size(); cw != w || ch != h {
		b.canvas = ui.NewCanvas(w, h)
	}
	b.canvas.Draw(p, fill)
	fx, amount := soak()
	v := fx.NewValues()
	v.Float(amount, drawn)
	fx.Draw(p, at, shader.Options{Values: v, Images: [4]*ui.Bitmap{b.canvas.Bitmap()}})
}

// parseBrand reads the mark's SVG: its view box and its paths, which use
// absolute moves, lines and cubic curves alone.
func parseBrand(data []byte) (brandArt, error) {
	var doc struct {
		Width  string `xml:"width,attr"`
		Height string `xml:"height,attr"`
		Paths  []struct {
			D        string `xml:"d,attr"`
			FillRule string `xml:"fill-rule,attr"`
		} `xml:"path"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return brandArt{}, err
	}
	w, err1 := strconv.ParseFloat(doc.Width, 32)
	h, err2 := strconv.ParseFloat(doc.Height, 32)
	if err1 != nil || err2 != nil {
		return brandArt{}, fmt.Errorf("brand: size %q×%q", doc.Width, doc.Height)
	}
	art := brandArt{w: float32(w), h: float32(h)}
	for _, el := range doc.Paths {
		if _, err := ParsePathData(el.D); err != nil {
			return brandArt{}, err
		}
		art.shapes = append(art.shapes, brandShape{data: el.D, evenOdd: el.FillRule == "evenodd"})
	}
	return art, nil
}

// ParsePathData reads an SVG path's data, of moves, lines, horizontal and
// vertical lines, cubic curves and closes, absolute or relative.
func ParsePathData(d string) (*ui.Path, error) { return pathData(d, 1) }

// pathData is ParsePathData with every coordinate scaled by k.
func pathData(d string, k float32) (*ui.Path, error) {
	var path ui.Path
	nums, cmd := []float32{}, byte(0)
	var cx, cy, sx, sy float32
	flush := func() error {
		need := map[byte]int{'M': 2, 'L': 2, 'H': 1, 'V': 1, 'C': 6, 'Z': 0}
		upper := cmd &^ 0x20
		n, ok := need[upper]
		if !ok {
			return fmt.Errorf("path: command %q", cmd)
		}
		if n == 0 {
			path.Close()
			cx, cy = sx, sy
			return nil
		}
		if len(nums) == 0 || len(nums)%n != 0 {
			return fmt.Errorf("path: %d numbers for %q", len(nums), cmd)
		}
		rel := cmd != upper
		for i := 0; i < len(nums); i += n {
			v := nums[i : i+n]
			ox, oy := float32(0), float32(0)
			if rel {
				ox, oy = cx, cy
			}
			switch upper {
			case 'M':
				cx, cy = v[0]+ox, v[1]+oy
				if i == 0 {
					path.MoveTo(cx, cy)
					sx, sy = cx, cy
				} else {
					path.LineTo(cx, cy)
				}
			case 'L':
				cx, cy = v[0]+ox, v[1]+oy
				path.LineTo(cx, cy)
			case 'H':
				cx = v[0] + ox
				path.LineTo(cx, cy)
			case 'V':
				cy = v[0] + oy
				path.LineTo(cx, cy)
			case 'C':
				path.CubeTo(v[0]+ox, v[1]+oy, v[2]+ox, v[3]+oy, v[4]+ox, v[5]+oy)
				cx, cy = v[4]+ox, v[5]+oy
			}
		}
		return nil
	}
	i := 0
	for i < len(d) {
		ch := d[i]
		switch {
		case ch == ' ' || ch == ',' || ch == '\n' || ch == '\t' || ch == '\r':
			i++
		case pathCommand(ch):
			if cmd != 0 {
				if err := flush(); err != nil {
					return nil, err
				}
			}
			cmd, nums = ch, nums[:0]
			i++
		default:
			j := i + 1
			for j < len(d) && (d[j] >= '0' && d[j] <= '9' || d[j] == '.' || d[j] == 'e' || d[j] == 'E' ||
				(d[j] == '-' && (d[j-1] == 'e' || d[j-1] == 'E'))) {
				// A second point starts a new number, as "1.5.5".
				if d[j] == '.' && strings.IndexByte(d[i:j], '.') >= 0 {
					break
				}
				j++
			}
			v, err := strconv.ParseFloat(d[i:j], 32)
			if err != nil {
				return nil, fmt.Errorf("path: number %q", d[i:j])
			}
			nums = append(nums, float32(v)*k)
			i = j
		}
	}
	if cmd != 0 {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	return &path, nil
}

// pathCommand is whether ch is a command of the path data ParsePathData
// reads.
func pathCommand(ch byte) bool {
	switch ch &^ 0x20 {
	case 'M', 'L', 'H', 'V', 'C', 'Z':
		return true
	}
	return false
}
