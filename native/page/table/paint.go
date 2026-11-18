package table

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// How a table's controls look: as the app's own controls do, sized by
// theme.DefaultControls and its spacing at the UI size.
//
//   - An area is drawn as components.CreateArea is: a dashed outline that
//     only hints at rest and takes the accent under the pointer, over its
//     tint, with a plus and what it makes, its icon and label coming in as
//     its room opens; pressed, it gives a little.
//   - A grip is an icon on nothing at rest, as the app's quiet buttons are,
//     its ghost tint coming in under the pointer and the accent as it is
//     held; the corner's, which resizes rather than moves, is a knob that
//     blends to the accent. A grip called up again glides there from where
//     it was; one that was not shown comes in where it is.
//   - A lit border is an accent line down the table.
//   - A carried line's copy is drawn over the page with nothing behind it,
//     the line veiled in its place, a drop frame where the grid parts; as
//     lines glide to new places, the one going furthest is drawn so, over
//     the others.
//   - A made line shows the area it was, fading into its cells; a corner's
//     Drag outlines and names the size it would set.

// chrome draws a table's controls over the page. It reports whether any
// still moves besides their springs.
func (Kind) Chrome(h Host, p *ui.Painter, u *Unit, ox, oy float32) bool {
	if u.Table == nil {
		return false
	}
	now := h.Now()
	for _, a := range areas(h, u) {
		paintArea(h, p, u, a, ox, oy)
	}
	moving := paintMade(h, p, u, ox, oy, now)
	paintCarry(h, p, u, ox, oy)
	paintBorder(h, p, u, ox, oy)
	paintGrips(h, p, u, ox, oy)
	paintResize(h, p, u, ox, oy)
	return moving
}

// paintArea draws an area.
func paintArea(h Host, p *ui.Painter, u *Unit, a tableArea, ox, oy float32) {
	th := h.Theme()
	size, scale := h.ChromeSize(), h.ChromeScale()
	metrics := h.ControlMetrics()
	key := ControlKey{Unit: u.Pres.Unit, Part: a.Part}
	hover := h.Lit(key)
	box := a.box
	box.X += ox + a.shift
	box.Y += oy
	press := float32(0)
	if h.Holding() == key {
		press = h.SpringValue(springKey(u.Pres.Unit, "press", ShareValue))
	}
	alpha := a.alpha
	draw := func() {
		p.Fill(box, components.Blend(ui.Transparent, th.Accent.Tint, hover).Alpha(alpha), metrics.Radius)
		components.DashedOutline(p, box, metrics.Radius, 1, components.Blend(th.Border.Default, th.Accent.Foreground, hover).Alpha(alpha))
		ink := components.Blend(th.Text.Muted, th.Accent.Foreground, hover)
		icon := 16 * scale
		label := ui.Span{Text: "Row", Font: theme.UIFontFamily, Size: theme.TextSmall.Size(size), Color: ink.Alpha(alpha * a.label)}
		if a.Part == partColumnArea || a.Part == partBeside {
			label.Text = "Column"
		}
		iconBox := ui.Rect{X: box.X + (box.W-icon)/2, Y: box.Y + (box.H-icon)/2, W: icon, H: icon}
		if !a.compact {
			iconBox.X = box.X + theme.Space08.DIPs(size)
		}
		p.Clip(box, metrics.Radius, func() {
			p.Icon(components.IconPlus.SVG(), iconBox, ink.Alpha(alpha*a.icon))
			if !a.compact {
				_, h := p.MeasureText(0, label)
				p.RichText(iconBox.X+icon+theme.Space06.DIPs(size), box.Y+(box.H-h)/2, 0, label)
			}
		})
	}
	if press > 0.001 {
		k := 1 - 0.02*press
		p.Transform(ui.Scaling(k, k), box.X+box.W/2, box.Y+box.H/2, draw)
		return
	}
	draw()
}

