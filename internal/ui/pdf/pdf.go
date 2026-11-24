// Package pdf writes what a view painted (ui.Drawing) as a PDF: its fills,
// paths and pictures as vectors and images, its text as glyphs that keep
// what they spell, so a reader selects, searches and copies it. It is its
// own writer, not the window's renderer: the renderer rasterizes glyphs
// and paths into an atlas, and a printed page needs them as curves.
//
// The drawing is one long column; Write cuts it into pages between lines,
// never through a line of text or a picture that fits on a page.
//
// Glyphs are drawn from their outlines as Type 3 fonts, one per font and
// size, up to 255 glyphs each: the outline is the glyph as the page shows
// it, at the weight and style the text system chose, variable fonts' too,
// with no font file to embed or subset. Each font maps its glyphs to the
// text they spell (ToUnicode).
package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Page is the paper and its margins, in points.
type Page struct {
	Width, Height, Margin float32
}

// A4 is the paper of most of the world, with margins of 2 cm.
var A4 = Page{Width: 595.28, Height: 841.89, Margin: 56.69}

// Letter is the paper of North America, with margins of 0.75 in.
var Letter = Page{Width: 612, Height: 792, Margin: 54}

// Options are what a document is written with.
type Options struct {
	Page  Page
	Title string
}

// Write writes d as a PDF of pages o.Page, its width fitted between the
// margins.
func Write(w io.Writer, d *ui.Drawing, o Options) error {
	if o.Page.Width <= 0 {
		o.Page = A4
	}
	if d == nil || d.Width <= 0 {
		return fmt.Errorf("pdf: nothing drawn")
	}
	scale := (o.Page.Width - 2*o.Page.Margin) / d.Width
	tall := (o.Page.Height - 2*o.Page.Margin) / scale
	doc := newDocument(o.Title)
	doc.drawing = d
	for _, cut := range pagesOf(d, tall) {
		doc.page(d, cut, scale, o.Page)
	}
	return doc.write(w)
}

// span is a stretch of the drawing's height, in DIPs.
type span struct{ top, bottom float32 }

// pagesOf cuts the drawing into stretches at most tall DIPs high, each
// ending between lines: the lowest place in its last half that no line
// of text, picture or small mark stands across, else where it is full.
func pagesOf(d *ui.Drawing, tall float32) []span {
	var keep []span
	end := float32(0)
	for _, it := range d.Items {
		b, ok := bounds(it)
		if !ok {
			continue
		}
		end = max(end, b.Y+b.H)
		switch it.Kind {
		case ui.DrawGlyphs, ui.DrawImage, ui.DrawPath:
		case ui.DrawFill:
			if b.H > 24 {
				continue // a panel or a bar, which may run on
			}
		default:
			continue
		}
		if b.H < tall {
			keep = append(keep, span{b.Y, b.Y + b.H})
		}
	}
	sort.Slice(keep, func(i, j int) bool { return keep[i].top < keep[j].top })
	crosses := func(y float32) bool {
		for _, k := range keep {
			if k.top >= y {
				return false
			}
			if k.bottom > y {
				return true
			}
		}
		return false
	}
	var out []span
	for top := float32(0); top < end-0.5; {
		bottom := top + tall
		if bottom < end {
			for y := bottom; y > top+tall/2; y-- {
				if !crosses(y) {
					bottom = y
					break
				}
			}
		}
		out = append(out, span{top, min(bottom, end)})
		top = bottom
	}
	if len(out) == 0 {
		out = append(out, span{0, tall})
	}
	return out
}

