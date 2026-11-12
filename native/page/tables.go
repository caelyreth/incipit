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
	// base and baseY are each surface's place with the grid unscrolled.
	base, baseY []float32
	header      bool
	// left is the grid's left edge in the unit unscrolled; overflow how far
	// it scrolls across what it shows (the unit's box), and scroll how far
	// it is scrolled as drawn, past an end while it pulls elastic there.
	left, overflow, scroll float32
	// in is what the page asked of the layout, and below, rowGap,
	// columnGap and apart the rooms it gave, in the unit, those across
	// unscrolled: under the foot, between two rows, between two columns,
	// and each that a carry opens, as in.apart has them.
	in                       tableIn
	below, rowGap, columnGap ui.Rect
	apart                    [maxApart]ui.Rect
}

// tableIn is what the page asks of a table's layout while its controls
// show, each as far as it has come: the room an area opens under the foot
// (below), between two rows before row, and between two columns before
// column; the rooms a carry opens before a line; the lines coming in; and
// a column's width as its border's drag sets it. Its zero value asks
// nothing.
type tableIn struct {
	below               float32
	row, column         int
	rowRoom, columnRoom float32
	apart               [maxApart]lineRoom
	// grow are the rows and the columns coming in.
	grow  [2]lineGrow
	sized int
	width float32
}

// maxApart is how many rooms a carry has open at once in a table: the one
// it would land in and those it left, closing.
const maxApart = 3

// lineRoom is room before line at, among the rows or the columns.
type lineRoom struct {
	rows bool
	at   int
	room float32
}

// lineGrow is a run of lines from to to, coming in at share of their
// size.
type lineGrow struct {
	from, to int
	share    float32
}

// apartAt is the room the carry opens before line at.
func (in tableIn) apartAt(rows bool, at int) float32 {
	room := float32(0)
	for _, a := range in.apart {
		if a.rows == rows && a.at == at {
			room += a.room
		}
	}
	return room
}

// share is how far line i has come in.
func (in tableIn) share(rows bool, i int) float32 {
	g := in.grow[1]
	if rows {
		g = in.grow[0]
	}
	if i >= g.from && i < g.to {
		return g.share
	}
	return 1
}

// tableKind draws a table: its cells in a grid.
type tableKind struct{ baseKind }

