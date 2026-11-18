package shell

import (
	"math"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The list posture is the field read in order (field-and-desk.md §4.7.6):
// the pill stands centred on the window's height at its open width,
// Neverland's name over it and its ribbons, what it says under them, the
// set laid out about the pill's middle; and the cards stand in one column
// beside it, top down, in the order the field ranks them, no wider than
// listMost, the set and the column centred in the field together. It is
// the same query, cards and actions; only where they stand changes, so
// going between the postures is the field's own change: the cards go from
// their slots to the column on the change's spring, and the pill, its
// ribbons, name and stats go across on one spring of their own, which,
// as every spring, turns from wherever it is when it is pressed again.

// Its column's measures at the reference type size: the card is no
// narrower than listLeast and no wider than listMost.
const listLeast, listMost = 280, 520

// listAfter is the room, at the reference type size, the list keeps
// between the pill's end and the column: for the pill's actions.
const listAfter = 140

// listGeometry is where the list posture stands in a field w wide: the
// pill's left, its column's left, and the column's card width.
type listGeometry struct {
	w, pillLeft, colLeft, cardW float32
	// fits reports whether the window holds the pill and the column side
	// by side.
	fits bool
}

// listLayout is the list posture's geometry in a field w wide, the pill
// at most pillW wide: the pill, listAfter for its actions, and the column
// taking the rest up to its widest, the whole centred.
func listLayout(w, pillW, k float32) listGeometry {
	margin := 24 * k
	block := pillW + listAfter*k
	cardW := min(listMost*k, w-2*margin-block)
	g := listGeometry{w: w, cardW: max(cardW, listLeast*k), fits: cardW >= listLeast*k}
	left := max(margin, (w-block-g.cardW)/2)
	g.pillLeft, g.colLeft = left, left+block
	return g
}

// listSide is how much further, as the list (Translate adds to where it
// stands), what stands before the pill on the
// horizon (before) or after it moves, el being as wide as it was last
// drawn: over the pill and its ribbons, or under them, centred on it.
func (s *State) listSide(el ui.Element, r stageRect, before bool, gap, k float32) stagePoint {
	t := min(1, max(0, s.field.listed))
	if t <= 0 {
		return stagePoint{}
	}
	elW := el.LayoutBounds().W
	off := 18 * k
	// It goes the way round the ribbons, not through them: clear of them
	// over or under first, then across, and back the same way.
	across, up := t*t*t, 1-(1-t)*(1-t)*(1-t)
	if before {
		// From its right at the pill's left less gap.
		return stagePoint{X: across * (r.W/2 + gap + elW/2), Y: -up * (s.field.band.above + off + r.H/2)}
	}
	// From its left at the pill's end and gap.
	return stagePoint{X: -across * (r.W/2 + gap + elW/2), Y: up * (s.field.band.below + off + r.H/2)}
}

// place stands blocks of heights in the column, gap apart, in a window h
// tall with top and foot kept clear, the horizon's middle at the origin:
// centred on the pill's line where they fit, else from the window's top
// down, the field scrolling them.
func (g listGeometry) place(heights []float32, h, top, foot, gap float32) []stageRect {
	total := float32(0)
	for i, hh := range heights {
		if i > 0 {
			total += gap
		}
		total += hh
	}
	y := -total / 2
	if total > h-top-foot {
		y = -(h/2 - top)
	}
	out := make([]stageRect, len(heights))
	for i, hh := range heights {
		out[i] = stageRect{X: g.colLeft - g.w/2, Y: y, W: g.cardW, H: hh}
		y += hh + gap
	}
	return out
}

// listMetrics are m for the list posture's column: one card wide, and as
// tall as the window holds.
func (g listGeometry) metrics(m blockMetrics) blockMetrics {
	m.col = g.cardW - 2*m.pad
	m.tall = m.alone
	m.horizon.Cols, m.horizon.Col = 1, g.cardW
	return m
}

// toggleList goes between the horizon and the list posture.
func (s *State) toggleList() { s.field.list = !s.field.list }

// posturedShown is how far a card shows, shown as it stands, the field
// listed so far between the postures: going across the pill's way, it is
// dimmed most halfway, as it passes under the pill and its ribbons, and
// whole at either end; turned back on the way, it brightens from as dim
// as it was.
func posturedShown(shown, listed float32) float32 {
	t := min(1, max(0, listed))
	return shown * (1 - 0.6*float32(math.Sin(math.Pi*float64(t))))
}

// listButton turns the list posture on and off, filled while it stands.
func (s *State) listButton(c *ui.Context, semantic theme.Theme, size float32, animate bool) ui.Element {
	label := map[bool]string{true: "Show as Field", false: "Show as List"}[s.field.list]
	btn := s.laneCircleButton(c, semantic, "list", components.IconList, label, label, s.field.list, size, animate)
	if btn.Clicked() {
		s.toggleList()
	}
	return btn
}