// bounds is where an item draws in the drawing.
func bounds(it ui.DrawItem) (ui.Rect, bool) {
	var r ui.Rect
	switch it.Kind {
	case ui.DrawFill, ui.DrawImage:
		r = it.Rect
	case ui.DrawPath:
		minX, minY, maxX, maxY := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
		it.Path.Each(func(_ ui.PathOp, pts [][2]float32) {
			for _, p := range pts {
				minX, minY, maxX, maxY = min(minX, p[0]), min(minY, p[1]), max(maxX, p[0]), max(maxY, p[1])
			}
		})
		if minX > maxX {
			return r, false
		}
		r = ui.Rect{X: minX - it.Width, Y: minY - it.Width, W: maxX - minX + 2*it.Width, H: maxY - minY + 2*it.Width}
	case ui.DrawGlyphs:
		top, bottom := float32(math.Inf(1)), float32(math.Inf(-1))
		left, right := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, g := range it.Glyphs {
			size := g.Font.Size()
			// The ink of a line: an em's ascenders and descenders.
			top, bottom = min(top, g.Y-size*0.8), max(bottom, g.Y+size*0.22)
			left, right = min(left, g.X), max(right, g.X+size)
		}
		r = ui.Rect{X: left, Y: top, W: right - left, H: bottom - top}
	default:
		return r, false
	}
	if it.Transform.Set {
		t := it.Transform
		xs := [4]float32{r.X, r.X + r.W, r.X, r.X + r.W}
		ys := [4]float32{r.Y, r.Y, r.Y + r.H, r.Y + r.H}
		minX, minY, maxX, maxY := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
		for i := range 4 {
			x, y := t.A*xs[i]+t.C*ys[i]+t.X, t.B*xs[i]+t.D*ys[i]+t.Y
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
		r = ui.Rect{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}
	}
	return r, true
}

// document is a PDF being made: its objects, numbered from 1, and what
// its pages share.
type document struct {
	objects [][]byte
	pages   []int
	title   string
	fonts   map[ui.GlyphFont]*fontSet
	order   []*fontSet
	images  map[uint64]int
	alphas  map[uint8]string
	// drawing is the drawing being written, which keeps its glyphs'
	// outlines.
	drawing *ui.Drawing
}

func newDocument(title string) *document {
	d := &document{title: title, fonts: map[ui.GlyphFont]*fontSet{}, images: map[uint64]int{}, alphas: map[uint8]string{}}
	d.reserve() // 1: the catalog
	d.reserve() // 2: the page tree
	d.reserve() // 3: the opacities, every page's ExtGState
	return d
}

func (d *document) reserve() int {
	d.objects = append(d.objects, nil)
	return len(d.objects)
}

func (d *document) set(n int, body string) { d.objects[n-1] = []byte(body) }

func (d *document) add(body string) int {
	n := d.reserve()
	d.set(n, body)
	return n
}

// stream adds a compressed stream with the dictionary entries dict.
func (d *document) stream(dict string, data []byte) int {
	var z bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&z, zlib.BestCompression)
	zw.Write(data)
	zw.Close()
	return d.rawStream(dict+" /Filter /FlateDecode", z.Bytes())
}

func (d *document) rawStream(dict string, data []byte) int {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<< %s /Length %d >>\nstream\n", dict, len(data))
	b.Write(data)
	b.WriteString("\nendstream")
	n := d.reserve()
	d.objects[n-1] = b.Bytes()
	return n
}

// fontSet is a font's glyphs, split into Type 3 fonts of 255 each.
type fontSet struct {
	font  ui.GlyphFont
	codes map[uint32]glyphCode
	subs  []*subFont
}

type subFont struct {
	name string
	// obj is the font's object, written once the pages are.
	obj    int
	glyphs []uint32 // by code - 1
	texts  []string
	widths []float32
}

type glyphCode struct {
	sub  int
	code byte
}

