package page

import (
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

// Formulas are drawn as math, set by internal/ui/formula, wherever they are:
//
//   - an inline formula is an object in its line, sitting on the text's
//     baseline; while its source card is open it wears a soft fill, and the
//     math above follows the source live as it is typed;
//   - a math block shows its formula set for display, centered, in a
//     display area of at least two display lines; its source is shown
//     under it, in an inset panel, while the caret or a selection end is in
//     the block, the panel opening and closing as the page below glides
//     with it. While its panel is open the block is framed: display and
//     panel read as one block.
//
// Math shows only a source that sets soundly: no placeholder boxes, no
// error glyphs. While a source half typed does not set, the math shown is
// the last source of that formula that did, dimmed, so its room holds
// still as it is typed. A formula that has never set shows no math: an
// inline one shows its source, and a math block's display area shows its
// hint, its frame and panel open whether held or not, so the block reads
// as math from the keystroke that makes it. Math fades in as it first shows, and dims and brightens
// as it goes stale and sound.

const (
	// mathPad is the room either side of inline math: room for the
	// outline drawn around it (paints.go), inside its own advance, so the
	// carets beside it stand outside that outline.
	mathPad = lineOutline + 1
	// displayScale sets display math over the body text.
	displayScale = 1.15
	// staleInk is how strongly math shows while its source does not set.
	staleInk = 0.45
	// displayLines is the least a math block's display area holds, in
	// display lines, so a block keeps its room as its first math sets.
	displayLines = 2.7
)

// mathSet is a formula as set for drawing: its box, whether it is the last
// sound source standing in for one that is not, and its geometry.
type mathSet struct {
	box             *untie.Box
	stale           bool
	ascent, descent float32
	// from and to are how far across the box its ink reaches.
	from, to float32
}

// lastGood keys a formula's last sound source by its node, dialect and mode.
type lastGood struct {
	node    doc.ID
	dialect profile.MathDialect
	display bool
}

// typeset sets a formula's source in dialect: as written when it sets soundly,
// else the last source of that formula in the same dialect that did, marked
// stale; a zero mathSet when none has.
func (lc *layoutCache) typeset(id doc.ID, src string, dialect profile.MathDialect, display bool, size, width float32) mathSet {
	k := lastGood{id, dialect, display}
	if strings.TrimSpace(src) == "" {
		// Emptied, a formula has no math: it shows its hint.
		delete(lc.good, k)
		return mathSet{}
	}
	if !formulaSound(src, dialect, display) {
		last, ok := lc.good[k]
		if !ok {
			return mathSet{}
		}
		ms := setMath(last, dialect, display, size, width)
		ms.stale = true
		return ms
	}
	if len(lc.good) > 4096 {
		clear(lc.good)
	}
	lc.good[k] = src
	return setMath(src, dialect, display, size, width)
}

func setMath(src string, dialect profile.MathDialect, display bool, size, width float32) mathSet {
	box := setFormula(src, dialect, display, size, width)
	if box == nil {
		return mathSet{}
	}
	ms := mathSet{box: box}
	ms.ascent, ms.descent = formulaExtent(box)
	ms.from, ms.to = formulaInk(box)
	return ms
}

// mathLayout is a math block's display: its formula, and how high its
// display stands over its source panel.
type mathLayout struct {
	mathSet
	dialect profile.MathDialect
	h       float32
	// fold and full are the block's height with its source panel closed
	// and open; open is whether the panel shows: while the block is held
	// (its source surface open), or always while it has no math.
	fold, full float32
	open       bool
	// hint is the block's hint as shaped when first drawn.
	hint *ui.TextLayout
}

// layMath lays out a math block's display over its source panel.
func layMath(u *unitLayout, lc *layoutCache, m Metrics, width float32) {
	if len(u.surfaces) == 0 {
		return
	}
	s := u.surfaces[0]
	dialect := profile.MathDialectOf(u.pres.Node)
	ms := lc.typeset(u.pres.Unit, u.pres.Node.Text(), dialect, true, m.Body*displayScale, width)
	ml := &mathLayout{mathSet: ms, dialect: dialect, open: s.open || ms.box == nil}
	s.shown = ml.open
	ml.h = m.Body * displayScale * displayLines
	if ms.box != nil {
		ml.h = max(ml.h, ms.ascent+ms.descent+2*m.Body*0.45)
	}
	s.x, s.y = cardPad(m), ml.h+cardPad(m)
	// The panel's fill reaches half a card pad past its text, leaving
	// half a pad to the frame on every side.
	ml.full = s.y + s.h + cardPad(m)
	ml.fold = ml.h
	if ms.box == nil {
		ml.fold = ml.full
	}
	u.math = ml
	u.h = ml.fold
	if ml.open {
		u.h = ml.full
	}
	// Its box is its display, which a click selects it on; open, it draws
	// its frame around display and panel, which an outline goes around.
	u.box = ui.Rect{W: width, H: ml.h}
	if ml.open {
		u.drawn, u.frame = ui.Rect{W: width, H: u.h}, cardPad(m)
	}
}

// mathLook is how strongly a formula's parts show this frame: its math's
// ink, its hint, and what its being edited opens (an inline formula's
// underline, a math block's frame and panel).
type mathLook struct {
	ink, open, hint float32
	// drawn is the math to draw at ink: the formula's, or the last it had.
	drawn mathSet
}

func mathSpring(id doc.ID, name string) springKey {
	return springKey{unit: id, name: name, of: shareValue}
}

// stepMath moves a formula's shares on. As a formula first shows, its math
// or hint fades in, and its underline or frame. Its hint leaves at once as
// math arrives, as a field's placeholder does; it would only crowd what
// arrives. Math leaving fades out where it stood, as its hint fades in;
// math dims and brightens as it goes stale and sound.
func (el *Element) stepMath(id doc.ID, ms mathSet, open bool) {
	ink, hint, opened := float32(1), float32(0), float32(0)
	switch {
	case ms.box == nil:
		ink, hint = 0, 1
	case ms.stale:
		ink = staleInk
	}
	if open {
		opened = 1
	}
	if hint == 0 {
		el.springs.jump(mathSpring(id, "hint"), 0)
	}
	el.springs.follow(mathSpring(id, "ink"), ink, layoutSettle)
	el.springs.follow(mathSpring(id, "open"), opened, chromeFollow)
	el.springs.follow(mathSpring(id, "hint"), hint, chromeFollow)
	if ms.box != nil {
		el.springs.keep(mathSpring(id, "ink"), ms)
	}
}

// mathLookOf is how a formula shows, as its shares stand.
func (el *Element) mathLookOf(id doc.ID) mathLook {
	lk := mathLook{ink: el.springs.value(mathSpring(id, "ink")), open: el.springs.value(mathSpring(id, "open")),
		hint: el.springs.value(mathSpring(id, "hint"))}
	lk.drawn, _ = el.springs.kept(mathSpring(id, "ink")).(mathSet)
	return lk
}

// inks are the colors math is drawn in, at a strength.
func (el *Element) inks(a float32) formulaInks {
	t := el.Theme
	return formulaInks{Text: t.Prose.Text.Alpha(a), Error: t.Status.Danger.Foreground.Alpha(a), Placeholder: t.Text.Muted.Alpha(a)}
}

// hintInk is a hint's color at a strength: muted, or, once the formula is
// no longer edited, the danger color if its source has never set.
func (el *Element) hintInk(src string, editing bool, a float32) ui.Color {
	if !editing && strings.TrimSpace(src) != "" {
		return el.Theme.Status.Danger.Foreground.Alpha(0.8 * a)
	}
	return el.Theme.Text.Muted.Alpha(0.7 * a)
}

// paintInline draws an inline formula in its object box o: its math or
// hint, centered, and, while its card is open, an accent underline that
// grows out from its middle.
func (el *Element) paintInline(p *ui.Painter, sg seg, o ui.Rect) {
	lk := el.mathLookOf(sg.node.ID())
	cx := o.X + o.W/2
	base := o.Y + o.H - sg.face.Size*0.25
	if ms := sg.math; ms.box != nil {
		base = o.Y + ms.ascent
	}
	if lk.open > 0.001 {
		w := o.W * (0.3 + 0.7*lk.open)
		p.Fill(ui.Rect{X: cx - w/2, Y: o.Y + o.H + 1.5, W: w, H: 1.5}, el.Theme.Accent.Foreground.Alpha(lk.open), 1)
	}
	if lk.hint > 0.001 {
		// The hint as the layout shaped it, or, fading as math comes, anew.
		lay := sg.shaped
		if lay == nil {
			face := sg.face
			lay = ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size, Italic: true}, ui.TextShape{})
		}
		w, h := lay.Size()
		paintText(p, lay, cx-w/2, base-h*0.78, el.hintInk(sg.text, sg.open, lk.hint))
	}
	if d := lk.drawn; lk.ink > 0.001 && d.box != nil {
		drawFormula(p, d.box, ui.Point{X: cx - (d.to-d.from)/2 - d.from, Y: base}, el.inks(lk.ink))
	}
}

