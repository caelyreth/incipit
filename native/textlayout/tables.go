package textlayout

import (
	"math"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

// A table is as wide as its cells' text asks, each column as wide as its
// longest line up to a share of the column, where longer text wraps. One
// that asks for less takes the column's width, what is over shared evenly
// among its columns; one that asks for more reaches past the column into
// the margins, as far each side, and one wider still shows as much as the
// margins hold, scrolled across within it. Each row is as tall as its
// tallest cell, a cell's lines stacked in it inside a little room of its
// own. Set apart from the text around it as a figure is, a table reads as
// one thing on the page.
const (
	// cellPadX and cellPadY are a cell's room around its text, as shares
	// of the body's line.
	cellPadX = 0.4
	cellPadY = 0.2
	// leastColumn is the narrowest a column is set, in DIPs, so a table of
	// many columns still shows each; widestColumn the widest a column's
	// text sets it, as a share of the column, before it wraps.
	leastColumn  = 32
	widestColumn = 0.6
	// headerWeight sets a header's text, as the app's own headings are.
	headerWeight = 600
	// unwrapped is the width text is set in to read its longest line.
	unwrapped = 1 << 16
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

// columnWidths sets each of a table's columns as wide as natural asks, or
// its share of width where it has one, and shares what the column has over
// evenly among those without; each as far as it has come in.
func columnWidths(table surface.Table, width float32, natural []float32) []float32 {
	widths := make([]float32, len(natural))
	total, open := float32(0), float32(0)
	for index := range widths {
		if index < len(table.Widths) && table.Widths[index] > 0 {
			widths[index] = table.Widths[index] * width
		} else {
			widths[index] = natural[index]
			open += table.Room.ColumnShare(index)
		}
		total += widths[index]
	}
	// Each column's edges are rounded, not its width, so the columns
	// together are as wide as they are before rounding.
	over := max(0, width-total)
	edge := float32(0)
	for index := range widths {
		share := table.Room.ColumnShare(index)
		if (index >= len(table.Widths) || table.Widths[index] <= 0) && open > 0 {
			widths[index] += over / open
		}
		from := float32(math.Round(float64(edge)))
		edge += widths[index] * share
		widths[index] = float32(math.Round(float64(edge))) - from
	}
	return widths
}

// placeTable sets table out from top, in paragraphs from its first cell's
// first line on, shaping each line with shape. It returns the table, its
// paragraphs, and its height, the room below it included.
func placeTable(
	table surface.Table,
	paragraphs []rawParagraph,
	top, width float32,
	style Style,
	shape func(raw rawParagraph, width float32, align ui.Align, header bool) *ui.TextLayout,
) (PlacedTable, []Paragraph, float32) {
	line := style.Prose.LineHeight()
	padX := float32(math.Round(float64(line * cellPadX)))
	padY := float32(math.Round(float64(line * cellPadY)))
	room := table.Room
	// Each cell's lines, and the width its longest asks.
	end := table.Range().End
	cells := make([][][]rawParagraph, len(table.Rows))
	natural := make([]float32, table.Columns())
	for index := range natural {
		natural[index] = leastColumn
	}
	next := 0
	for r, row := range table.Rows {
		cells[r] = make([][]rawParagraph, len(row))
		for c, cell := range row {
			for next < len(paragraphs) && model.UTF16Offset(paragraphs[next].startUTF16) <= cell.Range.End {
				raw := paragraphs[next]
				cells[r][c] = append(cells[r][c], raw)
				next++
				longest := float32(0)
				for _, set := range shape(raw, unwrapped, ui.Start, table.Header && r == 0).Lines() {
					longest = max(longest, set.Rect.W)
				}
				natural[c] = max(natural[c], min(float32(math.Ceil(float64(longest)))+2*padX, width*widestColumn))
				if model.UTF16Offset(raw.endUTF16) > end {
					break
				}
			}
		}
	}
	columnRoom := min(room.ColumnRoom, width/2)
	widths := columnWidths(table, width-columnRoom, natural)
	grid := columnRoom
	for _, w := range widths {
		grid += w
	}
	// Past the column, as far each side; past the margins, scrolled.
	shows := min(grid, width+2*style.Spread)
	left := min(0, (width-shows)/2)
	overflow := max(0, grid-shows)
	scroll := max(0, min(room.Scroll, overflow))
	placed := PlacedTable{Table: table, Widths: widths, Scroll: scroll, Overflow: overflow}
	x := left - scroll
	for index, w := range widths {
		if index == room.Column && columnRoom > 0 {
			placed.ColumnGap = ui.Rect{X: x, Y: top, W: columnRoom}
			x += columnRoom
		}
		placed.Columns = append(placed.Columns, x)
		x += w
	}
	if room.Column >= len(widths) && columnRoom > 0 {
		placed.ColumnGap = ui.Rect{X: x, Y: top, W: columnRoom}
	}
	var clip ui.Rect
	var shaped []Paragraph
	y := top
	for r, row := range table.Rows {
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
	if room.Row >= len(table.Rows) && room.RowRoom > 0 {
		placed.RowGap = ui.Rect{X: left, Y: y, W: shows, H: room.RowRoom}
		y += room.RowRoom
	}
	placed.Box = ui.Rect{X: left, Y: top, W: shows, H: y - top}
	placed.Grid = ui.Rect{X: left - scroll, Y: top, W: grid, H: y - top}
	placed.ColumnGap.H = placed.Box.H
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
