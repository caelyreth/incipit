package page

import (
	"math"
	"slices"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/egoist/mygo/ui"
)

// A table is its cells set out in a grid (vocab.Grid): each cell one of
// its surfaces, laid at its column's width in a little room of its own,
// each row as tall as its tallest cell. Its columns take the text's width
// when their text fits on a line, the room to spare shared evenly; a table
// asking more wraps its long columns and keeps its short ones on a line. A
// table wider than the text spreads into the margins, as far each side,
// and one wider still shows as much as they hold, scrolled across within
// them. Set apart from the text around it (GroupTable's room), it reads as
// one thing on the page.
const (
	// cellPadX and cellPadY are a cell's room around its text, as shares
	// of the body's line.
	cellPadX = 0.4
	cellPadY = 0.2
	// leastColumn is the narrowest a column is set, in DIPs, so a table of
	// many columns still shows each; readColumn the least a column that
	// wraps keeps, in the body's lines, its longest word if wider.
	leastColumn = 32
	readColumn  = 4
)

// tableLayout is a table's grid as it is laid out, in its unit.
type tableLayout struct {
	// columns are each column's left edge from the grid's and widths their
	// widths; rows are each row's top in the unit and heights theirs.
	columns, widths, rows, heights []float32
	// cells are each surface's cell, a row of -1 for one outside the grid.
	cells []doc.CellCoord
	// base is each surface's x with the grid unscrolled.
	base   []float32
	header bool
	// left is the grid's left edge in the unit unscrolled; overflow how far
	// it scrolls across what it shows (the unit's box), and scroll how far
	// it is scrolled as drawn, past an end while it pulls elastic there.
	left, overflow, scroll float32
}

// tableKind draws a table: its cells in a grid.
type tableKind struct{}

func (tableKind) lay(lc *layoutCache, u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, _ int) {
	g, ok := vocab.GridNode(profile.Schema(), p.Node)
	if !ok {
		stackKind{}.lay(lc, u, p, comp, th, m, width, -1)
		return
	}
	shape := profile.TableShapeOf(p.Node)
	rows, columns := g.Size()
	at := make(map[doc.ID]doc.CellCoord, rows*columns)
	for r := range rows {
		for c := range columns {
			cc := doc.CellCoord{Row: r, Column: c}
			at[g.Cell(cc).ID()] = cc
		}
	}
	line := lineHeight(m, m.Body)
	padX := float32(math.Round(float64(line * cellPadX)))
	padY := float32(math.Round(float64(line * cellPadY)))
	t := &tableLayout{header: shape.Header, cells: make([]doc.CellCoord, len(p.Surfaces)), base: make([]float32, len(p.Surfaces)),
		rows: make([]float32, rows), heights: make([]float32, rows)}
	// What each column's text asks: its longest line, and its longest word.
	natural, least := make([]float32, columns), make([]float32, columns)
	for c := range columns {
		natural[c], least[c] = leastColumn, leastColumn
	}
	for i, s := range p.Surfaces {
		cc, ok := at[s.Flow.Slot.Node]
		if !ok {
			t.cells[i] = doc.CellCoord{Row: -1}
			continue
		}
		t.cells[i] = cc
		wide := lc.laySurface(i, s, p, comp, th, m, ui.TextShape{})
		narrow := lc.laySurface(i, s, p, comp, th, m, ui.TextShape{Width: 1, NoBreakWords: true})
		// A line asks at most the room the table may show in, in whole
		// DIPs, so set at what it asks it keeps to its line.
		natural[cc.Column] = max(natural[cc.Column], min(ceil(longestLine(wide.layout))+2*padX, width+2*m.Reach))
		least[cc.Column] = max(least[cc.Column], min(ceil(longestLine(narrow.layout))+2*padX, width))
	}
	t.widths = columnWidths(shape.Columns, width, 2*m.Reach, natural, least, line*readColumn)
	grid := float32(0)
	for _, w := range t.widths {
		t.columns = append(t.columns, grid)
		grid += w
	}
	// Past the column, as far each side; past the margins, scrolled.
	shows := min(grid, width+2*m.Reach)
	t.left = min(0, (width-shows)/2)
	t.overflow = max(0, grid-shows)
	for i, s := range p.Surfaces {
		cc := t.cells[i]
		if cc.Row < 0 {
			sl := lc.laySurface(i, s, p, comp, th, m, ui.TextShape{Width: width})
			sl.shown = false
			u.surfaces = append(u.surfaces, sl)
			continue
		}
		align := ui.Start
		if cc.Column < len(shape.Columns) {
			switch shape.Columns[cc.Column].Align {
			case profile.AlignCenter:
				align = ui.Center
			case profile.AlignEnd:
				align = ui.End
			}
		}
		sl := lc.laySurface(i, s, p, comp, th, m, ui.TextShape{Width: max(1, t.widths[cc.Column]-2*padX), Align: align})
		// A cell shows always: the grid is its room, held or not.
		sl.shown = true
		t.heights[cc.Row] = max(t.heights[cc.Row], sl.h+2*padY)
		u.surfaces = append(u.surfaces, sl)
	}
	y := float32(0)
	for r := range rows {
		t.rows[r] = y
		y += t.heights[r]
	}
	for i, sl := range u.surfaces {
		if cc := t.cells[i]; cc.Row >= 0 {
			t.base[i] = t.left + t.columns[cc.Column] + padX
			sl.x, sl.y = t.base[i], t.rows[cc.Row]+padY
		}
	}
	u.table = t
	u.h = y
	u.box = ui.Rect{X: t.left, W: shows, H: y}
	u.view = u.box
	u.frame = objectRadius(m)
}

