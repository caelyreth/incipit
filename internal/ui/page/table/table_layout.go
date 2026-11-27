package table

import (
	"math"
	"slices"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// Kind draws a table: its cells in a grid.

// margins: a table wider than the column shows into the margins.
func (Kind) Margins() bool { return true }

func (Kind) Lay(lc Layouter, u *Unit, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, tin In) bool {
	g, ok := vocab.GridNode(profile.Schema(), p.Node)
	if !ok {
		return false
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
	line := m.Line()
	padX := float32(math.Round(float64(line * cellPadX)))
	padY := float32(math.Round(float64(line * cellPadY)))
	t := &Layout{header: shape.Header, cells: make([]doc.CellCoord, len(p.Surfaces)),
		base: make([]float32, len(p.Surfaces)), baseY: make([]float32, len(p.Surfaces)),
		rows: make([]float32, rows), heights: make([]float32, rows), in: tin}
	if tin.width > 0 && tin.sized < len(shape.Columns) {
		// A column's width as its border's Drag sets it.
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
		_, wide := lc.LayCell(i, s, p, comp, th, m, ui.TextShape{}, false, false)
		_, narrow := lc.LayCell(i, s, p, comp, th, m, ui.TextShape{Width: 1, NoBreakWords: true}, false, false)
		// A line asks at most the room the table may show in, in whole
		// DIPs, so set at what it asks it keeps to its line.
		natural[cc.Column] = max(natural[cc.Column], min(ceil(longestLine(wide.Layout))+2*padX, width+2*m.Reach))
		least[cc.Column] = max(least[cc.Column], min(ceil(longestLine(narrow.Layout))+2*padX, width))
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
		for _, g := range tin.lines.goneAfter(1, c-1) {
			apart += g.room
		}
	}
	shares := make([]float32, columns)
	for c := range shares {
		shares[c] = tin.share(c)
	}
	t.widths = columnWidths(shape.Columns, width-columnRoom-apart, 2*m.Reach, natural, least, line*readColumn, shares)
	grid := float32(0)
	for c := range columns + 1 {
		for _, g := range tin.lines.goneAfter(1, c-1) {
			t.gone[1] = append(t.gone[1], goneRoom{room: ui.Rect{X: grid, W: g.room}, kept: g.kept, alpha: g.alpha})
			grid += g.room
		}
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
			t.columns = append(t.columns, grid+tin.lines.offsetOf(1, c))
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
			sl, _ := lc.LayCell(i, s, p, comp, th, m, ui.TextShape{Width: width}, true, false)
			u.Surfaces = append(u.Surfaces, sl)
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
		sl, _ := lc.LayCell(i, s, p, comp, th, m, ui.TextShape{Width: max(1, t.widths[cc.Column]-2*padX), Align: align}, true, true)
		// A cell shows always: the grid is its room, held or not.
		t.heights[cc.Row] = max(t.heights[cc.Row], sl.H+2*padY)
		u.Surfaces = append(u.Surfaces, sl)
	}
	// Down the grid, each row as far as it has come in, and the rooms
	// between them.
	t.natural = slices.Clone(t.heights)
	y := float32(0)
	for r := range rows + 1 {
		for _, g := range tin.lines.goneAfter(0, r-1) {
			t.gone[0] = append(t.gone[0], goneRoom{room: ui.Rect{X: t.left, Y: y, W: shows, H: g.room}, kept: g.kept, alpha: g.alpha})
			y += g.room
		}
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
			if e, ok := tin.lines.extentOf(0, r); ok {
				t.heights[r] = e
			}
			t.rows[r] = y + tin.lines.offsetOf(0, r)
			y += t.heights[r]
		}
	}
	for i, sl := range u.Surfaces {
		if cc := t.cells[i]; cc.Row >= 0 {
			share := float32(1)
			if n := t.natural[cc.Row]; n > 0 {
				share = min(t.heights[cc.Row]/n, 1)
			}
			t.base[i] = t.left + t.columns[cc.Column] + padX
			t.baseY[i] = t.rows[cc.Row] + padY*share
			sl.X, sl.Y = t.base[i], t.baseY[i]
			u.Surfaces[i] = sl
			if sl.SetPos != nil {
				sl.SetPos(sl.X, sl.Y)
			}
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
	for k := range t.gone[1] {
		t.gone[1][k].room.X += t.left
		t.gone[1][k].room.H = y
	}
	u.Table = t
	u.H = y
	u.Box = ui.Rect{X: t.left, W: shows, H: y}
	if tin.below > 0 {
		// Under the table, the room of an area that makes a row.
		t.below = ui.Rect{X: t.left, Y: y, W: shows, H: tin.below}
		u.H += tin.below
	}
	return true
}

// across is a room across the grid as it is scrolled.
func (t *Layout) across(r ui.Rect) ui.Rect {
	r.X -= t.scroll
	return r
}

// parts are the frames the grid is drawn in, in the unit: its box, parted
// by each room open enough within it. A line that left closes within the
// frame, so the frame and its controls close with it.
func (t *Layout) parts(box ui.Rect) []ui.Rect {
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
// the whole, until a gap is open enough to Part it.
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
func (t *Layout) scrollTo(u *Unit, x float32) {
	t.scroll = x
	for i, s := range u.Surfaces {
		if cc := t.cells[i]; cc.Row >= 0 {
			s.X, s.Y = t.base[i]-x, t.baseY[i]
		}
	}
}