// paintMade shows a line an area made as the area it was, its tint and
// dashed outline fading into the cells.
func paintMade(h Host, p *ui.Painter, u *Unit, ox, oy float32, now time.Time) bool {
	ts := h.State(u.Pres.Unit, false)
	if ts == nil || ts.made == nil {
		return false
	}
	m, t := ts.made, u.Table
	n := len(t.rows)
	if m.column {
		n = len(t.columns)
	}
	if m.index >= n {
		return false
	}
	r := t.line(u, !m.column, m.index)
	r.X += ox + u.X
	r.Y += oy + u.Y
	f := components.Smoothstep(1 - float32(now.Sub(m.at))/float32(madeFade))
	radius := h.ObjectRadius(h.Metrics())
	p.Fill(r, h.Theme().Accent.Tint.Alpha(f), radius)
	components.DashedOutline(p, r, radius, 1, h.Theme().Accent.Foreground.Alpha(f))
	return true
}

// paintCarry shows a carry under way: the frame where it would land, the
// line veiled in its place, and its copy where the pointer carries it; or,
// as lines glide to new places, the one going furthest over the others,
// so their text never crosses.
func paintCarry(h Host, p *ui.Painter, u *Unit, ox, oy float32) {
	ts := h.State(u.Pres.Unit, false)
	if ts == nil {
		return
	}
	t := u.Table
	id := u.Pres.Unit
	radius := h.ObjectRadius(h.Metrics())
	if axis, index, ok := t.furthest(); ok {
		place := t.line(u, axis == 0, index)
		paintCarried(h, p, u, axis == 0, index, ui.Point{X: place.X, Y: place.Y}, 1, ox, oy)
	}
	d, ok := h.ActiveDrag().(*lineDrag)
	if !ok || !d.moved || d.table != id {
		return
	}
	for k, in := range t.in.apart {
		frame := t.apart[k]
		if in.room <= 0 || !ts.framed(in.rows, in.at) {
			continue
		}
		// The frame takes the line's size within its room, and comes in
		// as its room opens.
		frame = t.across(frame)
		if d.rows {
			frame.H = min(frame.H, d.size)
		} else {
			frame.W = min(frame.W, d.size)
		}
		frame.X += ox + u.X
		frame.Y += oy + u.Y
		components.DropFrame(p, frame, radius, min(1, in.room/max(1, d.size)), h.Theme())
	}
	paintCarried(h, p, u, d.rows, d.tablePress.at, d.corner(), carryVeil, ox, oy)
}

// framed reports whether the room before line at shows a drop frame.
func (ts *State) framed(rows bool, at int) bool {
	for _, a := range ts.apart {
		if a.rows == rows && a.at == at {
			return a.frame
		}
	}
	return false
}

// paintCarried veils a line in its place and draws its copy with its
// corner at, in the unit: the lines between its cells, its edge, and its
// cells' text.
func paintCarried(h Host, p *ui.Painter, u *Unit, rows bool, index int, at ui.Point, veil, ox, oy float32) {
	t := u.Table
	n := len(t.columns)
	if rows {
		n = len(t.rows)
	}
	if index < 0 || index >= n {
		return
	}
	ux, uy := ox+u.X, oy+u.Y
	place := t.line(u, rows, index)
	place.X += ux
	place.Y += uy
	p.Fill(place, h.Theme().Surface.Page.Alpha(veil), 0)
	dx, dy := at.X+ux-place.X, at.Y+uy-place.Y
	copied := place
	copied.X += dx
	copied.Y += dy
	p.Clip(grown(copied, 1, 1), 0, func() {
		t.paintGrid(h, p, copied, 0, ux+dx, uy+dy, !rows, rows)
		for i, s := range u.Surfaces {
			cc := t.cells[i]
			if cc.Row >= 0 && (rows && cc.Row == index || !rows && cc.Column == index) {
				s.Paint(p, ux+dx, uy+dy)
			}
		}
	})
}

// paintBorder lights the column border the pointer is on or a Drag holds.
func paintBorder(h Host, p *ui.Painter, u *Unit, ox, oy float32) {
	t := u.Table
	ts := h.State(u.Pres.Unit, false)
	lit := h.SpringValue(springKey(u.Pres.Unit, "border", ShareValue))
	if ts == nil || lit < 0.01 || ts.borderAt < 0 || ts.borderAt >= len(t.columns) {
		return
	}
	b := tableBox(u)
	b.X += ox
	b.Y += oy
	c := t.cell(doc.CellCoord{Column: ts.borderAt})
	x := ox + u.X + c.X + c.W
	p.Clip(b, u.Frame, func() {
		p.Fill(ui.Rect{X: x - borderRing/2, Y: b.Y, W: borderRing, H: b.H}, h.Theme().Accent.Foreground.Alpha(0.85*lit), 0)
	})
}