func ceil(v float32) float32 { return float32(math.Ceil(float64(v))) }

// longestLine is how wide a layout's longest line is.
func longestLine(l *ui.TextLayout) float32 {
	w := float32(0)
	for _, line := range l.Lines() {
		w = max(w, line.Rect.W)
	}
	return w
}

// columnWidths sets a table's columns in width, as far as reach past it,
// from what each column's text asks: natural, its longest line, and least,
// its longest word. A column of a width keeps it, and a fitted one its
// line. The others share the rest: all on a line if they fit, the room to
// spare levelled up across them; else each short one on its line and the
// long ones levelled down to fit, none under read or its longest word; or,
// past reach, each at that floor.
func columnWidths(shape []profile.Column, width, reach float32, natural, least []float32, read float32) []float32 {
	widths := make([]float32, len(natural))
	open := make([]bool, len(widths))
	floor, want := make([]float32, len(widths)), make([]float32, len(widths))
	left, floors, fits, any := width, float32(0), float32(0), false
	for i := range widths {
		var c profile.Column
		if i < len(shape) {
			c = shape[i]
		}
		switch {
		case c.Width > 0:
			widths[i] = max(float32(c.Width), leastColumn)
		case c.Fit:
			widths[i] = natural[i]
		default:
			open[i], any = true, true
			floor[i] = max(least[i], read)
			want[i] = max(floor[i], natural[i])
			floors += floor[i]
			fits += want[i]
			continue
		}
		left -= widths[i]
	}
	if any {
		if floors > left {
			// Out of the column, the table may as well spare its text a
			// wrap, as far as reach.
			left = min(max(floors, fits), left+reach)
		}
		unbounded := make([]float32, len(widths))
		for i := range unbounded {
			unbounded[i] = float32(math.Inf(1))
		}
		var level []float32
		switch {
		case fits <= left:
			level = levelled(open, left, want, unbounded)
		case floors < left:
			level = levelled(open, left, floor, want)
		default:
			level = floor
		}
		for i := range widths {
			if open[i] {
				widths[i] = level[i]
			}
		}
	}
	// Each column's edges are rounded, not its width, so the columns
	// together are as wide as they are before rounding.
	edge := float32(0)
	for i := range widths {
		from := float32(math.Round(float64(edge)))
		edge += widths[i]
		widths[i] = float32(math.Round(float64(edge))) - from
	}
	return widths
}

