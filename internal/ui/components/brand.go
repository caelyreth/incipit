package components

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/caelyreth/incipit/assets"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The brand mark is the profile of the application icon, drawn from its
// own vector (assets.Profile) rather than a picture of it, so it is crisp
// at any size and takes any ink. Its shapes are filled once into a kept
// canvas at about the pixels it is shown at, rather than as paths each
// frame, which would cost their curves every frame; each frame draws one
// quad of it, inked by the stamp shader.

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

// BrandMark is the mark as a view keeps it: a canvas its shapes were
// filled into, at a pixel width stepped up from the one shown (brandStep),
// so the stamp scales it a little down between steps and a window being
// resized fills it again only as it crosses a step.
type BrandMark struct {
	canvas *ui.Canvas
	w      int
}

// brandStep is the pixel widths the mark's canvas is filled at: multiples
// of it.
const brandStep = 64

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
	w := brandStep * max(1, int(math.Ceil(float64(at.W*p.Scale())/brandStep)))
	if b.canvas == nil || b.w != w {
		b.fill(p, art, w)
	}
	st := stamp()
	v := st.s.NewValues()
	v.Color(st.ink, ink)
	v.Float(st.drawn, min(drawn, 1))
	st.s.Draw(p, at, shader.Options{Values: v, Images: [4]*ui.Bitmap{b.canvas.Bitmap()}})
}

// fill fills the mark's shapes, in white, into a canvas w pixels across,
// read at its pixels from its corner, with no transform, so the masks lie
// on its pixel grid; the stamp inks it.
func (b *BrandMark) fill(p *ui.Painter, art brandArt, w int) {
	k := float32(w) / art.w
	h := max(1, int(math.Ceil(float64(art.h*k))))
	b.canvas, b.w = ui.NewCanvas(w, h), w
	b.canvas.Draw(p, func(cp *ui.Painter) {
		for _, shape := range art.shapes {
			var path ui.Path
			if walkPathData(shape.data, k, uiPath{&path}) != nil {
				continue
			}
			if shape.evenOdd {
				path.FillRule(ui.EvenOdd)
			}
			cp.FillPath(&path, ui.RGB(255, 255, 255))
		}
	})
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