// paintMath draws a math block's frame and display: its math, or its hint
// while it has none, centered in its column, crossfading as one comes and
// the other goes. It returns how strongly its frame, and with it its
// panel, shows.
func (el *Element) paintMath(p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	t, m, ml := el.Theme, el.Editor.metrics, u.math
	lk := el.mathLookOf(u.pres.Unit)
	a := min(panelOpened(u), lk.open)
	if a > 0.001 {
		components.PaintRing(p, frameOf(u, ux, uy), cardPad(m), 0, t.Border.Default.Alpha(a))
		pill := el.dialectPillRect(frameOf(u, ux, uy), ml.dialect, true)
		el.paintDialectPill(p, pill, controlKey{unit: u.pres.Unit, part: partDialect}, ml.dialect, a)
	}
	if lk.hint > 0.001 {
		if ml.hint == nil {
			// Shaped once for the layout, whose theme and measures it reads.
			hm := m
			hm.Body *= displayScale
			sp := span(locales.MathHint(), el.Editor.theme.Resolve(nil), hm)
			sp.Italic = true
			ml.hint = ui.ShapeRichTextWith([]ui.Span{sp}, ui.Font{Family: sp.Font, Size: sp.Size, Italic: true}, ui.TextShape{})
		}
		lay := ml.hint
		w, h := lay.Size()
		paintText(p, lay, ux+(m.Width-u.x-w)/2, uy+(ml.h-h)/2, el.hintInk(u.pres.Node.Text(), heldMath(u), lk.hint))
	}
	d := lk.drawn
	if lk.ink <= 0.001 || d.box == nil {
		return a
	}
	pad := (ml.h - d.ascent - d.descent) / 2
	drawFormula(p, d.box, ui.Point{X: ux, Y: uy + pad + d.ascent}, el.inks(lk.ink))
	return a
}