// levelled sets the open columns between their bounds low and high at one
// level, as far up as room holds them.
func levelled(open []bool, room float32, low, high []float32) []float32 {
	filled := func(level float32) (sum, slope float32) {
		for i := range low {
			if !open[i] {
				continue
			}
			sum += max(low[i], min(high[i], level))
			if level >= low[i] && level < high[i] {
				slope++
			}
		}
		return sum, slope
	}
	// The sum rises in a straight line between the columns' bounds: find
	// the stretch room falls in, and the level along it.
	bounds := make([]float32, 0, 2*len(low))
	for i := range low {
		if open[i] {
			bounds = append(bounds, low[i])
			if !math.IsInf(float64(high[i]), 1) {
				bounds = append(bounds, high[i])
			}
		}
	}
	slices.Sort(bounds)
	level := bounds[0]
	for _, bound := range bounds {
		if sum, _ := filled(bound); sum > room {
			break
		}
		level = bound
	}
	if sum, slope := filled(level); slope > 0 {
		level += (room - sum) / slope
	}
	widths := make([]float32, len(low))
	for i := range widths {
		widths[i] = max(low[i], min(high[i], level))
	}
	return widths
}

// scrollTo draws the grid scrolled across by x: its cells move with it.
func (t *tableLayout) scrollTo(u *unitLayout, x float32) {
	t.scroll = x
	for i, s := range u.surfaces {
		if t.cells[i].Row >= 0 {
			s.x = t.base[i] - x
		}
	}
}

// cell is a cell's box in its unit, as the grid is scrolled.
func (t *tableLayout) cell(cc doc.CellCoord) ui.Rect {
	return ui.Rect{X: t.left - t.scroll + t.columns[cc.Column], Y: t.rows[cc.Row], W: t.widths[cc.Column], H: t.heights[cc.Row]}
}

// paint draws the grid beneath its cells' text: the header's inset, the
// lines between the cells, and its edge.
func (tableKind) paint(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	t := u.table
	if t == nil {
		return 0
	}
	box := ui.Rect{X: ux + u.box.X, Y: uy + u.box.Y, W: u.box.W, H: u.box.H}
	line := el.Theme.Border.Default
	p.Clip(box, u.frame, func() {
		if t.header && len(t.rows) > 0 {
			p.Fill(ui.Rect{X: box.X, Y: uy + t.rows[0], W: box.W, H: t.heights[0]}, el.Theme.Surface.Inset, 0)
		}
		for r := 1; r < len(t.rows); r++ {
			p.Fill(ui.Rect{X: box.X, Y: uy + t.rows[r], W: box.W, H: 1}, line, 0)
		}
		for c := 1; c < len(t.columns); c++ {
			p.Fill(ui.Rect{X: ux + t.left - t.scroll + t.columns[c], Y: box.Y, W: 1, H: box.H}, line, 0)
		}
	})
	p.Stroke(box, line, u.frame, 1)
	return 0
}

// over fades the sides of a table wider than its room where more of it
// lies past them, as far as it does, up to a line's width.
func (tableKind) over(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) {
	t := u.table
	if t == nil || t.overflow <= 0 {
		return
	}
	reach := lineHeight(el.Editor.metrics, el.Editor.metrics.Body)
	page := el.Theme.Surface.Page
	box := ui.Rect{X: ux + u.box.X, Y: uy + u.box.Y, W: u.box.W, H: u.box.H}
	if past := min(t.scroll, reach); past > 0.5 {
		p.FillGradient(ui.Rect{X: box.X, Y: box.Y, W: past, H: box.H}, ui.LinearGradient{From: page, To: page.Alpha(0), Angle: 90}, 0)
	}
	if past := min(t.overflow-t.scroll, reach); past > 0.5 {
		p.FillGradient(ui.Rect{X: box.X + box.W - past, Y: box.Y, W: past, H: box.H}, ui.LinearGradient{From: page.Alpha(0), To: page, Angle: 90}, 0)
	}
}

func (tableKind) holds(*unitLayout) int                                        { return 0 }
func (tableKind) caretX(*Element, *unitLayout, *surfaceLayout) (float32, bool) { return 0, false }

// cover is a cell's box.
func (tableKind) cover(u *unitLayout, s *surfaceLayout) (ui.Rect, bool) {
	t := u.table
	if t == nil || s.index >= len(t.cells) || t.cells[s.index].Row < 0 {
		return ui.Rect{}, false
	}
	return t.cell(t.cells[s.index]), true
}

