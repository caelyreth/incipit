package components

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/caelyreth/neverland/assets"
	"github.com/egoist/mygo/ui"
)

// The brand mark is the profile of the application icon, drawn from its
// own vector (assets.Profile) rather than a picture of it, so it is crisp
// at any size, takes any ink, and can draw itself as a pen would.

// brandShape is one filled path of the mark.
type brandShape struct {
	path    *ui.Path
	outline *ui.Path
	evenOdd bool
}

var brandMark = sync.OnceValues(func() (brandArt, error) { return parseBrand(assets.Profile) })

type brandArt struct {
	w, h   float32
	shapes []brandShape
}

// BrandAspect is the mark's width over its height.
func BrandAspect() float32 {
	art, err := brandMark()
	if err != nil || art.h == 0 {
		return 1
	}
	return art.w / art.h
}

// DrawBrandMark draws the mark centered in box in ink. drawn, from 0 to 1,
// is how far it has drawn itself: first a pen tracing its outlines, then
// the ink filling them in behind it; 1 is the mark as the icon has it.
func DrawBrandMark(p *ui.Painter, box ui.Rect, ink ui.Color, drawn float32) {
	art, err := brandMark()
	if err != nil || art.w == 0 || box.W <= 0 || box.H <= 0 {
		return
	}
	drawn = max(0, min(drawn, 1))
	s := min(box.W/art.w, box.H/art.h)
	x, y := box.X+(box.W-art.w*s)/2, box.Y+(box.H-art.h*s)/2
	// The pen leads over the first two thirds; the fill comes up through
	// the last half, so the two overlap and the line seems to bleed.
	pen := Smoothstep(drawn / 0.68)
	fill := Smoothstep((drawn - 0.5) / 0.5)
	p.Transform(ui.Scaling(s, s).Then(ui.Translation(x, y)), 0, 0, func() {
		for _, shape := range art.shapes {
			if fill > 0 {
				p.FillPath(shape.path, ink.Alpha(fill*float32(ink.A)/255))
			}
			if pen > 0 && fill < 1 {
				shape.outline.Trim(0, pen)
				p.StrokePath(shape.outline, 1.4/s+0.9, ink.Alpha((1-fill*0.6)*float32(ink.A)/255))
			}
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
		fill, err := ParsePathData(el.D)
		if err != nil {
			return brandArt{}, err
		}
		outline, _ := ParsePathData(el.D)
		outline.Cap(ui.RoundCap).Join(ui.RoundJoin)
		shape := brandShape{path: fill, outline: outline, evenOdd: el.FillRule == "evenodd"}
		if shape.evenOdd {
			fill.FillRule(ui.EvenOdd)
		}
		art.shapes = append(art.shapes, shape)
	}
	return art, nil
}

// ParsePathData reads an SVG path's data, of moves, lines, horizontal and
// vertical lines, cubic curves and closes, absolute or relative.
func ParsePathData(d string) (*ui.Path, error) {
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
			nums = append(nums, float32(v))
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