func (tableKind) lay(lc *layoutCache, u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, in objectIn) {
	g, ok := vocab.GridNode(profile.Schema(), p.Node)
	if !ok {
		stackKind{}.lay(lc, u, p, comp, th, m, width, in)
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
	tin := in.table
	t := &tableLayout{header: shape.Header, cells: make([]doc.CellCoord, len(p.Surfaces)),
		base: make([]float32, len(p.Surfaces)), baseY: make([]float32, len(p.Surfaces)),
		rows: make([]float32, rows), heights: make([]float32, rows), in: tin}
	if tin.width > 0 && tin.sized < len(shape.Columns) {
		// A column's width as its border's drag sets it.
		shape.Columns = slices.Clone(shape.Columns)
		shape.Columns[tin.sized].Width, shape.Columns[tin.sized].Fit = int(math.Round(float64(tin.width))), false
	}
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
	// Room between two columns, an area's or a carry's, is taken from the
	// others; each column as far as it has come in.
	columnRoom := float32(0)
	if tin.column > 0 && tin.column < columns {
		columnRoom = min(tin.columnRoom, width/2)
	}
	apart := float32(0)
	for c := range columns + 1 {
		apart += tin.apartAt(false, c)
	}
	shares := make([]float32, columns)
	for c := range shares {
		shares[c] = tin.share(false, c)
	}
	t.widths = columnWidths(shape.Columns, width-columnRoom-apart, 2*m.Reach, natural, least, line*readColumn, shares)
	grid := float32(0)
	for c := range columns + 1 {
		for k, a := range tin.apart {
			if !a.rows && a.at == c && a.room > 0 {
				t.apart[k] = ui.Rect{X: grid, W: a.room}
				grid += a.room
			}
		}
		if c == tin.column && columnRoom > 0 {
			t.columnGap = ui.Rect{X: grid, W: columnRoom}
			grid += columnRoom
		}
		if c < columns {
			t.columns = append(t.columns, grid)
			grid += t.widths[c]
		}
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
	// Down the grid, each row as far as it has come in, and the rooms
	// between them.
	y := float32(0)
	for r := range rows + 1 {
		for k, a := range tin.apart {
			if a.rows && a.at == r && a.room > 0 {
				t.apart[k] = ui.Rect{X: t.left, Y: y, W: shows, H: a.room}
				y += a.room
			}
		}
		if r == tin.row && r > 0 && r < rows && tin.rowRoom > 0 {
			t.rowGap = ui.Rect{X: t.left, Y: y, W: shows, H: tin.rowRoom}
			y += tin.rowRoom
		}
		if r < rows {
			t.heights[r] *= tin.share(true, r)
			t.rows[r] = y
			y += t.heights[r]
		}
	}
	for i, sl := range u.surfaces {
		if cc := t.cells[i]; cc.Row >= 0 {
			t.base[i] = t.left + t.columns[cc.Column] + padX
			t.baseY[i] = t.rows[cc.Row] + padY*tin.share(true, cc.Row)
			sl.x, sl.y = t.base[i], t.baseY[i]
		}
	}
	// Rooms across stand down the whole grid.
	if t.columnGap.W > 0 {
		t.columnGap.X += t.left
		t.columnGap.H = y
	}
	for k, a := range tin.apart {
		if !a.rows && t.apart[k].W > 0 {
			t.apart[k].X += t.left
			t.apart[k].H = y
		}
	}
	u.table = t
	u.h = y
	u.box = ui.Rect{X: t.left, W: shows, H: y}
	if tin.below > 0 {
		// Under the table, the room of an area that makes a row.
		t.below = ui.Rect{X: t.left, Y: y, W: shows, H: tin.below}
		u.h += tin.below
	}
	u.view = u.box
	u.frame = objectRadius(m)
}

// across is a room across the grid as it is scrolled.
func (t *tableLayout) across(r ui.Rect) ui.Rect {
	r.X -= t.scroll
	return r
}

// parts are the frames the grid is drawn in, in the unit: its box, parted
// by each room open enough within it.
func (t *tableLayout) parts(box ui.Rect) []ui.Rect {
	var downs, acrosses [][2]float32
	if g := t.rowGap; g.H > 0 {
		downs = append(downs, [2]float32{g.Y, g.H})
	}
	if g := t.across(t.columnGap); g.W > 0 {
		acrosses = append(acrosses, [2]float32{g.X, g.W})
	}
	for k, a := range t.in.apart {
		switch r := t.apart[k]; {
		case a.rows && r.H > 0:
			downs = append(downs, [2]float32{r.Y, r.H})
		case !a.rows && r.W > 0:
			r = t.across(r)
			acrosses = append(acrosses, [2]float32{r.X, r.W})
		}
	}
	var parts []ui.Rect
	for _, ys := range tableSpans(box.Y, box.H, downs) {
		for _, xs := range tableSpans(box.X, box.W, acrosses) {
			parts = append(parts, ui.Rect{X: xs[0], Y: ys[0], W: xs[1] - xs[0], H: ys[1] - ys[0]})
		}
	}
	return parts
}

// tableSpans are the stretches from start over size that gaps leave, each
// gap where it opens and how far, and each stretch a start and an end:
// the whole, until a gap is open enough to part it.
func tableSpans(start, size float32, gaps [][2]float32) [][2]float32 {
	spans := [][2]float32{{start, start + size}}
	for _, gap := range gaps {
		at, open := gap[0], gap[1]
		if open < 1 {
			continue
		}
		var parted [][2]float32
		for _, span := range spans {
			if at+open <= span[0] || at >= span[1] {
				parted = append(parted, span)
				continue
			}
			if at-span[0] >= 1 {
				parted = append(parted, [2]float32{span[0], at})
			}
			if span[1]-(at+open) >= 1 {
				parted = append(parted, [2]float32{at + open, span[1]})
			}
		}
		spans = parted
	}
	return spans
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
// past reach, each at that floor. Each column counts as far as it has
// come in (share, 1 where nil), and is set at that share of its width.
func columnWidths(shape []profile.Column, width, reach float32, natural, least []float32, read float32, share []float32) []float32 {
	widths := make([]float32, len(natural))
	shareOf := func(i int) float32 {
		if i < len(share) {
			return share[i]
		}
		return 1
	}
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
			floors += floor[i] * shareOf(i)
			fits += want[i] * shareOf(i)
			continue
		}
		left -= widths[i] * shareOf(i)
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
			level = levelled(open, shareOf, left, want, unbounded)
		case floors < left:
			level = levelled(open, shareOf, left, floor, want)
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
		edge += widths[i] * shareOf(i)
		widths[i] = float32(math.Round(float64(edge))) - from
	}
	return widths
}

// levelled sets the open columns between their bounds low and high at one
// level, as far up as room holds them, each counting as far as it has
// come in.
func levelled(open []bool, share func(int) float32, room float32, low, high []float32) []float32 {
	filled := func(level float32) (sum, slope float32) {
		for i := range low {
			if !open[i] {
				continue
			}
			sum += max(low[i], min(high[i], level)) * share(i)
			if level >= low[i] && level < high[i] {
				slope += share(i)
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
		if cc := t.cells[i]; cc.Row >= 0 {
			s.x, s.y = t.base[i]-x, t.baseY[i]
		}
	}
}

// cell is a cell's box in its unit, as the grid is scrolled.
func (t *tableLayout) cell(cc doc.CellCoord) ui.Rect {
	return ui.Rect{X: t.left - t.scroll + t.columns[cc.Column], Y: t.rows[cc.Row], W: t.widths[cc.Column], H: t.heights[cc.Row]}
}

// line is a row's box across what the table shows, or a column's down
// it, in its unit, as the grid is scrolled.
func (t *tableLayout) line(u *unitLayout, rows bool, i int) ui.Rect {
	if rows {
		return ui.Rect{X: u.box.X, Y: t.rows[i], W: u.box.W, H: t.heights[i]}
	}
	return ui.Rect{X: t.left - t.scroll + t.columns[i], Y: u.box.Y, W: t.widths[i], H: u.box.H}
}

// rowAt and columnAt are the row and the column at a place across the
// unit, -1 past the grid.
func (t *tableLayout) rowAt(y float32) int {
	for r := range t.rows {
		if top := t.rows[r]; y >= top && y < top+t.heights[r] {
			return r
		}
	}
	return -1
}

func (t *tableLayout) columnAt(x float32) int {
	for c := range t.columns {
		if left := t.left - t.scroll + t.columns[c]; x >= left && x < left+t.widths[c] {
			return c
		}
	}
	return -1
}

// paint draws the grid beneath its cells' text, in each of its parts:
// the header's inset, the lines between the cells, and its edge.
func (tableKind) paint(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	t := u.table
	if t == nil {
		return 0
	}
	for _, part := range t.parts(u.box) {
		part.X += ux
		part.Y += uy
		t.paintGrid(el, p, part, u.frame, ux, uy, true, true)
	}
	return 0
}

// paintGrid draws the grid within frame, its corners rounded by radius and
// the unit drawn at ux, uy: the header's inset, the lines between its rows
// and between its columns as asked, and the frame's edge.
func (t *tableLayout) paintGrid(el *Element, p *ui.Painter, frame ui.Rect, radius, ux, uy float32, rows, columns bool) {
	line := el.Theme.Border.Default
	p.Clip(frame, radius, func() {
		if t.header && len(t.rows) > 0 {
			p.Fill(ui.Rect{X: frame.X, Y: uy + t.rows[0], W: frame.W, H: t.heights[0]}, el.Theme.Surface.Inset, 0)
		}
		for r := range t.rows {
			if at := uy + t.rows[r]; rows && at > frame.Y+0.5 && at < frame.Y+frame.H-0.5 {
				p.Fill(ui.Rect{X: frame.X, Y: at, W: frame.W, H: 1}, line, 0)
			}
		}
		for c := range t.columns {
			if at := ux + t.left - t.scroll + t.columns[c]; columns && at > frame.X+0.5 && at < frame.X+frame.W-0.5 {
				p.Fill(ui.Rect{X: at, Y: frame.Y, W: 1, H: frame.H}, line, 0)
			}
		}
	})
	p.Stroke(frame, line, radius, 1)
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

// settle draws a table scrolled as far as it is, within what it scrolls;
// a paint steps its scroll at its own time.
func (tableKind) settle(el *Element, u *unitLayout, frame bool) bool {
	t := u.table
	if t == nil {
		return false
	}
	ts := recalled[tableScroll](el, u.pres.Unit, "scroll")
	if ts == nil && t.overflow <= 0 {
		t.scrollTo(u, 0)
		return false
	}
	ts = el.tableScroll(u.pres.Unit)
	moving := false
	if frame {
		_, pulling := ts.over.Step(el.paintNow, el.animate)
		var gliding bool
		ts.x, gliding = ts.glide.Step(ts.x, el.paintNow, el.animate)
		moving = pulling || gliding
	}
	ts.x = max(0, min(ts.x, t.overflow))
	t.scrollTo(u, ts.x-ts.over.Offset())
	return moving
}

// tableScroll is how far a table is scrolled, as the page remembers it
// while the table is on the page.
func (el *Element) tableScroll(id doc.ID) *tableScroll {
	return memo(el, id, "scroll", func() *tableScroll { return &tableScroll{} })
}

// wheel scrolls a table wider than its room across by dx, by hand, with
// the pointer over it.
func (tableKind) wheel(el *Element, u *unitLayout, x, y float32, ev ui.InputEvent, dx float32) bool {
	t := u.table
	if t == nil || t.overflow <= 0 || y < u.y || y >= u.bottom() || x < u.x+u.box.X || x >= u.x+u.box.X+u.box.W {
		return false
	}
	ts := el.tableScroll(u.pres.Unit)
	ts.glide.Cancel()
	ts.x = components.OverscrollPanel.Scroll(&ts.over, ts.x, dx, 0, t.overflow, u.box.W, 0, scrollfx.Phase(ev.Precise, ev.Momentum))
	t.scrollTo(u, ts.x-ts.over.Offset())
	return true
}

// reveal glides a table the caret is in across, as far as brings
// the caret's cell into view clear of the fades at its edges (over), or,
// in a cell wider than that, the caret.
func (tableKind) reveal(el *Element, pc place, now time.Time) {
	if pc.surface == nil || pc.unit.table == nil || pc.unit.table.overflow <= 0 {
		return
	}
	u, t, s := pc.unit, pc.unit.table, pc.surface
	if s.index >= len(t.cells) || t.cells[s.index].Row < 0 {
		return
	}
	ts := el.tableScroll(u.pres.Unit)
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

// holdsCell reports whether s is one of the grid's cells.
func (t *tableLayout) holdsCell(s *surfaceLayout) bool {
	return s.index < len(t.cells) && t.cells[s.index].Row >= 0
}