// tableScroll is how far a table wider than its room is scrolled across:
// by hand, pulling elastic past its ends, or gliding to reveal the caret.
// It is the page's, not the layout's, so it lasts as the table changes.
type tableScroll struct {
	x     float32
	over  scrollfx.Elastic
	glide motion.Glide
}

// scrollTables draws each table scrolled as far as it is, within what it
// scrolls; a paint steps the scrolls at its own time. It reports whether
// any still moves.
func (el *Element) scrollTables(pl *pageLayout, now time.Time, frame, animate bool) bool {
	if el.tables == nil {
		el.tables = map[doc.ID]*tableScroll{}
	}
	moving := false
	seen := map[doc.ID]bool{}
	for _, u := range pl.units {
		t := u.table
		if t == nil {
			continue
		}
		ts, ok := el.tables[u.pres.Unit]
		if !ok && t.overflow <= 0 {
			t.scrollTo(u, 0)
			continue
		}
		if !ok {
			ts = &tableScroll{}
			el.tables[u.pres.Unit] = ts
		}
		seen[u.pres.Unit] = true
		if frame {
			_, pulling := ts.over.Step(now, animate)
			var gliding bool
			ts.x, gliding = ts.glide.Step(ts.x, now, animate)
			moving = moving || pulling || gliding
		}
		ts.x = max(0, min(ts.x, t.overflow))
		t.scrollTo(u, ts.x-ts.over.Offset())
	}
	for id := range el.tables {
		if !seen[id] {
			delete(el.tables, id)
		}
	}
	return moving
}

// scrollTable scrolls the table under the page point x, y across by dx,
// by hand, and reports whether there is one that scrolls.
func (el *Element) scrollTable(x, y float32, ev ui.InputEvent, dx float32) bool {
	pl := el.Editor.layout
	if pl == nil {
		return false
	}
	for _, u := range pl.units {
		t := u.table
		if t == nil || t.overflow <= 0 || y < u.y || y >= u.bottom() || x < u.x+u.box.X || x >= u.x+u.box.X+u.box.W {
			continue
		}
		ts := el.tables[u.pres.Unit]
		if ts == nil {
			ts = &tableScroll{}
			el.tables[u.pres.Unit] = ts
		}
		ts.glide.Cancel()
		ts.x = components.OverscrollPanel.Scroll(&ts.over, ts.x, dx, 0, t.overflow, u.box.W, 0, scrollfx.Phase(ev.Precise, ev.Momentum))
		t.scrollTo(u, ts.x-ts.over.Offset())
		return true
	}
	return false
}

// revealAcross glides a table the caret is in across, as far as brings
// the caret's cell into view clear of the fades at its edges (over), or,
// in a cell wider than that, the caret.
func (el *Element) revealAcross(pl *pageLayout, now time.Time) {
	pc, ok := pl.lookup(el.Editor.Selection().Head)
	if !ok || pc.surface == nil || pc.unit.table == nil || pc.unit.table.overflow <= 0 {
		return
	}
	u, t, s := pc.unit, pc.unit.table, pc.surface
	if s.index >= len(t.cells) || t.cells[s.index].Row < 0 {
		return
	}
	ts := el.tables[u.pres.Unit]
	if ts == nil {
		ts = &tableScroll{}
		el.tables[u.pres.Unit] = ts
	}
	// Across the grid, from what the table shows.
	m := el.Editor.metrics
	reach := lineHeight(m, m.Body)
	c := t.cells[s.index].Column
	from, w := t.columns[c]-reach, t.widths[c]+2*reach
	if w > u.box.W {
		from, w = t.base[s.index]-t.left+s.caretRect(pc.stop.u).X-reach, 2*reach
	}
	target, changed := motion.Reveal(ts.x, u.box.W, from, w, false, false)
	if !changed {
		return
	}
	target = max(0, min(target, t.overflow))
	if target != ts.x || ts.glide.Moving() {
		ts.glide.To(ts.x, target, now)
	}
}