// heldMath reports whether a math block is held: its source open.
func heldMath(u *unitLayout) bool { return len(u.surfaces) > 0 && u.surfaces[0].open }

// panelOpened is how far a math block's source panel has opened past its
// display.
func panelOpened(u *unitLayout) float32 {
	ml := u.math
	if ml.full <= ml.fold {
		return 1
	}
	return min(max((u.shown-ml.fold)/(ml.full-ml.fold), 0), 1)
}

const partDialect part = "dialect"

func cardPillRoom(m Metrics) float32 { return m.Body * 3.2 }

func inlineFormulaNode(u *unitLayout, anchor doc.ID) *doc.Node {
	if u == nil || len(u.surfaces) == 0 {
		return nil
	}
	for _, sg := range u.surfaces[0].segs {
		if sg.kind == segBox && sg.node != nil && sg.node.ID() == anchor && sg.node.Kind() == profile.Formula {
			return sg.node
		}
	}
	return nil
}

func (el *Element) dialectPillLayout(dialect profile.MathDialect) (*ui.TextLayout, float32, float32) {
	size := el.chromeSize() * 0.78
	font := ui.Font{Family: theme.UIFontFamily, Size: size, Weight: 500}
	lay := ui.ShapeRichTextWith([]ui.Span{{Text: dialect.Label(), Font: font.Family, Size: font.Size, Weight: font.Weight}}, font, ui.TextShape{})
	w, h := lay.Size()
	return lay, w, h
}

