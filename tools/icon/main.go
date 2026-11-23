// Brandicon draws the app's icon as a PNG from its Icon Composer document,
// for the platforms that take a PNG (Windows and Linux, and the
// development app on macOS): the document's artwork on its fill, as Icon
// Composer places it, on a tile with macOS's continuous corners inside
// macOS's icon grid. The document is the source; the PNG is built, as Git
// stores no binaries.
//
//	go tool icon assets/profile.icon assets/icon.png
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	// canvas is the side of an icon in Icon Composer's points, which the
	// PNG is drawn at; tile and radius are macOS's icon grid on it: the
	// tile's side and its corners' radius.
	canvas = 1024
	tile   = 824
	radius = 185.4
)

// document is what the drawing reads of an Icon Composer document: its
// fill and its first layer.
type document struct {
	Fill   paint `json:"fill"`
	Groups []struct {
		Layers []struct {
			Fill     paint  `json:"fill"`
			Image    string `json:"image-name"`
			Position struct {
				Scale       float32    `json:"scale"`
				Translation [2]float32 `json:"translation-in-points"`
			} `json:"position"`
		} `json:"layers"`
	} `json:"groups"`
}

type paint struct {
	Gradient string `json:"automatic-gradient"`
	Solid    string `json:"solid"`
}

// color is a paint's color, "srgb:r,g,b,a".
func (p paint) color() (ui.Color, error) {
	s := p.Solid
	if s == "" {
		s = p.Gradient
	}
	parts := strings.Split(strings.TrimPrefix(s, "srgb:"), ",")
	if len(parts) != 4 {
		return ui.Color{}, fmt.Errorf("unreadable color %q", s)
	}
	var c [4]uint8
	for i, part := range parts {
		f, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return ui.Color{}, err
		}
		c[i] = uint8(f*255 + 0.5)
	}
	return ui.Color{R: c[0], G: c[1], B: c[2], A: c[3]}, nil
}

func main() {
	full := flag.Bool("full", false, "fill the whole canvas, square, as the document's own artwork")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: icon [-full] <document.icon> <icon.png>")
		os.Exit(2)
	}
	if err := run(flag.Arg(0), flag.Arg(1), *full); err != nil {
		fmt.Fprintln(os.Stderr, "icon:", err)
		os.Exit(1)
	}
}

// clear is a drawing with what lies outside it transparent: the renderer
// draws on a ground, so it is drawn on black and on white, and each pixel's
// coverage is how little the ground shows through it.
func clear(on func(under ui.Color) *image.RGBA) *image.NRGBA {
	black, white := on(ui.Color{A: 255}), on(ui.Color{R: 255, G: 255, B: 255, A: 255})
	out := image.NewNRGBA(black.Rect)
	for i := 0; i < len(black.Pix); i += 4 {
		a := 255 - (int(white.Pix[i+1]) - int(black.Pix[i+1]))
		a = min(max(a, 0), 255)
		out.Pix[i+3] = uint8(a)
		if a == 0 {
			continue
		}
		for k := range 3 {
			out.Pix[i+k] = uint8(min(int(black.Pix[i+k])*255/a, 255))
		}
	}
	return out
}

func run(src, dst string, full bool) error {
	data, err := os.ReadFile(filepath.Join(src, "icon.json"))
	if err != nil {
		return err
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	if len(doc.Groups) == 0 || len(doc.Groups[0].Layers) == 0 {
		return fmt.Errorf("%s has no layer", src)
	}
	layer := doc.Groups[0].Layers[0]
	ground, err := doc.Fill.color()
	if err != nil {
		return err
	}
	ink, err := layer.Fill.color()
	if err != nil {
		return err
	}
	art, err := os.ReadFile(filepath.Join(src, "Assets", layer.Image))
	if err != nil {
		return err
	}
	svg, err := ui.ParseSVG(art)
	if err != nil {
		return err
	}
	w, h := svg.Size()
	scale := layer.Position.Scale
	if scale == 0 {
		scale = 1
	}
	// The artwork stands centered on the canvas, scaled, moved by the
	// layer's translation, y down; the tile shows the canvas at its grid.
	side, at, r := float32(canvas), float32(0), float32(0)
	if !full {
		side, at, r = tile, (canvas-tile)/2, radius
	}
	k := side / canvas
	aw, ah := w*scale*k, h*scale*k
	box := ui.Rect{
		X: at + side/2 - aw/2 + layer.Position.Translation[0]*k,
		Y: at + side/2 - ah/2 + layer.Position.Translation[1]*k,
		W: aw, H: ah,
	}
	draw := func(p *ui.Painter, _ ui.Rect) {
		p.Fill(ui.Rect{X: at, Y: at, W: side, H: side}, ground, r)
		p.Icon(svg, box, ink)
	}
	img := clear(func(under ui.Color) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Background(under).Draw(draw)
		}, canvas, canvas, 1)
	})
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
