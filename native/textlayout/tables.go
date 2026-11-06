package textlayout

import (
	"math"
	"slices"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

// A table keeps to the column as a paragraph does, as a page's tables do
// on the web. One whose text fits on a line in each cell takes the
// column's width, what is over shared evenly among its columns. One that
// asks for more wraps: a short column keeps its line, and the long ones
// share the rest. Every column keeps the least it reads well in, its
// longest word or a few words' width, even empty, and a column set to fit
// its text keeps its line whatever it asks. A table whose columns ask for
// more than the column reaches past it into the margins as far as they
// ask, as far each side, and one wider still shows as much as the margins
// hold, scrolled across within it. Each row is as tall as its tallest
// cell, a cell's lines stacked in it inside a little room of its own. Set
// apart from the text around it as a figure is, a table reads as one
// thing on the page.
const (
	// cellPadX and cellPadY are a cell's room around its text, as shares
	// of the body's line.
	cellPadX = 0.4
	cellPadY = 0.2
	// tableSteps sets a table apart from the blocks around it, in block
	// steps: wider than a figure's room, its grid standing on the text's
	// edge with no margin of its own, yet short of a line, so an empty
	// line by it still reads as one.
	tableSteps = 1.75
	// LeastColumn is the narrowest a column is set, in DIPs, so a table of
	// many columns still shows each; readColumn the least a column that
	// wraps keeps, in the body's lines, its longest word if wider.
	LeastColumn = 32
	readColumn  = 4
	// headerWeight sets a header's text, as the app's own headings are.
	headerWeight = 600
	// unwrapped is the width text is set in to read its longest line, and
	// narrowest the one to read its longest word: as narrow as it goes,
	// its words kept whole.
	unwrapped = 1 << 16
	narrowest = 0
)

// PlacedTable is a table as the layout sets it out.
type PlacedTable struct {
	Table surface.Table
	// Box is the room the grid shows in, and Below the room open under
	// it; the grid itself is Grid, as wide as its columns, scrolled across
	// Box by Scroll when it is wider, as far as Overflow.
	Box, Below, Grid ui.Rect
	Scroll, Overflow float32
	// Columns and Rows are each column's left edge and each row's top, and
	// Widths and Heights their sizes; RowGap and ColumnGap are the room
	// open between them.
	Columns, Widths, Rows, Heights []float32
	RowGap, ColumnGap              ui.Rect
	// RowsApart and ColumnsApart are the room a drag opens before each
	// row and column, the last after them all; empty where it opens none.
	RowsApart, ColumnsApart []ui.Rect
	// Natural is each column's width as its text asks on one line, at
	// most the room the table may show in, before any share of the room
	// over is spread to it.
	Natural []float32
}

// Cell is the box of the cell at row and column.
func (t PlacedTable) Cell(row, column int) ui.Rect {
	return ui.Rect{X: t.Columns[column], Y: t.Rows[row], W: t.Widths[column], H: t.Heights[row]}
}

// At is the row and column of the cell at point, the nearest one when it
// is outside.
func (t PlacedTable) At(point ui.Point) (int, int) {
	return edgeIndex(t.Rows, point.Y), edgeIndex(t.Columns, point.X)
}

// edgeIndex is the last of starts at or before at.
func edgeIndex(starts []float32, at float32) int {
	index := 0
	for next := 1; next < len(starts); next++ {
		if at >= starts[next] {
			index = next
		}
	}
	return index
}

// Tables are the tables the layout sets out, in order.
func (l *Layout) Tables() []PlacedTable { return append([]PlacedTable(nil), l.tables...) }

// TableAt is the table whose box holds y.
func (l *Layout) TableAt(y float32) (*PlacedTable, bool) {
	for index := range l.tables {
		if box := l.tables[index].Box; y >= box.Y && y < box.Y+box.H {
			return &l.tables[index], true
		}
	}
	return nil, false
}

// columnWidths sets a table's columns. One that fits its text is as wide
// as its longest line. The others wrap theirs as the table has room, each
// keeping a floor, its longest word or read, whichever is wider, so a new
// or empty column is never thinner than one reads in. Where all their
// text fits on a line, they are even but for those whose text asks for
// more, as wide as it asks: typing in a cell moves no border until its
// line outgrows its share. Where it does not, those whose text asks for
// less keep their line, and the rest share what is left evenly, none under
// its floor; the two meet where the text just fits. Where the floors ask
// for more than width, the table takes what its text asks past it, as far
// as reach, and past that is wider still. Each as far as it has come in.
func columnWidths(table surface.Table, room TableRoom, width, reach float32, natural, least []float32, read float32) []float32 {
	widths := make([]float32, len(natural))
	fit := func(index int) bool { return index < len(table.Fit) && table.Fit[index] }
	floor, want := make([]float32, len(widths)), make([]float32, len(widths))
	left, open, floors, fits := width, float32(0), float32(0), float32(0)
	for index := range widths {
		share := room.ColumnShare(index)
		if fit(index) {
			widths[index] = natural[index]
			left -= natural[index] * share
			continue
		}
		floor[index] = max(least[index], read)
		want[index] = max(floor[index], natural[index])
		floors += floor[index] * share
		fits += want[index] * share
		open += share
	}
	if open > 0 {
		if floors > left {
			// Out of the column, the table may as well spare its text
			// a wrap, as far as reach.
			left = min(max(floors, fits), left+reach)
		}
		unbounded := make([]float32, len(widths))
		for index := range unbounded {
			unbounded[index] = float32(math.Inf(1))
		}
		var level []float32
		switch {
		case fits <= left:
			level = levelled(room, fit, left, want, unbounded)
		case floors < left:
			level = levelled(room, fit, left, floor, want)
		default:
			level = floor
		}
		for index := range widths {
			if !fit(index) {
				widths[index] = level[index]
			}
		}
	}
	// Each column's edges are rounded, not its width, so the columns
	// together are as wide as they are before rounding.
	edge := float32(0)
	for index := range widths {
		from := float32(math.Round(float64(edge)))
		edge += widths[index] * room.ColumnShare(index)
		widths[index] = float32(math.Round(float64(edge))) - from
	}
	return widths
}

// levelled sets the columns not fitted as one level, each kept between its
// low and high, the level such that they fill room as far as each has
// come in, as shown says.
func levelled(shown TableRoom, fit func(int) bool, room float32, low, high []float32) []float32 {
	filled := func(level float32) (sum, slope float32) {
		for index := range low {
			if fit(index) {
				continue
			}
			share := shown.ColumnShare(index)
			sum += max(low[index], min(high[index], level)) * share
			if level >= low[index] && level < high[index] {
				slope += share
			}
		}
		return sum, slope
	}
	// The sum rises in a straight line between the columns' bounds: find
	// the stretch room falls in, and the level along it.
	bounds := make([]float32, 0, 2*len(low))
	for index := range low {
		if !fit(index) {
			bounds = append(bounds, low[index])
			if !math.IsInf(float64(high[index]), 1) {
				bounds = append(bounds, high[index])
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
	for index := range widths {
		widths[index] = max(low[index], min(high[index], level))
	}
	return widths
}

// longestLine is how wide the longest of text's lines is, to the pixel.
func longestLine(text *ui.TextLayout) float32 {
	longest := float32(0)
	for _, set := range text.Lines() {
		longest = max(longest, set.Rect.W)
	}
	return float32(math.Ceil(float64(longest)))
}

func tableGap(body float32) float32 {
	return blockGap(body, tableSteps)
}

// placeTable sets table out from top, as room opens it, in paragraphs from its first cell's
// first line on, shaping each line with shape. It returns the table, its
// paragraphs, and its height, the room below it included.
func placeTable(
	table surface.Table,
	room TableRoom,
	paragraphs []rawParagraph,
	top, width float32,
	style Style,
	shape func(raw rawParagraph, width float32, align ui.Align, header bool) *ui.TextLayout,
) (PlacedTable, []Paragraph, float32) {
	line := style.Prose.LineHeight()
	padX := float32(math.Round(float64(line * cellPadX)))
	padY := float32(math.Round(float64(line * cellPadY)))
	// Each cell's lines, and the width its longest asks.
	end := table.Range().End
	cells := make([][][]rawParagraph, len(table.Rows))
	natural, least := make([]float32, table.Columns()), make([]float32, table.Columns())
	for index := range natural {
		natural[index], least[index] = LeastColumn, LeastColumn
	}
	next := 0
	for r, row := range table.Rows {
		cells[r] = make([][]rawParagraph, len(row))
		for c, cell := range row {
			for next < len(paragraphs) && model.UTF16Offset(paragraphs[next].startUTF16) <= cell.Range.End {
				raw := paragraphs[next]
				cells[r][c] = append(cells[r][c], raw)
				next++
				header := table.Header && r == 0
				// A line asks at most the room the table may show in.
				natural[c] = max(natural[c], min(longestLine(shape(raw, unwrapped, ui.Start, header))+2*padX, width+2*style.Spread))
				least[c] = max(least[c], min(longestLine(shape(raw, narrowest, ui.Start, header))+2*padX, width))
				if model.UTF16Offset(raw.endUTF16) > end {
					break
				}
			}
		}
	}
	columnRoom := min(room.ColumnRoom, width/2)
	apart := float32(0)
	for column := range table.Columns() + 1 {
		apart += room.ColumnApart(column)
	}
	// Room a drag opens between columns is taken from the others, as the
	// room an area opens between them is.
	widths := columnWidths(table, room, width-columnRoom-apart, 2*style.Spread, natural, least, line*readColumn)
	grid := columnRoom + apart
	for _, w := range widths {
		grid += w
	}
	// Past the column, as far each side; past the margins, scrolled.
	shows := min(grid, width+2*style.Spread)
	left := min(0, (width-shows)/2)
	overflow := max(0, grid-shows)
	scroll := max(0, min(room.Scroll, overflow)) + room.Pull
	placed := PlacedTable{
		Table: table, Widths: widths, Natural: natural, Scroll: scroll, Overflow: overflow,
		RowsApart:    make([]ui.Rect, len(table.Rows)+1),
		ColumnsApart: make([]ui.Rect, len(widths)+1),
	}
	x := left - scroll
	for index := range len(widths) + 1 {
		if a := room.ColumnApart(index); a > 0 {
			placed.ColumnsApart[index] = ui.Rect{X: x, Y: top, W: a}
			x += a
		}
		if index == room.Column && columnRoom > 0 {
			placed.ColumnGap = ui.Rect{X: x, Y: top, W: columnRoom}
			x += columnRoom
		}
		if index < len(widths) {
			placed.Columns = append(placed.Columns, x)
			x += widths[index]
		}
	}
	var clip ui.Rect
	var shaped []Paragraph
	y := top
	for r, row := range table.Rows {
		if a := room.RowApart(r); a > 0 {
			placed.RowsApart[r] = ui.Rect{X: left, Y: y, W: shows, H: a}
			y += a
		}
		if r == room.Row && room.RowRoom > 0 {
			placed.RowGap = ui.Rect{X: left, Y: y, W: shows, H: room.RowRoom}
			y += room.RowRoom
		}
		share := room.RowShare(r)
		rowHeight := float32(0)
		for c := range row {
			align := ui.Start
			switch table.Align[c] {
			case surface.CellCenter:
				align = ui.Center
			case surface.CellEnd:
				align = ui.End
			}
			inner := max(1, widths[c]-2*padX)
			cellTop := padY
			for _, raw := range cells[r][c] {
				layout := shape(raw, inner, align, table.Header && r == 0)
				_, height := layout.Size()
				shaped = append(shaped, Paragraph{
					layout:       layout,
					block:        raw.block,
					startByte:    raw.startByte,
					endByte:      raw.endByte,
					startUTF16:   raw.startUTF16,
					endUTF16:     raw.endUTF16,
					contentUTF16: raw.contentUTF16,
					top:          y + cellTop*share,
					height:       height * share,
					left:         placed.Columns[c] + padX,
					cell:         true,
				})
				cellTop += height
			}
			rowHeight = max(rowHeight, cellTop+padY)
		}
		rowHeight *= share
		placed.Rows = append(placed.Rows, y)
		placed.Heights = append(placed.Heights, rowHeight)
		y += rowHeight
	}
	if a := room.RowApart(len(table.Rows)); a > 0 {
		placed.RowsApart[len(table.Rows)] = ui.Rect{X: left, Y: y, W: shows, H: a}
		y += a
	}
	if room.Row >= len(table.Rows) && room.RowRoom > 0 {
		placed.RowGap = ui.Rect{X: left, Y: y, W: shows, H: room.RowRoom}
		y += room.RowRoom
	}
	placed.Box = ui.Rect{X: left, Y: top, W: shows, H: y - top}
	placed.Grid = ui.Rect{X: left - scroll, Y: top, W: grid, H: y - top}
	placed.ColumnGap.H = placed.Box.H
	for index := range placed.ColumnsApart {
		if placed.ColumnsApart[index].W > 0 {
			placed.ColumnsApart[index].H = placed.Box.H
		}
	}
	placed.Below = ui.Rect{X: left, Y: y, W: shows, H: room.Below}
	if overflow > 0 {
		clip = placed.Box
		for index := range shaped {
			shaped[index].clip = clip
		}
	}
	return placed, shaped, y - top + room.Below
}

// headerSpans sets a header row's text as heavy as the app's headings.
func headerSpans(spans []ui.Span) []ui.Span {
	heavy := make([]ui.Span, len(spans))
	for index, span := range spans {
		span.Weight = max(span.Weight, headerWeight)
		heavy[index] = span
	}
	return heavy
}

// cellOf is the table and cell paragraph stands in, as its row and column.
func (l *Layout) cellOf(paragraph *Paragraph) (*PlacedTable, int, int, bool) {
	if !paragraph.cell {
		return nil, 0, 0, false
	}
	for index := range l.tables {
		table := &l.tables[index]
		if row, column, ok := table.Table.CellAt(model.UTF16Offset(paragraph.startUTF16)); ok {
			return table, row, column, true
		}
	}
	return nil, 0, 0, false
}

// cellLines are the paragraphs of the cell at row and column of table.
func (l *Layout) cellLines(table *PlacedTable, row, column int) []*Paragraph {
	cell := table.Table.Rows[row][column].Range
	var lines []*Paragraph
	for index := range l.paragraphs {
		paragraph := &l.paragraphs[index]
		if model.UTF16Offset(paragraph.startUTF16) > cell.End {
			break
		}
		if model.UTF16Offset(paragraph.startUTF16) >= cell.Start {
			lines = append(lines, paragraph)
		}
	}
	return lines
}

// paragraphIn is the paragraph of the cell nearest y.
func paragraphIn(lines []*Paragraph, y float32) *Paragraph {
	for _, line := range lines {
		if y < line.top+line.height {
			return line
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return lines[len(lines)-1]
}