func (d *document) glyph(g ui.DrawGlyph) (string, byte) {
	fs := d.fonts[g.Font]
	if fs == nil {
		fs = &fontSet{font: g.Font, codes: map[uint32]glyphCode{}}
		d.fonts[g.Font] = fs
		d.order = append(d.order, fs)
	}
	gc, ok := fs.codes[g.ID]
	if !ok {
		if len(fs.subs) == 0 || len(fs.subs[len(fs.subs)-1].glyphs) == 255 {
			fs.subs = append(fs.subs, &subFont{name: fmt.Sprintf("T%d", d.fontCount()+1), obj: d.reserve()})
		}
		sub := fs.subs[len(fs.subs)-1]
		sub.glyphs = append(sub.glyphs, g.ID)
		sub.texts = append(sub.texts, g.Text)
		sub.widths = append(sub.widths, g.Advance)
		gc = glyphCode{sub: len(fs.subs) - 1, code: byte(len(sub.glyphs))}
		fs.codes[g.ID] = gc
	} else if sub := fs.subs[gc.sub]; sub.texts[gc.code-1] == "" && g.Text != "" {
		sub.texts[gc.code-1] = g.Text
	}
	return fs.subs[gc.sub].name, gc.code
}

func (d *document) fontCount() int {
	n := 0
	for _, fs := range d.order {
		n += len(fs.subs)
	}
	return n
}

// alpha is the graphics state of an opacity, by its name.
func (d *document) alpha(a uint8) string {
	if name, ok := d.alphas[a]; ok {
		return name
	}
	name := fmt.Sprintf("A%d", len(d.alphas)+1)
	d.alphas[a] = name
	return name
}

// content is a page's drawing as PDF operators.
type content struct {
	b       strings.Builder
	fill    [3]uint8
	hasFill bool
	alpha   uint8
	stack   []gstate
	images  map[string]int
}

type gstate struct {
	fill    [3]uint8
	hasFill bool
	alpha   uint8
}

func (c *content) f(format string, args ...any) { fmt.Fprintf(&c.b, format, args...) }

func (c *content) save() {
	c.stack = append(c.stack, gstate{c.fill, c.hasFill, c.alpha})
	c.b.WriteString("q\n")
}

func (c *content) restore() {
	if len(c.stack) == 0 {
		return
	}
	s := c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	c.fill, c.hasFill, c.alpha = s.fill, s.hasFill, s.alpha
	c.b.WriteString("Q\n")
}

// color sets the fill and stroke color and the opacity of what follows.
func (c *content) color(d *document, col ui.Color) {
	rgb := [3]uint8{col.R, col.G, col.B}
	if !c.hasFill || c.fill != rgb {
		r, g, b := float32(col.R)/255, float32(col.G)/255, float32(col.B)/255
		c.f("%s %s %s rg %s %s %s RG\n", num(r), num(g), num(b), num(r), num(g), num(b))
		c.fill, c.hasFill = rgb, true
	}
	if c.alpha != col.A {
		c.f("/%s gs\n", d.alpha(col.A))
		c.alpha = col.A
	}
}

