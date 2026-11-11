package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

// Objects (§13.1) are each drawn by their kind's renderer: a figure's
// picture, a math block's display, an inline formula's math, a divider's
// line. Everything else about them is shared here, so it looks and moves
// alike whatever the kind:
//
//   - what an object draws is its box: u.box for a block object, its
//     object rect in its line for an inline one;
//   - a surface not shown (a closed card, a math block's folded source)
//     stands in that box for every range painted over it, the selection,
//     highlights and pulses alike, so what covers an object shows on it;
//   - held whole, by a node selection or a range over it, an object wears
//     the app's ring around all it draws (u.drawn: its box, or a math
//     block's frame while open), fading in and out.

const (
	// objectRingOutset sets the ring off what the object draws.
	objectRingOutset = 3
	// objectFade springs an object's ring.
	objectFade = float32(18)
)

// standIn is what a surface not shown stands in for, in its unit: the
// box its card hangs from, or its object's box.
func standIn(u *unitLayout, s *surfaceLayout) ui.Rect {
	if s.card && len(u.surfaces) > 0 {
		main := u.surfaces[0]
		for i, sg := range main.segs {
			if sg.kind == segBox && sg.node != nil && sg.node.ID() == s.anchor {
				if r, ok := main.objectRect(i); ok {
					r.X += main.x
					r.Y += main.y
					return r
				}
			}
		}
	}
	return u.box
}

// rangeRows are the rows a range of a surface paints, in the page: its
// text's lines where the surface is shown, else what stands in for it.
func rangeRows(u *unitLayout, s *surfaceLayout, rg ui.TextInputRange) []decoRow {
	if !s.shown {
		r := standIn(u, s)
		r.X += u.x
		r.Y += u.y
		return []decoRow{{box: r, base: r.Y + r.H}}
	}
	return decoRows(s, rg, u.x+s.x, u.y+s.y)
}

// heldBoxes are the boxes, in the page, of the objects a selection holds
// whole: a selected object, or a block object a range covers both gaps of.
func heldBoxes(pl *pageLayout, sel doc.Selection, covers whole) map[doc.ID]ui.Rect {
	out := map[doc.ID]ui.Rect{}
	for _, u := range pl.units {
		if u.object && covers[u.pres.Unit] {
			r := u.drawn
			r.X += u.x
			r.Y += u.y
			out[u.pres.Unit] = r
		}
		if sel.Kind != doc.SelectNode || len(u.surfaces) == 0 {
			continue
		}
		main := u.surfaces[0]
		for i, sg := range main.segs {
			if sg.kind == segBox && sg.node != nil && sg.node.ID() == sel.Node {
				if r, ok := main.objectRect(i); ok {
					r.X += u.x + main.x
					r.Y += u.y + main.y
					out[sel.Node] = r
				}
			}
		}
	}
	return out
}

// whole is which units a selection holds whole.
type whole map[doc.ID]bool

// objectMotion is an object's ring across frames.
type objectMotion struct {
	ring motion.Spring
	box  ui.Rect
	seen bool
}

// paintRings draws the ring of every object held whole, and fades out
// the ring of one no longer held where it stood.
func (el *Element) paintRings(p *ui.Painter, held map[doc.ID]ui.Rect, ox, oy float32, now time.Time, animate bool) {
	if el.objects == nil {
		el.objects = map[doc.ID]*objectMotion{}
	}
	for id, box := range held {
		om, ok := el.objects[id]
		if !ok {
			om = &objectMotion{ring: motion.NewUnitSpring(0, now)}
			el.objects[id] = om
		}
		om.box = box
	}
	color := el.Theme.Accent.Foreground
	for id, om := range el.objects {
		_, on := held[id]
		to := float32(0)
		if on {
			to = 1
		}
		if animate {
			om.ring.Step(to, now, objectFade)
		} else {
			om.ring = motion.NewUnitSpring(to, now)
		}
		if om.ring.Moving() {
			p.AnimationFrame()
		}
		a := om.ring.Value()
		if a <= 0.001 {
			if !on {
				delete(el.objects, id)
			}
			continue
		}
		box := om.box
		box.X += ox
		box.Y += oy
		components.PaintRing(p, box, objectRadius(el.Editor.metrics), objectRingOutset, color.Alpha(0.85*a))
	}
}

// objectRadius rounds what objects draw and their rings.
func objectRadius(m Metrics) float32 { return figureRadius(m) }