// paintGrips draws the grips of the row and the column under the pointer
// and the table's corner, as far as they have come in.
func paintGrips(h Host, p *ui.Painter, u *Unit, ox, oy float32) {
	id := u.Pres.Unit
	shown := h.SpringValue(springKey(id, "grips", ShareValue))
	if shown < 0.01 {
		return
	}
	th := h.Theme()
	metrics := h.ControlMetrics()
	row, column := gripLines(h, u)
	rr, cr, corner := gripRects(h, u, row, column)
	carrying := false
	if d, ok := h.ActiveDrag().(*lineDrag); ok {
		carrying = d.moved
	}
	for k, g := range []struct {
		rect ui.Rect
		Part Part
		at   int
	}{{rr, partRowGrip, row}, {cr, partColumnGrip, column}, {corner, partCorner, 0}} {
		r := g.rect
		if r.W <= 0 {
			continue
		}
		// It glides from where it was, but comes in where it is from out
		// of view.
		xKey, yKey := SpringKey{Unit: id, Name: "grip-x", At: k, Of: PlaceValue}, SpringKey{Unit: id, Name: "grip-y", At: k, Of: PlaceValue}
		if top := r.Y + oy; top+r.H < 0 || top > h.Viewport().Y+h.Viewport().H {
			h.SpringJump(xKey, r.X)
			h.SpringJump(yKey, r.Y)
		}
		r.X = h.SpringFollow(xKey, r.X, chromeFollow) + ox
		r.Y = h.SpringFollow(yKey, r.Y, chromeFollow) + oy
		key := ControlKey{Unit: id, Part: g.Part, At: g.at}
		hover := h.Lit(key)
		if g.Part == partCorner {
			radius := min(r.W, r.H) / 2
			p.Shadow(r, radius, 0, 1, 4, 0, ui.Color{A: 255}.Alpha(0.2*shown))
			p.Fill(r, components.Blend(th.Surface.Elevated, th.Accent.Foreground, hover).Alpha(shown), radius)
			p.Stroke(r, components.Blend(th.Border.Default, th.Accent.Foreground, hover).Alpha(shown), radius, 1)
			continue
		}
		icon := components.IconGrip
		if g.Part == partColumnGrip {
			icon = components.IconGripAcross
		}
		p.Fill(r, th.GhostElement.Hover.Alpha(hover*shown), metrics.Radius)
		ink := components.Blend(th.Icon.Muted, th.Icon.Default, hover)
		if h.Holding() == key || carrying {
			ink = th.Accent.Foreground
		}
		size := gripIcon * h.ChromeScale()
		p.Icon(icon.SVG(), ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size)/2, W: size, H: size}, ink.Alpha(shown))
	}
}

// paintResize shows the size a corner's Drag would make the table,
// outlined and named under its far corner.
func paintResize(h Host, p *ui.Painter, u *Unit, ox, oy float32) {
	d, ok := h.ActiveDrag().(*cornerDrag)
	if !ok || d.table != u.Pres.Unit || !d.moved {
		return
	}
	t := u.Table
	scale := h.ChromeScale()
	b := tableBox(u)
	b.X += ox
	b.Y += oy
	rows, columns := len(t.rows), len(t.columns)
	r := ui.Rect{X: b.X, Y: b.Y,
		W: max(b.W/float32(columns), b.W*float32(d.columns)/float32(columns)),
		H: max(b.H/float32(rows), b.H*float32(d.rows)/float32(rows))}
	components.DashedOutline(p, r, h.ObjectRadius(h.Metrics()), 1.5, h.Theme().Accent.Foreground)
	h.PaintChip(p, fmt.Sprintf("%d × %d", d.rows, d.columns), func(chip ui.Rect) (ui.Rect, bool) {
		// The view's right edge, in the paint's frame.
		right := ox - h.Column() + h.Viewport().W - 8*scale
		chip.X, chip.Y = min(r.X+r.W, right)-chip.W, r.Y+r.H+4*scale
		return chip, true
	})
}
