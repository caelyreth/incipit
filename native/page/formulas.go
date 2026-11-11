package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
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
//   - a math block shows its formula set for display, centered; its source
//     is shown under it, in an inset panel, only while the caret or a
//     selection end is in the block, the panel opening and closing as the
//     page below glides with it.
//
// A source half typed does not set soundly: the math shown is the last
// source that did, dimmed, until it sets again. A formula that never has
// shows its source. Math fades in as it first shows, and dims and brightens
// as it goes stale and sound again.

const (
	// mathPad is the room either side of inline math.
	mathPad = 2
	// displayScale sets display math over the body text.
	displayScale = 1.15
	// staleInk is how strongly math shows while its source does not set.
	staleInk = 0.45
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

// typeset sets a formula's source: as written when it sets soundly,
// else the last source of that formula that did, marked stale; a zero
// mathSet when none has.
func (lc *layoutCache) typeset(id doc.ID, src string, display bool, size, width float32) mathSet {
	k := lastGood{id, display}
	if formula.Sound(src, display) {
		if len(lc.good) > 4096 {
			clear(lc.good)
		}
		lc.good[k] = src
	} else if last, ok := lc.good[k]; ok {
		ms := setMath(last, display, size, width)
		ms.stale = true
		return ms
	} else {
		return mathSet{}
	}
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
	// and open; held is whether a selection end is in it.
	fold, full float32
	held       bool
}

// layMath lays out a math block's display over its source: the source
// panel opens only while the block is held.
func layMath(u *unitLayout, lc *layoutCache, m Metrics, width float32, held bool) {
	if len(u.surfaces) == 0 {
		return
	}
	ms := lc.typeset(u.pres.Unit, u.pres.Node.Text(), true, m.Body*displayScale, width)
	if ms.box == nil {
		// Nothing has set yet: the block shows its source.
		return
	}
	pad := m.Body * 0.45
	ml := &mathLayout{mathSet: ms, held: held}
	ml.h = ms.ascent + ms.descent + 2*pad
	s := u.surfaces[0]
	s.x, s.y = cardPad(m), ml.h+cardPad(m)
	ml.fold, ml.full = ml.h, s.y+s.h+cardPad(m)*0.75
	u.math = ml
	u.h = ml.fold
	if held {
		u.h = ml.full
	}
}

// heldBy reports whether a selection has an end in a unit's own text.
func heldBy(p *present.Presentation, sel doc.Selection) bool {
	if sel.Kind != doc.SelectText {
		return false
	}
	for _, s := range p.Surfaces {
		if s.Flow.Slot.Node == sel.Anchor.Pos.Node || s.Flow.Slot.Node == sel.Head.Pos.Node {
			return true
		}
	}
	return false
}

// mathMotion is a formula's springs across frames: its ink, as it shows
// and goes stale, and the fill its open card gives it.
type mathMotion struct {
	ink, fill motion.Spring
	seen      bool
}

// mathAlpha steps a formula's springs toward where they are going and
// returns its ink's and its fill's strength.
func (el *Element) mathAlpha(id doc.ID, stale, open bool, now time.Time, animate bool, p *ui.Painter) (float32, float32) {
	ink, fill := float32(1), float32(0)
	if stale {
		ink = staleInk
	}
	if open {
		fill = 1
	}
	mm, ok := el.maths[id]
	if !ok {
		// Math fades in as it first shows; a fill starts where it is.
		mm = &mathMotion{ink: motion.NewUnitSpring(0, now), fill: motion.NewUnitSpring(fill, now)}
		el.maths[id] = mm
	}
	mm.seen = true
	if animate {
		mm.ink.Step(ink, now, mathFrequency)
		mm.fill.Step(fill, now, mathFrequency)
		if mm.ink.Moving() || mm.fill.Moving() {
			p.AnimationFrame()
		}
	} else {
		mm.ink, mm.fill = motion.NewUnitSpring(ink, now), motion.NewUnitSpring(fill, now)
	}
	return mm.ink.Value(), mm.fill.Value()
}

// inks are the colors math is drawn in, at a strength.
func (el *Element) inks(a float32) formula.Inks {
	t := el.Theme
	return formula.Inks{Text: t.Prose.Text.Alpha(a), Error: t.Status.Danger.Foreground.Alpha(a), Placeholder: t.Text.Muted.Alpha(a)}
}

// paintMath draws a math block's display, centered in its column.
func (el *Element) paintMath(p *ui.Painter, u *unitLayout, ux, uy float32, covered bool, now time.Time, animate bool) {
	ml := u.math
	ink, _ := el.mathAlpha(u.pres.Unit, ml.stale, false, now, animate, p)
	pad := (ml.h - ml.ascent - ml.descent) / 2
	if covered {
		// Held by a selection with its source closed, the display is
		// washed as the text it stands for would be.
		x := ux + ml.from - mathPad*2
		p.Fill(ui.Rect{X: x, Y: uy + pad/2, W: ml.to - ml.from + mathPad*4, H: ml.h - pad}, el.Theme.Prose.Selection.Alpha(el.selection.Opacity()), 4)
	}
	formula.Draw(p, ml.box, ui.Point{X: ux, Y: uy + pad + ml.ascent}, el.inks(ink))
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
