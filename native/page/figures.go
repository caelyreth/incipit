package page

import (
	"fmt"
	"math"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// A figure is drawn as its picture, at its span of the column or its own
// size, over its caption. The layout keeps the picture's room from its
// recorded size, so a figure takes its place before its picture is read:
// its place shows first, as quiet as an inset, and the picture fades in
// over it once decoded, with a hairline keeping a picture as light as the
// page from bleeding into it.
//
// The caption is the figure's text, under the picture. An empty one is
// presented only while the figure is selected or the caret is in it, so a
// figure without one takes no room for it; its line opens as the figure is
// selected, the page below gliding down, Escape or a click there writes
// one, and it closes again once neither holds and it is still empty.
//
// Held whole by the selection, a figure wears the ring every object does
// (objects.go), never a wash over its picture; selected alone, or under
// the pointer, it shows the grips it resizes from. A resize is the
// renderer's until release, when it commits as one ResizeFigure.

const (
	// gapBarRoom sets an object's gap carets off what it draws, beyond
	// the outline drawn around it (paints.go).
	gapBarRoom = objectOutset + 3
	// figureMin is the narrowest a figure is drawn.
	figureMin = 48
	// gripWidth and gripInset shape and place a resize grip.
	gripWidth, gripInset = 6, 8
)

// captionHint is what an empty caption shows while the caret is in it.
const captionHint = "Caption"

// captionGap is the room between a picture and its caption.
func captionGap(m Metrics) float32 { return m.Body * 0.5 }

// figureRadius rounds a picture's corners.
func figureRadius(m Metrics) float32 { return m.Body * 0.35 }

// A picture is drawn as large as reads well, never more:
//
//   - at its own size, a pixel to a DIP, at most the column: a small or
//     low-resolution picture is never scaled up past its pixels, so it
//     stays sharp, and keeps a least width only if it has that many;
//   - no taller than figureTall of the column (at least figureTallLines
//     lines of body text), so one picture never fills the window: a tall
//     one narrows to fit, down to figureNarrow of the column;
//   - a picture still taller at that width, a long screenshot or a scroll
//     of a page, shows its top at that height, fading out below to say
//     there is more;
//   - a span the writer set by resizing is kept as set, a long picture
//     cropped as above, so a resize only ever changes the width;
//   - a picture whose size is unknown takes the column at sixteen by nine.

const (
	figureTall      = 0.75
	figureTallLines = 14
	figureNarrow    = 0.35
	// figureFadeRoom is how much of a cropped picture's height fades out.
	figureFadeRoom = 0.18
)

// figureFit is how a picture is drawn: its box, and the share of its
// height the box shows from its top (1 for all of it).
type figureFit struct {
	w, h, shown float32
}

// figureSize fits a figure's picture to a column width wide.
func figureSize(shape profile.Shape, width float32, m Metrics) figureFit {
	pw, ph := float32(shape.Width), float32(shape.Height)
	if pw <= 0 || ph <= 0 {
		pw, ph = width, width*9/16
	}
	tall := max(width*figureTall, m.Body*m.Spacing*figureTallLines)
	least := min(float32(figureMin), pw)
	var w float32
	if shape.Span > 0 {
		w = width * float32(shape.Span) / profile.SpanFull
	} else {
		w = min(pw, width)
		if h := w * ph / pw; h > tall {
			// Narrow a tall picture to fit, but not to a sliver.
			w = max(tall*pw/ph, min(width*figureNarrow, w))
		}
	}
	w = min(max(w, least), width)
	fit := figureFit{w: w, h: w * ph / pw, shown: 1}
	if fit.h > tall {
		fit.shown, fit.h = tall/fit.h, tall
	}
	return fit
}

// layFigure lays a figure out: its picture's box, and its caption under it
// in the picture's alignment.
func (lc *layoutCache) layFigure(u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, span int) {
	shape := profile.ShapeOf(p.Node)
	if span >= 0 {
		shape.Span = span
	}
	u.figure = &shape
	fit := figureSize(shape, width, m)
	w, h := fit.w, fit.h
	u.crop = fit.shown
	x := float32(0)
	align := ui.Start
	switch shape.Align {
	case profile.AlignCenter:
		x, align = (width-w)/2, ui.Center
	case profile.AlignEnd:
		x, align = width-w, ui.End
	}
	u.box = ui.Rect{X: x, W: w, H: h}
	u.h = h
	for i, s := range p.Surfaces {
		sl := lc.laySurface(i, s, p, comp, th, m, ui.TextShape{Width: width, Align: align})
		sl.y = u.h + captionGap(m)
		u.surfaces = append(u.surfaces, sl)
		// A caption with text shows; an empty one only while the figure
		// is held, to write in, and takes no room otherwise.
		sl.shown = sl.open || sl.text != ""
		if sl.shown {
			u.h = sl.y + sl.h
		}
	}
}

// figureResize is a resize from a figure's grip, the page's drag until
// release: the figure, the grip's side, where the press started and the
// span it previews.
type figureResize struct {
	el     *Element
	node   doc.ID
	side   float32 // -1 for the start side's grip, 1 for the end's
	x0, w0 float32
	both   bool // a centered figure grows from both sides
	width  float32
	span   int
}

// spanAt is the span a resize previews with the pointer at x.
func (r *figureResize) spanAt(x float32) int {
	d := (x - r.x0) * r.side
	if r.both {
		d *= 2
	}
	w := min(max(r.w0+d, figureMin), r.width)
	return int(math.Round(float64(w / r.width * profile.SpanFull)))
}

// label names the span a resize previews.
func (r *figureResize) label() string {
	if r.span >= profile.SpanFull {
		return "Full width"
	}
	return fmt.Sprintf("%d%%", (r.span+5)/10)
}

func (r *figureResize) key() controlKey {
	return controlKey{unit: r.node, part: partFigureGrip, at: int(r.side)}
}
func (r *figureResize) cursor() ui.Cursor { return ui.CursorResizeEW }

func (r *figureResize) move(x, _ float32, _ time.Time) { r.span = r.spanAt(x) }

// finish commits the span as one intent, unless canceled.
func (r *figureResize) finish(canceled bool, at time.Time) {
	if !canceled {
		r.el.Editor.Do(profile.ResizeFigure{ID: r.node, Span: r.span}, at)
	}
}

// ask previews the span in the figure's layout.
func (r *figureResize) ask(asks map[doc.ID]objectIn) {
	a := askOf(asks, r.node)
	a.span = r.span
	asks[r.node] = a
}

// partFigure is a figure's picture, partFigureGrip its grip, at its side.
const (
	partFigure     part = "figure"
	partFigureGrip part = "figure-grip"
)

// controls are a figure's grips while it shows them, each resizing it
// from its side.
// figureMenu are the commands a figure offers for itself.
var figureMenu = [][]vocab.CommandID{{"figures.alignStart", "figures.alignCenter"}}

// menu is a figure's commands.
func (figureKind) menu(*Element, *unitLayout) [][]vocab.CommandID { return figureMenu }

func (figureKind) controls(el *Element, u *unitLayout) []control {
	// The picture offers the figure's commands, selecting it as a click on
	// it does.
	id := u.pres.Unit
	out := []control{{key: controlKey{unit: id, part: partFigure}, rect: ui.Rect{X: u.x + u.box.X, Y: u.y + u.box.Y, W: u.box.W, H: u.box.H},
		menu: func(_, _ float32, at time.Time) [][]vocab.CommandID {
			el.Editor.Do(vocab.Place{Node: id}, at)
			return figureKind{}.menu(el, u)
		}}}
	if !el.gripsShown(u) {
		return out
	}
	for _, side := range grips(u) {
		r, ok := gripRect(u, side)
		if !ok {
			continue
		}
		out = append(out, control{
			key:    controlKey{unit: u.pres.Unit, part: partFigureGrip, at: int(side)},
			rect:   grown(r, 4, 4),
			cursor: ui.CursorResizeEW,
			press: func(x, _ float32, _ int, _ bool, _ time.Time) drag {
				d := &figureResize{el: el, node: u.pres.Unit, side: side, x0: x, w0: u.box.W,
					both: u.figure.Align == profile.AlignCenter, width: max(el.Editor.metrics.Width-u.x, 1)}
				d.span = d.spanAt(x)
				return d
			},
		})
	}
	return out
}

// grips are the sides a figure resizes from: both of a centered one, the
// end of one at the start, the start of one at the end.
func grips(u *unitLayout) []float32 {
	switch u.figure.Align {
	case profile.AlignCenter:
		return []float32{-1, 1}
	case profile.AlignEnd:
		return []float32{-1}
	}
	return []float32{1}
}

// gripRect is a grip's rectangle in page coordinates.
func gripRect(u *unitLayout, side float32) (ui.Rect, bool) {
	box := u.box
	h := min(36, box.H*0.4)
	if h < 12 || box.W < 3*gripInset {
		return ui.Rect{}, false
	}
	x := u.x + box.X + gripInset
	if side > 0 {
		x = u.x + box.X + box.W - gripInset - gripWidth
	}
	return ui.Rect{X: x, Y: u.y + box.Y + (box.H-h)/2, W: gripWidth, H: h}, true
}

// gripsShown reports whether a figure shows its grips: selected alone
// while the page has the keys, under the pointer, or being resized.
func (el *Element) gripsShown(u *unitLayout) bool {
	if r, ok := el.drag.(*figureResize); ok {
		return r.node == u.pres.Unit
	}
	sel := el.shownSelection()
	return sel.Kind == doc.SelectNode && sel.Node == u.pres.Unit && el.focused || el.near == u.pres.Unit
}

// reach is a figure's picture and its grips either side.
func (figureKind) reach(_ *Element, u *unitLayout) ui.Rect {
	box := u.box
	box.X += u.x
	box.Y += u.y
	return grown(box, gripWidth, 0)
}

// figureSpring is the key of one of a figure's shares.
func figureSpring(id doc.ID, name string) springKey {
	return springKey{unit: id, name: name, of: shareValue}
}

// step moves a figure's shares on: its picture fading in as it arrives,
// its grips as it shows them, its caption's hint. A picture is asked for
// only as it is drawn; one that arrived before it was drawn again shows at
// once.
func (figureKind) step(el *Element, u *unitLayout, _ *objectIn) {
	shown, grip, hint := float32(0), float32(0), float32(0)
	state := el.Pictures.State(u.figure.Media)
	if state == PictureReady {
		shown = 1
	}
	if el.gripsShown(u) && state != PictureMissing {
		grip = 1
	}
	if el.hinted(u) {
		hint = 1
	}
	id := u.pres.Unit
	el.springs.follow(figureSpring(id, "picture"), shown, layoutSettle)
	el.springs.follow(figureSpring(id, "grips"), grip, chromeFollow)
	el.springs.follow(figureSpring(id, "hint"), hint, chromeFollow)
}

// paintFigure draws a figure's picture, its place while the picture is on
// its way, and its grips; the caption is drawn as text with the rest.
func (el *Element) paintFigure(p *ui.Painter, u *unitLayout, ux, uy float32, now time.Time, animate bool) {
	t := el.Theme
	m := el.Editor.metrics
	box := u.box
	box.X += ux
	box.Y += uy
	radius := figureRadius(m)
	bitmap, state := el.Pictures.Picture(u.figure.Media, now)
	id := u.pres.Unit
	alpha := el.springs.value(figureSpring(id, "picture"))
	gripsShown := el.springs.value(figureSpring(id, "grips"))
	hinted := el.springs.value(figureSpring(id, "hint"))

	if alpha < 1 {
		p.Fill(box, t.Surface.Inset.Alpha(1-alpha), radius)
	}
	if state == PictureMissing {
		size := min(box.W, box.H, m.Body*1.4)
		icon := ui.Rect{X: box.X + (box.W-size)/2, Y: box.Y + (box.H-size)/2, W: size, H: size}
		p.Icon(components.IconImageOff.SVG(), icon, t.Text.Muted)
	} else if bitmap != nil && alpha > 0 {
		var src ui.Rect
		if u.crop < 1 {
			// A long picture shows its top, its region kept in proportion.
			bw, bh := bitmap.Size()
			src = ui.Rect{W: float32(bw), H: float32(bh) * u.crop}
		}
		p.Clip(box, radius, func() {
			p.ImageRegion(bitmap, src, box, ui.ImageOptions{Tint: ui.Color{R: 255, G: 255, B: 255, A: 255}.Alpha(alpha)})
		})
		p.Stroke(box, t.Prose.Subtle.Alpha(alpha), radius, 1)
		if u.crop < 1 {
			// It fades into the page below, saying there is more.
			fade := box.H * figureFadeRoom
			band := ui.Rect{X: box.X - 1, Y: box.Y + box.H - fade, W: box.W + 2, H: fade + 1}
			page := t.Surface.Page
			p.FillGradient(band, ui.LinearGradient{From: page.Alpha(0), To: page.Alpha(alpha), Angle: 180}, 0)
		}
	}
	if a := gripsShown; a > 0.001 {
		for _, side := range grips(u) {
			r, ok := gripRect(u, side)
			if !ok {
				continue
			}
			r.X += ux - u.x
			r.Y += uy - u.y
			// It lights under the pointer as every control does.
			hover := el.lit(controlKey{unit: id, part: partFigureGrip, at: int(side)})
			p.Shadow(r, gripWidth/2, 0, 1, 4, 0, ui.Color{A: 255}.Alpha(0.3*a))
			p.Fill(r, components.Blend(t.Surface.Elevated, t.Accent.Foreground, hover).Alpha(a), gripWidth/2)
			p.Stroke(r, components.Blend(t.Border.Default, t.Accent.Foreground, hover).Alpha(a), gripWidth/2, 1)
		}
	}
	if r, ok := el.drag.(*figureResize); ok && r.node == u.pres.Unit {
		el.paintResizeLabel(p, box, r.label())
	}
	if a := hinted; a > 0.001 && len(u.surfaces) > 0 {
		s := u.surfaces[0]
		span, x, _ := hintOf(u, el.Editor.theme, m)
		span.Color = t.Text.Muted.Alpha(0.65 * a)
		c := s.caretRect(0)
		h := span.Size * 1.2
		p.RichText(ux+s.x+x, uy+s.y+c.Y+(c.H-h)/2, 0, span)
	}
}

// hintOf is an empty caption's hint and where it starts in its surface:
// in the caption's alignment, the caret standing at its start as an empty
// field's stands at its placeholder's.
func hintOf(u *unitLayout, th present.Theme, m Metrics) (ui.Span, float32, float32) {
	span := span(captionHint, th.Resolve([]vocab.Group{u.pres.Group}), m.Body)
	lay := ui.ShapeRichTextWith([]ui.Span{span}, ui.Font{Family: span.Font, Size: span.Size, Weight: span.Weight, Italic: span.Italic}, ui.TextShape{})
	w, _ := lay.Size()
	x := u.surfaces[0].caretRect(0).X
	switch u.figure.Align {
	case profile.AlignCenter:
		x -= w / 2
	case profile.AlignEnd:
		x -= w
	}
	return span, x, w
}

// hinted reports whether a figure's caption shows its hint: it is empty,
// and the caret is in it while the page has the keys.
func (el *Element) hinted(u *unitLayout) bool {
	if len(u.surfaces) == 0 || u.surfaces[0].text != "" || !el.focused {
		return false
	}
	sel := el.shownSelection()
	return sel.Kind == doc.SelectText && sel.IsCollapsed() && sel.Head.Pos.Ref() == u.surfaces[0].slot
}

// paintResizeLabel names the span a resize previews, on a chip at the top
// of the picture.
func (el *Element) paintResizeLabel(p *ui.Painter, box ui.Rect, label string) {
	el.paintChip(p, label, func(chip ui.Rect) (ui.Rect, bool) {
		chip.X, chip.Y = box.X+(box.W-chip.W)/2, box.Y+chip.H*0.75
		return chip, chip.W <= box.W && chip.H*2 <= box.H
	})
}

// paintChip names what a drag sets on a chip, which place puts where it
// goes, or nowhere.
func (el *Element) paintChip(p *ui.Painter, label string, place func(chip ui.Rect) (ui.Rect, bool)) {
	size := el.Editor.metrics.Body * 0.8
	span := ui.Span{Text: label, Size: size, Color: el.Theme.Text.Default}
	w, h := p.MeasureText(0, span)
	padX, padY := size*0.75, size*0.3
	chip, ok := place(ui.Rect{W: w + 2*padX, H: h + 2*padY})
	if !ok {
		return
	}
	p.Fill(chip, el.Theme.Surface.Elevated.Alpha(0.94), chip.H/2)
	p.RichText(chip.X+padX, chip.Y+padY, 0, span)
}

// While pictures are dragged over the page, a line between its blocks
// shows where they would go: before the block under the pointer, or after
// it from its lower half. It fades in where it first shows, glides as the
// pointer moves from block to block, and fades out as the drag leaves or
// lands.

const (
	dropWeight = 2
)

type dropLine struct {
	y       motion.Spring
	shown   motion.Fade
	known   bool
	target  float32
	showing bool
}

// takeDrops makes the page take pictures dragged onto it.
func (el *Element) takeDrops(box ui.Element) {
	if el.Editor.Intake == nil {
		return
	}
	options := transfer.DropOptions{Formats: pictureFormats}
	_, over := ui.DataDragOver(box, options)
	el.drop.showing = false
	if over {
		if x, y, inside := box.PointerPosition(); inside {
			px, py := el.toPage(x, y)
			if at, line, ok := el.dropPlace(px, py); ok {
				el.dropAt, el.drop.target, el.drop.showing = &at, line, true
			}
		}
	}
	if drop, ok := ui.DropData(box, options); ok {
		el.drop.showing = false
		if pictures := picturesOf(drop.Data, drop.Data.Formats()); len(pictures) > 0 {
			el.Editor.Intake(pictures, el.dropAt)
		}
	}
}

// dropPlace is where pictures dropped at a page point go, and the page y
// of the line showing it.
func (el *Element) dropPlace(x, y float32) (doc.Caret, float32, bool) {
	pl := el.Editor.layout
	if pl == nil || len(pl.units) == 0 {
		return doc.Caret{}, 0, false
	}
	u := pl.units[len(pl.units)-1]
	for _, cand := range pl.units {
		if y < cand.bottom() {
			u = cand
			break
		}
	}
	after := y >= u.y+u.h/2
	gap := el.Editor.metrics.Gap / 2
	line := u.y - gap
	if after {
		line = u.bottom() + gap
	}
	switch {
	case u.kind != nil && u.pres.Gap != nil:
		return u.beside(after), line, true
	case len(u.surfaces) > 0 && len(u.surfaces[0].stops) > 0:
		stops := u.surfaces[0].stops
		at := stops[0].caret
		if after {
			at = stops[len(stops)-1].caret
		}
		return doc.CaretAt(at.Pos), line, true
	}
	return doc.Caret{}, 0, false
}

// paintDrop draws the drop line, and reports whether it still moves.
func (el *Element) paintDrop(p *ui.Painter, ox, oy float32, m Metrics, now time.Time, animate bool) bool {
	d := &el.drop
	if !d.known || !animate || d.shown.Gone() && d.showing {
		d.y = motion.NewSpring(d.target, now)
		d.known = true
	}
	if d.showing {
		d.y.Step(d.target, now, carryPart)
	}
	a := d.shown.Step(d.showing, now, animate)
	if a > 0.001 {
		y := oy + d.y.Value() - dropWeight/2
		p.Fill(ui.Rect{X: ox, Y: y, W: m.Width, H: dropWeight}, el.Theme.Accent.Foreground.Alpha(a), dropWeight/2)
	}
	return animate && (d.y.Moving() || d.shown.Moving())
}