func num(v float32) string {
	s := fmt.Sprintf("%.3f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

// page adds the page showing the stretch cut of the drawing.
func (d *document) page(dr *ui.Drawing, cut span, scale float32, paper Page) {
	c := &content{alpha: 255, images: map[string]int{}}
	// Points from the paper's bottom left, y up, to DIPs from the cut's
	// top, y down.
	c.f("%s 0 0 %s %s %s cm\n", num(scale), num(-scale), num(paper.Margin), num(paper.Height-paper.Margin+scale*cut.top))
	c.f("0 %s %s %s re W n\n", num(cut.top), num(dr.Width), num(cut.bottom-cut.top))
	fonts := map[string]bool{}
	for _, it := range dr.Items {
		switch it.Kind {
		case ui.DrawPushClip:
			c.save()
			c.transform(it.Transform)
			roundRect(c, it.Rect, it.Radii)
			c.b.WriteString("W n\n")
			continue
		case ui.DrawPopClip:
			c.restore()
			continue
		}
		b, ok := bounds(it)
		if !ok || b.Y > cut.bottom || b.Y+b.H < cut.top {
			continue
		}
		if it.Transform.Set {
			c.save()
			c.transform(it.Transform)
		}
		switch it.Kind {
		case ui.DrawFill:
			d.drawFill(c, it)
		case ui.DrawPath:
			d.drawPath(c, it)
		case ui.DrawImage:
			d.drawImage(c, it, scale)
		case ui.DrawGlyphs:
			d.drawGlyphs(c, it, fonts)
		}
		if it.Transform.Set {
			c.restore()
		}
	}
	for len(c.stack) > 0 {
		c.restore()
	}
	contents := d.stream("", []byte(c.b.String()))
	var res strings.Builder
	res.WriteString("<< /ProcSet [/PDF /Text /ImageC /ImageB]")
	if len(fonts) > 0 {
		res.WriteString(" /Font <<")
		for _, name := range sortedKeys(fonts) {
			fmt.Fprintf(&res, " /%s %s", name, ref(d.fontObj(name)))
		}
		res.WriteString(" >>")
	}
	if len(c.images) > 0 {
		res.WriteString(" /XObject <<")
		for _, name := range sortedKeys(c.images) {
			fmt.Fprintf(&res, " /%s %s", name, ref(c.images[name]))
		}
		res.WriteString(" >>")
	}
	res.WriteString(" /ExtGState 3 0 R")
	res.WriteString(" >>")
	n := d.add(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Contents %s /Resources %s >>",
		num(paper.Width), num(paper.Height), ref(contents), res.String()))
	d.pages = append(d.pages, n)
}

// fontObj is the object of the font a page names.
func (d *document) fontObj(name string) int {
	for _, fs := range d.order {
		for _, sub := range fs.subs {
			if sub.name == name {
				return sub.obj
			}
		}
	}
	return 0
}

func ref(n int) string { return fmt.Sprintf("%d 0 R", n) }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (c *content) transform(t ui.DrawTransform) {
	if t.Set {
		c.f("%s %s %s %s %s %s cm\n", num(t.A), num(t.B), num(t.C), num(t.D), num(t.X), num(t.Y))
	}
}

// roundRect adds r, its corners rounded by radii, as a path.
func roundRect(c *content, r ui.Rect, radii [4]float32) {
	for i := range radii {
		radii[i] = min(abs(radii[i]), r.W/2, r.H/2)
	}
	if radii == [4]float32{} {
		c.f("%s %s %s %s re\n", num(r.X), num(r.Y), num(r.W), num(r.H))
		return
	}
	const k = 0.5523
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	tl, tr, br, bl := radii[0], radii[1], radii[2], radii[3]
	c.f("%s %s m\n", num(x0+tl), num(y0))
	c.f("%s %s l\n", num(x1-tr), num(y0))
	c.f("%s %s %s %s %s %s c\n", num(x1-tr+tr*k), num(y0), num(x1), num(y0+tr-tr*k), num(x1), num(y0+tr))
	c.f("%s %s l\n", num(x1), num(y1-br))
	c.f("%s %s %s %s %s %s c\n", num(x1), num(y1-br+br*k), num(x1-br+br*k), num(y1), num(x1-br), num(y1))
	c.f("%s %s l\n", num(x0+bl), num(y1))
	c.f("%s %s %s %s %s %s c\n", num(x0+bl-bl*k), num(y1), num(x0), num(y1-bl+bl*k), num(x0), num(y1-bl))
	c.f("%s %s l\n", num(x0), num(y0+tl))
	c.f("%s %s %s %s %s %s c h\n", num(x0), num(y0+tl-tl*k), num(x0+tl-tl*k), num(y0), num(x0+tl), num(y0))
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (d *document) drawFill(c *content, it ui.DrawItem) {
	if it.Color.A > 0 {
		c.color(d, it.Color)
		roundRect(c, it.Rect, it.Radii)
		c.b.WriteString("f\n")
	}
	w := it.Border
	if it.BorderColor.A == 0 || w == [4]float32{} {
		return
	}
	c.color(d, it.BorderColor)
	roundRect(c, it.Rect, it.Radii)
	inner := ui.Rect{X: it.Rect.X + w[3], Y: it.Rect.Y + w[0], W: it.Rect.W - w[1] - w[3], H: it.Rect.H - w[0] - w[2]}
	var radii [4]float32
	for i, side := range [4][2]int{{0, 3}, {0, 1}, {2, 1}, {2, 3}} {
		radii[i] = max(abs(it.Radii[i])-max(w[side[0]], w[side[1]]), 0)
	}
	if inner.W > 0 && inner.H > 0 {
		roundRect(c, inner, radii)
	}
	c.b.WriteString("f*\n")
}

// writePath adds p as a path; flip turns it upside down, as a y down
// outline drawn in a y up glyph space.
func writePath(c *content, p *ui.Path, flip bool) {
	var at, from [2]float32
	p.Each(func(op ui.PathOp, pts [][2]float32) {
		if flip {
			flipped := make([][2]float32, len(pts))
			for i, q := range pts {
				flipped[i] = [2]float32{q[0], -q[1]}
			}
			pts = flipped
		}
		switch op {
		case ui.PathMoveTo:
			c.f("%s %s m\n", num(pts[0][0]), num(pts[0][1]))
			from = pts[0]
		case ui.PathLineTo:
			c.f("%s %s l\n", num(pts[0][0]), num(pts[0][1]))
		case ui.PathQuadTo:
			// The quadratic as the cubic it is: its controls two thirds of
			// the way from each end to the quadratic's.
			q, e := pts[0], pts[1]
			c1 := [2]float32{at[0] + 2*(q[0]-at[0])/3, at[1] + 2*(q[1]-at[1])/3}
			c2 := [2]float32{e[0] + 2*(q[0]-e[0])/3, e[1] + 2*(q[1]-e[1])/3}
			c.f("%s %s %s %s %s %s c\n", num(c1[0]), num(c1[1]), num(c2[0]), num(c2[1]), num(e[0]), num(e[1]))
		case ui.PathCubeTo:
			c.f("%s %s %s %s %s %s c\n", num(pts[0][0]), num(pts[0][1]), num(pts[1][0]), num(pts[1][1]), num(pts[2][0]), num(pts[2][1]))
		case ui.PathClose:
			c.b.WriteString("h\n")
			at = from
			return
		}
		at = pts[len(pts)-1]
	})
}

func (d *document) drawPath(c *content, it ui.DrawItem) {
	if it.Color.A == 0 {
		return
	}
	c.color(d, it.Color)
	writePath(c, it.Path, false)
	switch {
	case it.Width > 0:
		c.f("%s w 1 J 1 j S\n", num(it.Width))
	case it.Path.EvenOdd():
		c.b.WriteString("f*\n")
	default:
		c.b.WriteString("f\n")
	}
}

func (d *document) drawGlyphs(c *content, it ui.DrawItem, fonts map[string]bool) {
	c.b.WriteString("BT\n")
	current := ""
	for _, g := range it.Glyphs {
		name, code := d.glyph(g)
		if name != current {
			c.f("/%s 1 Tf\n", name)
			current = name
			fonts[name] = true
		}
		c.color(d, g.Color)
		// The text space upright, y up as text is, its origin on the
		// glyph's baseline.
		c.f("1 0 0 -1 %s %s Tm <%02x> Tj\n", num(g.X), num(g.Y), code)
	}
	c.b.WriteString("ET\n")
}

// drawImage draws a picture, its pixels kept no finer than print needs:
// 300 dots an inch at the size it stands.
func (d *document) drawImage(c *content, it ui.DrawItem, scale float32) {
	img := it.Image
	src := it.Src
	if src.W == 0 || src.H == 0 {
		src = ui.Rect{W: float32(img.W), H: float32(img.H)}
	}
	n, ok := d.images[img.ID]
	if !ok {
		n = d.addImage(img, max(it.Rect.W, it.Rect.H)*scale*300/72)
		d.images[img.ID] = n
	}
	name := fmt.Sprintf("I%d", n)
	c.images[name] = n
	c.save()
	if it.Color.A < 255 {
		c.f("/%s gs\n", d.alpha(it.Color.A))
	}
	r := it.Rect
	// The image's unit square onto r, its first row on top; Src as the
	// part of it r shows.
	c.f("%s %s %s %s re W n\n", num(r.X), num(r.Y), num(r.W), num(r.H))
	sx, sy := r.W/src.W, r.H/src.H
	fw, fh := float32(img.W)*sx, float32(img.H)*sy
	c.f("%s 0 0 %s %s %s cm /%s Do\n", num(fw), num(-fh), num(r.X-src.X*sx), num(r.Y-src.Y*sy+fh), name)
	c.restore()
}

func (d *document) addImage(img *ui.DrawImageData, side float32) int {
	w, h, pix := img.W, img.H, img.Pix
	if long := max(w, h); side > 0 && float32(long) > side*1.25 {
		w, h, pix = shrink(img, side/float32(long))
	}
	rgb := make([]byte, 0, w*h*3)
	alpha := make([]byte, 0, w*h)
	opaque := true
	for i := 0; i < w*h; i++ {
		r, g, b, a := pix[4*i], pix[4*i+1], pix[4*i+2], pix[4*i+3]
		if a < 255 {
			opaque = false
			if a > 0 {
				r, g, b = byte(int(r)*255/int(a)), byte(int(g)*255/int(a)), byte(int(b)*255/int(a))
			}
		}
		rgb = append(rgb, r, g, b)
		alpha = append(alpha, a)
	}
	dict := fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8", w, h)
	if opaque {
		// A photograph as a JPEG, at a quality print does not tell apart.
		rgba := &image.NRGBA{Pix: make([]byte, 4*w*h), Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
		for i := 0; i < w*h; i++ {
			copy(rgba.Pix[4*i:], rgb[3*i:3*i+3])
			rgba.Pix[4*i+3] = 255
		}
		var j bytes.Buffer
		if err := jpeg.Encode(&j, rgba, &jpeg.Options{Quality: 90}); err == nil {
			return d.rawStream(dict+" /Filter /DCTDecode", j.Bytes())
		}
	}
	if !opaque {
		mask := d.stream(fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8", w, h), alpha)
		dict += " /SMask " + ref(mask)
	}
	return d.stream(dict, rgb)
}

// shrink scales premultiplied pixels down by f, averaging the pixels
// each new one covers.
func shrink(img *ui.DrawImageData, f float32) (int, int, []byte) {
	w, h := max(int(float32(img.W)*f), 1), max(int(float32(img.H)*f), 1)
	out := make([]byte, 4*w*h)
	for y := range h {
		y0, y1 := y*img.H/h, max((y+1)*img.H/h, y*img.H/h+1)
		for x := range w {
			x0, x1 := x*img.W/w, max((x+1)*img.W/w, x*img.W/w+1)
			var sum [4]int
			for sy := y0; sy < y1; sy++ {
				row := img.Pix[4*sy*img.W:]
				for sx := x0; sx < x1; sx++ {
					for k := range 4 {
						sum[k] += int(row[4*sx+k])
					}
				}
			}
			n := (y1 - y0) * (x1 - x0)
			for k := range 4 {
				out[4*(y*w+x)+k] = byte(sum[k] / n)
			}
		}
	}
	return w, h, out
}

// write writes the document: its fonts and opacities, then every object
// and the table that finds them.
func (d *document) write(w io.Writer) error {
	for _, fs := range d.order {
		for _, sub := range fs.subs {
			d.setFont(fs.font, sub)
		}
	}
	var alphas strings.Builder
	alphas.WriteString("<<")
	for _, a := range sortedAlphas(d.alphas) {
		fmt.Fprintf(&alphas, " /%s << /Type /ExtGState /ca %s /CA %s >>", d.alphas[a], num(float32(a)/255), num(float32(a)/255))
	}
	alphas.WriteString(" >>")
	d.set(3, alphas.String())
	kids := make([]string, len(d.pages))
	for i, p := range d.pages {
		kids[i] = ref(p)
	}
	d.set(1, "<< /Type /Catalog /Pages 2 0 R >>")
	d.set(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(d.pages)))
	info := d.add(fmt.Sprintf("<< /Title %s /Producer (Neverland) >>", textString(d.title)))

	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(d.objects))
	for i, body := range d.objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(body)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(d.objects)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info %s >>\nstartxref\n%d\n%%%%EOF\n", len(d.objects)+1, ref(info), xref)
	_, err := w.Write(out.Bytes())
	return err
}

func sortedAlphas(m map[uint8]string) []uint8 {
	keys := make([]uint8, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// setFont writes the Type 3 font of sub's glyphs, drawn from their
// outlines, and the text each spells.
func (d *document) setFont(font ui.GlyphFont, sub *subFont) {
	var procs strings.Builder
	var diffs strings.Builder
	minX, minY, maxX, maxY := float32(0), float32(0), float32(0), float32(0)
	for i, id := range sub.glyphs {
		c := &content{}
		path := d.drawing.Outline(font, id)
		gx0, gy0, gx1, gy1 := float32(0), float32(0), float32(0), float32(0)
		if path != nil {
			first := true
			path.Each(func(_ ui.PathOp, pts [][2]float32) {
				for _, p := range pts {
					if first {
						gx0, gy0, gx1, gy1, first = p[0], -p[1], p[0], -p[1], false
					}
					gx0, gy0, gx1, gy1 = min(gx0, p[0]), min(gy0, -p[1]), max(gx1, p[0]), max(gy1, -p[1])
				}
			})
		}
		c.f("%s 0 %s %s %s %s d1\n", num(sub.widths[i]), num(gx0), num(gy0), num(gx1), num(gy1))
		if path != nil {
			writePath(c, path, true)
			c.b.WriteString("f\n")
		}
		n := d.stream("", []byte(c.b.String()))
		fmt.Fprintf(&procs, " /g%d %s", i+1, ref(n))
		fmt.Fprintf(&diffs, " /g%d", i+1)
		minX, minY, maxX, maxY = min(minX, gx0), min(minY, gy0), max(maxX, gx1), max(maxY, gy1)
	}
	ws := make([]string, len(sub.widths))
	for i, w := range sub.widths {
		ws[i] = num(w)
	}
	widths := strings.Join(ws, " ")
	cmap := d.stream("", []byte(toUnicode(sub)))
	d.set(sub.obj, fmt.Sprintf("<< /Type /Font /Subtype /Type3 /FontBBox [%s %s %s %s] /FontMatrix [1 0 0 1 0 0] /CharProcs <<%s >> /Encoding << /Type /Encoding /Differences [1%s] >> /FirstChar 1 /LastChar %d /Widths [%s] /ToUnicode %s /Resources << >> >>",
		num(minX), num(minY), num(maxX), num(maxY), procs.String(), diffs.String(), len(sub.glyphs), widths, ref(cmap)))
}

// toUnicode is the CMap mapping each of sub's codes to the text its glyph
// spells; a glyph that spells nothing of its own, as the second of a
// cluster, maps to nothing.
func toUnicode(sub *subFont) string {
	var b strings.Builder
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /Neverland-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<01> <ff>\nendcodespacerange\n")
	var lines []string
	for i, t := range sub.texts {
		if t == "" {
			continue
		}
		var hex strings.Builder
		for _, u := range utf16Of(t) {
			fmt.Fprintf(&hex, "%04X", u)
		}
		lines = append(lines, fmt.Sprintf("<%02X> <%s>", i+1, hex.String()))
	}
	for len(lines) > 0 {
		n := min(len(lines), 100)
		fmt.Fprintf(&b, "%d beginbfchar\n%s\nendbfchar\n", n, strings.Join(lines[:n], "\n"))
		lines = lines[n:]
	}
	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return b.String()
}

func utf16Of(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
			continue
		}
		out = append(out, uint16(r))
	}
	return out
}

// textString is s as a PDF text string: UTF-16 with its byte order mark.
func textString(s string) string {
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, u := range utf16Of(s) {
		fmt.Fprintf(&b, "%04X", u)
	}
	b.WriteString(">")
	return b.String()
}