func (el *Element) dialectPillRect(container ui.Rect, dialect profile.MathDialect, topRight bool) ui.Rect {
	scale := el.chromeScale()
	_, lw, lh := el.dialectPillLayout(dialect)
	pw := lw + 12*scale
	ph := max(18*scale, lh+4*scale)
	if topRight {
		return ui.Rect{X: container.X + container.W - pw - 8*scale, Y: container.Y + 6*scale, W: pw, H: ph}
	}
	return ui.Rect{X: container.X + container.W - pw - 6*scale, Y: container.Y + (container.H-ph)/2, W: pw, H: ph}
}

func fadeColor(c ui.Color, a float32) ui.Color {
	return c.Alpha(float32(c.A) / 255 * a)
}

func (el *Element) paintDialectPill(p *ui.Painter, rect ui.Rect, key controlKey, dialect profile.MathDialect, alpha float32) {
	scale := el.chromeScale()
	lit := el.lit(key)
	radius := 4 * scale
	bg := fadeColor(el.Theme.Surface.Inset.Mix(el.Theme.Element.Hover, lit), alpha)
	border := fadeColor(el.Theme.Border.Variant.Mix(el.Theme.Accent.Foreground, lit*0.5), alpha)
	ink := fadeColor(el.Theme.Text.Muted.Mix(el.Theme.Accent.Foreground, lit), alpha)
	p.Fill(rect, bg, radius)
	components.PaintRing(p, rect, radius, 0, border)
	lay, lw, lh := el.dialectPillLayout(dialect)
	paintText(p, lay, rect.X+(rect.W-lw)/2, rect.Y+(rect.H-lh)/2, ink)
}

type clickControlDrag struct {
	id     controlKey
	rect   ui.Rect
	last   ui.Point
	action func(at time.Time)
}

func (d *clickControlDrag) key() controlKey                 { return d.id }
func (d *clickControlDrag) move(x, y float32, _ time.Time) { d.last = ui.Point{X: x, Y: y} }
func (d *clickControlDrag) cursor() ui.Cursor               { return ui.CursorPointer }
func (d *clickControlDrag) finish(canceled bool, at time.Time) {
	if !canceled && contains(d.rect, d.last.X, d.last.Y) && d.action != nil {
		d.action(at)
	}
}

func (el *Element) dialectControl(id doc.ID, key controlKey, rect ui.Rect, dialect profile.MathDialect) control {
	next := profile.DialectTypst
	if dialect == profile.DialectTypst {
		next = profile.DialectTeX
	}
	return control{
		key:    key,
		rect:   rect,
		cursor: ui.CursorPointer,
		press: func(x, y float32, _ int, _ bool, _ time.Time) drag {
			return &clickControlDrag{
				id:   key,
				rect: rect,
				last: ui.Point{X: x, Y: y},
				action: func(at time.Time) {
					el.Editor.Glance(vocab.Input{Intent: profile.SetMathDialect{ID: id, Dialect: next}, At: at})
				},
			}
		},
	}
}

func (el *Element) inlineFormulaControls(u *unitLayout) []control {
	if len(u.surfaces) <= 1 {
		return nil
	}
	var out []control
	for _, s := range u.surfaces[1:] {
		if !s.card || !s.shown || s.cardFormula == 0 {
			continue
		}
		rect := el.dialectPillRect(el.panelArea(s, u.x, u.y), s.cardDialect, false)
		key := controlKey{unit: u.pres.Unit, part: partDialect, at: s.index}
		out = append(out, el.dialectControl(s.cardFormula, key, rect, s.cardDialect))
	}
	return out
}
