package page

import (
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/formula"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

// Formulas are drawn as math, set by native/formula, wherever they are:
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
	// mathPad is the room either side of inline math.
	mathPad = 2
	// displayScale sets display math over the body text.
	displayScale = 1.15
	// staleInk is how strongly math shows while its source does not set.
	staleInk = 0.45
	// displayLines is the least a math block's display area holds, in
	// display lines, so a block keeps its room as its first math sets.
	displayLines = 2.7
	// mathHint is what a math block's display shows while it has no math,
	// and formulaHint an inline formula.
	mathHint    = "Formula"
	formulaHint = "formula"
	// mathFrequency springs math's fades.
	mathFrequency = float32(16)
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

// lastGood keys a formula's last sound source by its node and mode.
type lastGood struct {
	node    doc.ID
	display bool
}

// typeset sets a formula's source: as written when it sets soundly, else
// the last source of that formula that did, marked stale; a zero mathSet
// when none has.
func (lc *layoutCache) typeset(id doc.ID, src string, display bool, size, width float32) mathSet {
	k := lastGood{id, display}
	if strings.TrimSpace(src) == "" {
		// Emptied, a formula has no math: it shows its hint.
		delete(lc.good, k)
		return mathSet{}
	}
	if !formula.Sound(src, display) {
		last, ok := lc.good[k]
		if !ok {
			return mathSet{}
		}
		ms := setMath(last, display, size, width)
		ms.stale = true
		return ms
	}
	if len(lc.good) > 4096 {
		clear(lc.good)
	}
	lc.good[k] = src
	return setMath(src, display, size, width)
}

func setMath(src string, display bool, size, width float32) mathSet {
	box := formula.Set(src, display, size, width)
	if box == nil {
		return mathSet{}
	}
	ms := mathSet{box: box}
	ms.ascent, ms.descent = formula.Extent(box)
	ms.from, ms.to = formula.Ink(box)
	return ms
}

// mathLayout is a math block's display: its formula, and how high its
// display stands over its source panel.
type mathLayout struct {
	mathSet
	h float32
	// fold and full are the block's height with its source panel closed
	// and open; open is whether the panel shows: while the block is held
	// (its source surface open), or always while it has no math.
	fold, full float32
	open       bool
}

// layMath lays out a math block's display over its source panel.
func layMath(u *unitLayout, lc *layoutCache, m Metrics, width float32) {
	if len(u.surfaces) == 0 {
		return
	}
	s := u.surfaces[0]
	ms := lc.typeset(u.pres.Unit, u.pres.Node.Text(), true, m.Body*displayScale, width)
	ml := &mathLayout{mathSet: ms, open: s.open || ms.box == nil}
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
	// its frame around display and panel, which its ring goes around.
	u.box = ui.Rect{W: width, H: ml.h}
	if ml.open && ms.box != nil {
		u.drawn = ui.Rect{W: width, H: u.h}
	}
}

// mathMotion is a formula's springs across frames, and the math it last
// drew, which fades out where its math has gone.
type mathMotion struct {
	ink, open, hint motion.Spring
	drawn           mathSet
	seen            bool
}

// mathLook is how strongly a formula's parts show this frame: its math's
// ink, its hint, and what its being edited opens (an inline formula's
// underline, a math block's frame and panel).
type mathLook struct {
	ink, open, hint float32
	// drawn is the math to draw at ink: the formula's, or the last it had.
	drawn mathSet
}

// mathLookOf steps a formula's springs toward where they are going. All
// start from nothing, so a formula fades in as it first shows: its math
// or hint, its underline or frame. Its hint leaves at once as math
// arrives; math leaving fades out as its hint fades in. Math dims and
// brightens as it goes stale and sound.
func (el *Element) mathLookOf(id doc.ID, ms mathSet, open bool, p *ui.Painter) mathLook {
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
	now := el.paintNow
	mm, ok := el.maths[id]
	if !ok {
		mm = &mathMotion{ink: motion.NewUnitSpring(0, now), open: motion.NewUnitSpring(0, now), hint: motion.NewUnitSpring(0, now)}
		el.maths[id] = mm
	}
	mm.seen = true
	if ms.box != nil {
		mm.drawn = ms
	}
	if hint == 0 {
		// A hint leaves as math arrives, as a field's placeholder does;
		// it would only crowd what arrives.
		mm.hint = motion.NewUnitSpring(0, now)
	}
	if el.animate {
		mm.ink.Step(ink, now, mathFrequency)
		mm.open.Step(opened, now, mathFrequency)
		mm.hint.Step(hint, now, mathFrequency)
		if mm.ink.Moving() || mm.open.Moving() || mm.hint.Moving() {
			p.AnimationFrame()
		}
	} else {
		mm.ink, mm.open, mm.hint = motion.NewUnitSpring(ink, now), motion.NewUnitSpring(opened, now), motion.NewUnitSpring(hint, now)
	}
	return mathLook{ink: mm.ink.Value(), open: mm.open.Value(), hint: mm.hint.Value(), drawn: mm.drawn}
}

// inks are the colors math is drawn in, at a strength.
func (el *Element) inks(a float32) formula.Inks {
	t := el.Theme
	return formula.Inks{Text: t.Prose.Text.Alpha(a), Error: t.Status.Danger.Foreground.Alpha(a), Placeholder: t.Text.Muted.Alpha(a)}
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
	lk := el.mathLookOf(sg.node.ID(), sg.math, sg.open, p)
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
		face := sg.face
		lay := ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size, Italic: true}, ui.TextShape{})
		w, h := lay.Size()
		p.TextLayout(lay, cx-w/2, base-h*0.78, el.hintInk(sg.text, sg.open, lk.hint))
	}
	if d := lk.drawn; lk.ink > 0.001 && d.box != nil {
		formula.Draw(p, d.box, ui.Point{X: cx - (d.to-d.from)/2 - d.from, Y: base}, el.inks(lk.ink))
	}
}

