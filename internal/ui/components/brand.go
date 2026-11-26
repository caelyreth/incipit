package components

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"image"
	"image/draw"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/caelyreth/neverland/assets"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"golang.org/x/image/vector"
)

// The brand mark is the profile of the application icon, drawn from its
// own vector (assets.Profile) rather than a picture of it, so it is crisp
// at any size and takes any ink. It is rasterized once at the size it is
// shown, in the display's pixels, into a coverage bitmap of one piece: so
// large a shape filled as a path each frame is cut into tiles whose edges
// can show as seams, and costs its curves every frame. Each frame then
// draws one quad, inked by the stamp shader.

// brandShape is one filled path of the mark: its data and its fill rule.
type brandShape struct {
	data    string
	evenOdd bool
}

var brandArtwork = sync.OnceValues(func() (brandArt, error) { return parseBrand(assets.Profile) })

type brandArt struct {
	w, h   float32
	shapes []brandShape
}

//go:embed stamp.kage
var stampSource string

// stamp is the shader the mark is inked and spread with, and its
// uniforms.
var stamp = sync.OnceValue(func() *stampShader {
	s := shader.MustCompile(stampSource)
	return &stampShader{s: s, ink: s.Uniform("Ink"), drawn: s.Uniform("Drawn")}
})

type stampShader struct {
	s          *shader.Shader
	ink, drawn shader.Uniform
}

// BrandAspect is the mark's width over its height.
func BrandAspect() float32 {
	art, err := brandArtwork()
	if err != nil || art.h == 0 {
		return 1
	}
	return art.w / art.h
}

// BrandMark is the mark as a view keeps it: its coverage, rasterized at
// the pixels it was last shown at.
type BrandMark struct {
	bitmap *ui.Bitmap
	w      int
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
	if b.bitmap == nil || b.w != w {
		b.bitmap, b.w = ui.NewBitmap(rasterizeBrand(art, w)), w
	}
	st := stamp()
	v := st.s.NewValues()
	v.Color(st.ink, ink)
	v.Float(st.drawn, min(drawn, 1))
	st.s.Draw(p, at, shader.Options{Values: v, Images: [4]*ui.Bitmap{b.bitmap}})
}

// rasterizeBrand draws the mark's coverage w pixels across, white with
// the coverage as its alpha. A shape filled even-odd is its subpaths each
// rasterized alone and laid over one another exclusively, as the rule
// takes them; the rasterizer's own fill is non-zero.
func rasterizeBrand(art brandArt, w int) *image.NRGBA {
	k := float32(w) / art.w
	h := max(1, int(math.Ceil(float64(art.h*k))))
	cover := image.NewAlpha(image.Rect(0, 0, w, h))
	layer := image.NewAlpha(cover.Rect)
	for _, shape := range art.shapes {
		subpaths := []string{shape.data}
		if shape.evenOdd {
			subpaths = splitSubpaths(shape.data)
		}
		var odd *image.Alpha
		if shape.evenOdd {
			odd = image.NewAlpha(cover.Rect)
		}
		for _, sub := range subpaths {
			r := vector.NewRasterizer(w, h)
			if walkPathData(sub, k, rasterPath{r}) != nil {
				continue
			}
			clear(layer.Pix)
			r.Draw(layer, layer.Rect, image.Opaque, image.Point{})
			if odd != nil {
				for i, a := range layer.Pix {
					x, y := int(odd.Pix[i]), int(a)
					odd.Pix[i] = uint8(x + y - 2*x*y/255)
				}
				continue
			}
			draw.Draw(cover, cover.Rect, layer, image.Point{}, draw.Over)
		}
		if odd != nil {
			draw.Draw(cover, cover.Rect, odd, image.Point{}, draw.Over)
		}
	}
	out := image.NewNRGBA(cover.Rect)
	for i, a := range cover.Pix {
		out.Pix[4*i], out.Pix[4*i+1], out.Pix[4*i+2], out.Pix[4*i+3] = 255, 255, 255, a
	}
	return out
}

// splitSubpaths cuts path data at each move, absolute ones alone, which
// is all the mark's data has.
func splitSubpaths(d string) []string {
	var out []string
	for at := 0; at < len(d); {
		next := strings.IndexByte(d[at+1:], 'M')
		if next < 0 {
			out = append(out, d[at:])
			break
		}
		out = append(out, d[at:at+1+next])
		at += 1 + next
	}
	return out
}

// rasterPath is a vector.Rasterizer as a pathSink.
type rasterPath struct{ r *vector.Rasterizer }

func (v rasterPath) MoveTo(x, y float32)             { v.r.MoveTo(x, y) }
func (v rasterPath) LineTo(x, y float32)             { v.r.LineTo(x, y) }
func (v rasterPath) CubeTo(a, b, c, d, x, y float32) { v.r.CubeTo(a, b, c, d, x, y) }
func (v rasterPath) Close()                          { v.r.ClosePath() }

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
func ParsePathData(d string) (*ui.Path, error) {
	var path ui.Path
	err := walkPathData(d, 1, uiPath{&path})
	return &path, err
}

// pathSink takes the segments of path data as walkPathData reads them.
type pathSink interface {
	MoveTo(x, y float32)
	LineTo(x, y float32)
	CubeTo(c1x, c1y, c2x, c2y, x, y float32)
	Close()
}

// uiPath is a ui.Path as a pathSink.
type uiPath struct{ p *ui.Path }

func (u uiPath) MoveTo(x, y float32)             { u.p.MoveTo(x, y) }
func (u uiPath) LineTo(x, y float32)             { u.p.LineTo(x, y) }
func (u uiPath) CubeTo(a, b, c, d, x, y float32) { u.p.CubeTo(a, b, c, d, x, y) }
func (u uiPath) Close()                          { u.p.Close() }

// walkPathData reads path data d into path, every coordinate scaled by k.
func walkPathData(d string, k float32, path pathSink) error {
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
					return err
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
				return fmt.Errorf("path: number %q", d[i:j])
			}
			nums = append(nums, float32(v)*k)
			i = j
		}
	}
	if cmd != 0 {
		return flush()
	}
	return nil
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
