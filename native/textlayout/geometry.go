package textlayout

import (
	"slices"
	"sort"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

// CheckboxMarker returns the reserved inline object's box in content space.
func (l *Layout) CheckboxMarker(checkbox surface.Checkbox) (ui.Rect, bool) {
	if l == nil {
		return ui.Rect{}, false
	}
	paragraph, local := l.paragraphAt(checkbox.Marker.Start)
	if paragraph == nil {
		return ui.Rect{}, false
	}
	local = paragraph.shapedOffset(local, false)
	for _, object := range paragraph.layout.Objects() {
		if object.Index == local {
			box := object.Rect
			box.X += paragraph.left
			box.Y += paragraph.top
			return box, true
		}
	}
	return ui.Rect{}, false
}

func (l *Layout) Caret(offset model.UTF16Offset) ui.Rect {
	paragraph, local := l.paragraphAt(offset)
	if paragraph == nil {
		lineHeight := l.style.Prose.LineHeight()
		height := l.style.Prose.CaretHeight()
		return ui.Rect{
			Y: l.height - lineHeight + (lineHeight-height)/2 + l.style.Prose.CaretOffsetY(),
			W: 2,
			H: height,
		}
	}
	caret := paragraph.layout.Caret(paragraph.shapedOffset(local, false))
	caret.X += paragraph.left
	caret.Y += paragraph.top
	// The caret grows with its row: a row of larger text is taller than
	// body text's, and its caret is as much taller, centered alike. In a row
	// packed closer than body text it keeps the body's size.
	lineHeight := l.style.Prose.LineHeight()
	row := caret.H
	if row <= 0 {
		row = lineHeight
	}
	scale := max(1, row/lineHeight)
	height := l.style.Prose.CaretHeight() * scale
	caret.Y += (row-height)/2 + l.style.Prose.CaretOffsetY()*scale
	caret.W = 2
	caret.H = height
	return caret
}

// CaretLine returns the full retained line box containing the caret. The
// returned H is the shaped visual-row height, not the drawn caret thickness.
func (l *Layout) CaretLine(offset model.UTF16Offset) (ui.Rect, bool) {
	if l == nil {
		return ui.Rect{}, false
	}
	paragraph, local := l.paragraphAt(offset)
	if paragraph == nil {
		return ui.Rect{}, false
	}
	line := paragraph.layout.Caret(paragraph.shapedOffset(local, false))
	line.X += paragraph.left
	line.Y += paragraph.top
	return line, line.H > 0
}

// RowNear is the offset on the visual row above (direction -1) or below
// (1) the row offset is on, nearest x: its paragraph's next row, or the
// nearest row of the paragraph beside it, whatever room lies between them,
// as a figure's or a closed caption's. Above the first row is that row,
// at x, and below the last the page's end.
func (l *Layout) RowNear(offset model.UTF16Offset, direction int, x float32) model.UTF16Offset {
	paragraph, local := l.paragraphAt(offset)
	if paragraph == nil {
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	row := paragraph.layout.Caret(paragraph.shapedOffset(local, false))
	_, height := paragraph.layout.Size()
	if y := row.Y + row.H/2 + float32(direction)*row.H; y >= 0 && y < height {
		return model.UTF16Offset(paragraph.startUTF16 + paragraph.sourceOffset(indexOnRow(paragraph.layout, x-paragraph.left, y)))
	}
	index := slices.IndexFunc(l.paragraphs, func(candidate Paragraph) bool {
		return candidate.startUTF16 == paragraph.startUTF16
	})
	beside, ok := l.verticalNeighbor(index, direction, x)
	if !ok {
		if direction < 0 {
			return model.UTF16Offset(paragraph.startUTF16 + paragraph.sourceOffset(indexOnRow(paragraph.layout, x-paragraph.left, row.Y+row.H/2)))
		}
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	// Its first row going down, its last coming up.
	_, height = beside.layout.Size()
	y := min(1, height/2)
	if direction < 0 {
		y = height - y
	}
	return model.UTF16Offset(beside.startUTF16 + beside.sourceOffset(indexOnRow(beside.layout, x-beside.left, y)))
}

// verticalNeighbor is the paragraph above (direction -1) or below (1) the
// one at index, nearest x: in a table, the next line of its cell or the
// cell above or below in its column, then the text beside the table; into
// a table, the cell of its first or last row under x.
func (l *Layout) verticalNeighbor(index, direction int, x float32) (*Paragraph, bool) {
	paragraph := &l.paragraphs[index]
	if table, row, column, ok := l.cellOf(paragraph); ok {
		lines := l.cellLines(table, row, column)
		at := slices.Index(lines, paragraph) + direction
		if at >= 0 && at < len(lines) {
			return lines[at], true
		}
		if next := row + direction; next >= 0 && next < len(table.Table.Rows) {
			lines = l.cellLines(table, next, column)
			if direction > 0 {
				return lines[0], true
			}
			return lines[len(lines)-1], true
		}
		// Out of the table, past its first or last cell's lines.
		edge := table.Table.Range().Start
		if direction > 0 {
			edge = table.Table.Range().End
		}
		index = slices.IndexFunc(l.paragraphs, func(candidate Paragraph) bool {
			return model.UTF16Offset(candidate.startUTF16) <= edge && edge <= model.UTF16Offset(candidate.contentUTF16)
		})
		if direction > 0 {
			for index+1 < len(l.paragraphs) && l.paragraphs[index+1].cell {
				index++
			}
		} else {
			for index-1 >= 0 && l.paragraphs[index-1].cell {
				index--
			}
		}
	}
	index += direction
	if index < 0 || index >= len(l.paragraphs) {
		return nil, false
	}
	beside := &l.paragraphs[index]
	if table, _, _, ok := l.cellOf(beside); ok {
		row := 0
		if direction < 0 {
			row = len(table.Table.Rows) - 1
		}
		lines := l.cellLines(table, row, edgeIndex(table.Columns, x))
		if direction > 0 {
			return lines[0], true
		}
		return lines[len(lines)-1], true
	}
	return beside, true
}

// indexOnRow is the index nearest x on a paragraph's row at y. Past the
// end of a row that wraps, the nearest index is where the next row
// starts, which the caret shows there: it steps back onto its own row.
func indexOnRow(layout *ui.TextLayout, x, y float32) int {
	index := layout.IndexAt(ui.Point{X: x, Y: y})
	if index > 0 && layout.Caret(index).Y > y {
		return layout.PreviousBoundary(index)
	}
	return index
}

func (l *Layout) Hit(point ui.Point) model.UTF16Offset {
	if point.Y >= l.height {
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	paragraph := l.paragraphAtY(point.Y)
	if table, ok := l.TableAt(point.Y); ok {
		row, column := table.At(point)
		paragraph = paragraphIn(l.cellLines(table, row, column), point.Y)
	}
	if paragraph == nil {
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	localPoint := ui.Point{X: point.X - paragraph.left, Y: point.Y - paragraph.top}
	local := paragraph.layout.IndexAt(localPoint)
	return model.UTF16Offset(paragraph.startUTF16 + paragraph.sourceOffset(local))
}

func (l *Layout) RangeRects(selection model.UTF16Range) []ui.Rect {
	if selection.Start >= selection.End {
		return nil
	}
	start, end := int(selection.Start), int(selection.End)
	rects := make([]ui.Rect, 0, 4)
	for _, paragraph := range l.paragraphs {
		if end < paragraph.startUTF16 {
			break
		}
		if start > paragraph.endUTF16 {
			continue
		}
		// A caption takes as much of its row as it is open; a closed one
		// shows no selection, its picture showing it instead.
		room := float32(1)
		if paragraph.hasFigure {
			room = captionRoom(paragraph.figure)
		}
		if room == 0 {
			continue
		}
		from := max(start, paragraph.startUTF16)
		to := min(end, paragraph.contentUTF16)
		// A selection through the break before a paragraph takes the gap
		// above it too, so a selection across blocks has no holes.
		lift := func(rect ui.Rect) ui.Rect {
			rect.H *= room
			rect.X += paragraph.left
			if start < paragraph.startUTF16 && rect.Y <= paragraph.top && !paragraph.cell {
				rect.Y -= paragraph.gap
				rect.H += paragraph.gap
			}
			return rect
		}
		// A cell scrolled out of its table's room shows none of it.
		add := func(rect ui.Rect) {
			if clip, ok := paragraph.Clip(); ok {
				left, right := max(rect.X, clip.X), min(rect.X+rect.W, clip.X+clip.W)
				if right <= left {
					return
				}
				rect.X, rect.W = left, right-left
			}
			rects = append(rects, rect)
		}
		if from < to {
			localRects := paragraph.layout.SelectionRects(ui.TextInputRange{
				Start: paragraph.shapedOffset(from-paragraph.startUTF16, false),
				End:   paragraph.shapedOffset(to-paragraph.startUTF16, true),
			})
			for _, rect := range localRects {
				rect.Y += paragraph.top
				add(lift(rect))
			}
		}
		if end > paragraph.contentUTF16 && start <= paragraph.contentUTF16 {
			// The break past a selected paragraph's end shows as a sliver of
			// its last row, as tall as that row: larger text's is taller
			// than body text's, and a shorter sliver would notch the selection.
			caret := paragraph.layout.Caret(paragraph.shapedOffset(paragraph.contentUTF16-paragraph.startUTF16, false))
			caret.Y += paragraph.top
			caret.W = 6
			add(lift(caret))
		}
	}
	sort.SliceStable(rects, func(i, j int) bool {
		if rects[i].Y != rects[j].Y {
			return rects[i].Y < rects[j].Y
		}
		return rects[i].X < rects[j].X
	})
	return rects
}

func (l *Layout) BoundsForRange(selection model.UTF16Range) (ui.Rect, bool) {
	rects := l.RangeRects(selection)
	if len(rects) > 0 {
		return rects[0], true
	}
	if selection.Start == selection.End {
		return l.Caret(selection.Start), true
	}
	return ui.Rect{}, false
}

// CaretAtPoint returns the projected caret rect for an IME candidate anchor.
// Hit currently narrows the shaped position to an index, so a bidi or soft-wrap
// boundary's visual affinity cannot yet be preserved here.
func (l *Layout) CaretAtPoint(point ui.Point) ui.Rect {
	return l.Caret(l.Hit(point))
}

// VisibleBounds returns the content rectangle currently visible in a
// viewport of the given height at scrollY.
func (l *Layout) VisibleBounds(scrollY, height float32) ui.Rect {
	if l == nil {
		return ui.Rect{}
	}
	top := max(0, scrollY)
	bottom := min(l.height, scrollY+max(0, height))
	if bottom <= top {
		return ui.Rect{Y: top, H: 0}
	}
	return ui.Rect{Y: top, H: bottom - top}
}

func (l *Layout) PreviousBoundary(offset model.UTF16Offset) model.UTF16Offset {
	paragraph, local := l.paragraphAt(offset)
	if paragraph == nil {
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	if local > 0 {
		before := paragraph.layout.PreviousBoundary(paragraph.shapedOffset(local, true))
		return model.UTF16Offset(paragraph.startUTF16 + paragraph.sourceOffset(before))
	}
	if paragraph.startUTF16 == 0 {
		return 0
	}
	return model.UTF16Offset(paragraph.startUTF16 - 1)
}

func (l *Layout) NextBoundary(offset model.UTF16Offset) model.UTF16Offset {
	paragraph, local := l.paragraphAt(offset)
	if paragraph == nil {
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	if local < paragraph.contentUTF16-paragraph.startUTF16 {
		after := paragraph.layout.NextBoundary(paragraph.shapedOffset(local, false))
		return model.UTF16Offset(paragraph.startUTF16 + paragraph.sourceOffset(after))
	}
	if paragraph.endUTF16 > paragraph.contentUTF16 {
		return model.UTF16Offset(paragraph.contentUTF16 + 1)
	}
	return model.UTF16Offset(paragraph.endUTF16)
}

// paragraphAt resolves an offset to its owning paragraph. An interior
// paragraph owns [start, end), where end follows its newline, so a paragraph
// start belongs to the following paragraph. The final paragraph has no
// newline and also owns its end: the document end is a caret position.
func (l *Layout) paragraphAt(offset model.UTF16Offset) (*Paragraph, int) {
	value := int(offset)
	if len(l.paragraphs) == 0 {
		return nil, 0
	}
	index := sort.Search(len(l.paragraphs), func(index int) bool {
		return l.paragraphs[index].endUTF16 > value
	})
	if index == len(l.paragraphs) {
		if value != l.paragraphs[index-1].endUTF16 {
			return nil, 0
		}
		index--
	}
	paragraph := &l.paragraphs[index]
	if value < paragraph.startUTF16 {
		return paragraph, 0
	}
	local := min(value-paragraph.startUTF16, paragraph.contentUTF16-paragraph.startUTF16)
	return paragraph, local
}

func (l *Layout) paragraphAtY(y float32) *Paragraph {
	if len(l.paragraphs) == 0 {
		return nil
	}
	if y <= 0 {
		return &l.paragraphs[0]
	}
	index := sort.Search(len(l.paragraphs), func(index int) bool {
		return l.paragraphs[index].top+l.paragraphs[index].height > y
	})
	if index == len(l.paragraphs) {
		return nil
	}
	return &l.paragraphs[index]
}
