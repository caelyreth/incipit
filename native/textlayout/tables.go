package textlayout

import (
	"math"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

// A table takes the column's width and sets its cells out in a grid: each
// column as wide as its share, or an even part of what the shared ones
// leave, each row as tall as its tallest cell, a cell's lines stacked in it
// inside a little room of its own. Set apart from the text around it as a
// figure is, a table reads as one thing on the page.
const (
	// cellPadX and cellPadY are a cell's room around its text, as shares
	// of the body's line.
	cellPadX = 0.4
	cellPadY = 0.2
	// leastColumn is the narrowest a column is set, in DIPs, so a table of
	// many columns still shows each.
	leastColumn = 32
	// headerWeight sets a header's text, as the app's own headings are.
	headerWeight = 600
)

// PlacedTable is a table as the layout sets it out.
type PlacedTable struct {
	Table surface.Table
	// Box is the grid, and Below the room open under it.
	Box, Below ui.Rect
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

// columnWidths shares width among a table's columns: one with a share
// takes it, and the rest share what is left evenly; each as far as it has
// come in.
func columnWidths(table surface.Table, width float32) []float32 {
	columns := table.Columns()
	widths := make([]float32, columns)
	shared, open := float32(0), float32(0)
	for index, share := range table.Widths {
		if share > 0 {
			widths[index] = share * width
			shared += widths[index]
		} else {
			open += table.Room.ColumnShare(index)
		}
	}
	left := max(0, width-shared)
	for index := range widths {
		if widths[index] == 0 && open > 0 {
			widths[index] = left / open
		}
	}
	// Shares above the whole shrink together to fit.
	total := float32(0)
	for _, w := range widths {
		total += w
	}
	if total > width && total > 0 {
		for index := range widths {
			widths[index] *= width / total
		}
	}
	for index := range widths {
		share := table.Room.ColumnShare(index)
		widths[index] = float32(math.Round(float64(max(leastColumn, widths[index]) * share)))
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
	columnRoom := min(room.ColumnRoom, width/2)
	widths := columnWidths(table, width-columnRoom)
	placed := PlacedTable{Table: table, Widths: widths}
	x := float32(0)
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
		x += columnRoom
	}
	end := table.Range().End
	var shaped []Paragraph
	next := 0
	y := top
	for r, row := range table.Rows {
		if r == room.Row && room.RowRoom > 0 {
			placed.RowGap = ui.Rect{Y: y, W: x, H: room.RowRoom}
			y += room.RowRoom
		}
		share := room.RowShare(r)
		rowHeight := float32(0)
		for c, cell := range row {
			align := ui.Start
			switch table.Align[c] {
			case surface.CellCenter:
				align = ui.Center
			case surface.CellEnd:
				align = ui.End
			}
			inner := max(1, widths[c]-2*padX)
			cellTop := padY
			for next < len(paragraphs) && model.UTF16Offset(paragraphs[next].startUTF16) <= cell.Range.End {
				raw := paragraphs[next]
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
				next++
				if model.UTF16Offset(raw.endUTF16) > end {
					break
				}
			}
			rowHeight = max(rowHeight, cellTop+padY)
		}
		rowHeight *= share
		placed.Rows = append(placed.Rows, y)
		placed.Heights = append(placed.Heights, rowHeight)
		y += rowHeight
	}
	if room.Row >= len(table.Rows) && room.RowRoom > 0 {
		placed.RowGap = ui.Rect{Y: y, W: x, H: room.RowRoom}
		y += room.RowRoom
	}
	placed.Box = ui.Rect{Y: top, W: x, H: y - top}
	placed.ColumnGap.H = placed.Box.H
	placed.Below = ui.Rect{Y: y, W: x, H: room.Below}
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