// paintMath draws a math block's frame and display: its math, or its hint
// while it has none, centered in its column, crossfading as one comes and
// the other goes. It returns how strongly its frame, and with it its
// panel, shows.
func (el *Element) paintMath(p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	t, m, ml := el.Theme, el.Editor.metrics, u.math
	lk := el.mathLookOf(u.pres.Unit, ml.mathSet, ml.open, p)
	a := min(panelOpened(u), lk.open)
	if a > 0.001 {
		p.Stroke(ui.Rect{X: ux + 0.5, Y: uy + 0.5, W: m.Width - u.x - 1, H: u.shown - 1}, t.Border.Default.Alpha(a), cardPad(m), 1)
	}
	if lk.hint > 0.001 {
		sp := span(mathHint, el.Editor.theme.Resolve(nil), m.Body*displayScale)
		sp.Italic = true
		lay := ui.ShapeRichTextWith([]ui.Span{sp}, ui.Font{Family: sp.Font, Size: sp.Size, Italic: true}, ui.TextShape{})
		w, h := lay.Size()
		p.TextLayout(lay, ux+(m.Width-u.x-w)/2, uy+(ml.h-h)/2, el.hintInk(u.pres.Node.Text(), heldMath(u), lk.hint))
	}
	d := lk.drawn
	if lk.ink <= 0.001 || d.box == nil {
		return a
	}
	pad := (ml.h - d.ascent - d.descent) / 2
	formula.Draw(p, d.box, ui.Point{X: ux, Y: uy + pad + d.ascent}, el.inks(lk.ink))
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

// sweepMaths forgets the motion of formulas no longer drawn.
func (el *Element) sweepMaths() {
	for id, mm := range el.maths {
		if !mm.seen {
			delete(el.maths, id)
		}
		mm.seen = false
	}
}
