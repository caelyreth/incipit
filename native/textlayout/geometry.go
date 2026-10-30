package textlayout

import (
	"sort"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/egoist/mygo/ui"
)

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
	caret := paragraph.layout.Caret(local)
	caret.Y += paragraph.top
	// The caret grows with its row: a heading's row is taller than body
	// text's, and its caret is as much taller, centered alike.
	lineHeight := l.style.Prose.LineHeight()
	row := max(lineHeight, caret.H)
	scale := row / lineHeight
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
	line := paragraph.layout.Caret(local)
	line.Y += paragraph.top
	return line, line.H > 0
}

func (l *Layout) Hit(point ui.Point) model.UTF16Offset {
	if point.Y >= l.height {
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	paragraph := l.paragraphAtY(point.Y)
	if paragraph == nil {
		return model.UTF16Offset(l.flow.UTF16Len())
	}
	localPoint := ui.Point{X: point.X, Y: point.Y - paragraph.top}
	local := paragraph.layout.IndexAt(localPoint)
	return model.UTF16Offset(paragraph.startUTF16 + local)
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
		from := max(start, paragraph.startUTF16)
		to := min(end, paragraph.contentUTF16)
		if from < to {
			localRects := paragraph.layout.SelectionRects(ui.TextInputRange{
				Start: from - paragraph.startUTF16,
				End:   to - paragraph.startUTF16,
			})
			for _, rect := range localRects {
				rect.Y += paragraph.top
				rects = append(rects, rect)
			}
		}
		if end > paragraph.contentUTF16 && start <= paragraph.contentUTF16 {
			caret := paragraph.layout.Caret(paragraph.contentUTF16 - paragraph.startUTF16)
			caret.Y += paragraph.top
			caret.W = 6
			caret.H = l.style.Prose.LineHeight()
			rects = append(rects, caret)
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
// It preserves the shaped visual affinity selected by IndexAt.
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
		return model.UTF16Offset(paragraph.startUTF16 + paragraph.layout.PreviousBoundary(local))
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
		return model.UTF16Offset(paragraph.startUTF16 + paragraph.layout.NextBoundary(local))
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
