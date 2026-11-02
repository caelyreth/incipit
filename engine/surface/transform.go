package surface

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/engine/model"
)

// replacement is one checked UTF-16 range edit over a flow. All metadata
// positions are mapped in UTF-16 units from the same edit, so runs, figures,
// checkboxes, tables, and rooms cannot drift relative to the replacement text.
type replacement struct {
	original model.UTF16Range
	text     string
	style    TextStyle
}

// replaceRange replaces original with text and remaps the flow's positional
// metadata as one immutable transformation.
//
// A positional anchor strictly inside a replaced range has no surviving
// position and is omitted. A collapsed insertion at an anchor belongs after
// the inserted text. Range endpoints are clamped to the replacement, while
// table/figure shape is kept only when the resulting flow still satisfies the
// existing WithTables/WithFigures contracts.
//
// Tables are remapped when the edit is outside the table or wholly within one
// cell. A cross-cell edit is not projected as a table: semantic composition
// cannot cross a cell boundary, and extending one cell's node range across
// another would invent semantics the editor does not have. The preview text
// is still shown; only its no-longer-representable structural metadata drops.
func (f *Flow) replaceRange(next replacement) (*Flow, error) {
	if f == nil {
		return nil, fmt.Errorf("surface replacement requires a flow")
	}
	bytes, err := f.byteRange(next.original)
	if err != nil {
		return nil, err
	}
	if !model.ValidDocumentText(next.text) {
		return nil, fmt.Errorf("invalid replacement text")
	}

	text := f.text[:int(bytes.Start)] + next.text + f.text[int(bytes.End):]
	runs := replacementRuns(f.runs, bytes, next.text, next.style)
	flow, err := NewFlow(f.version, text, runs)
	if err != nil {
		return nil, err
	}

	edit := rangeEdit{
		original: next.original,
		replaced: model.UTF16Offset(model.StringLenUTF16(next.text)),
	}
	flow.figures = edit.figures(flow, f.figures)
	flow.checkboxes = edit.checkboxes(flow, f.checkboxes)
	flow.tables = edit.tables(flow, f.tables)
	flow.rooms = edit.rooms(flow, f.rooms)
	return flow, nil
}

func (e rangeEdit) checkboxes(flow *Flow, checkboxes []Checkbox) []Checkbox {
	next := make([]Checkbox, 0, len(checkboxes))
	previousEnd := model.ByteOffset(0)
	for _, checkbox := range checkboxes {
		r := checkbox.Marker
		if e.original.Empty() {
			if r.Start < e.original.Start && e.original.Start < r.End {
				continue
			}
		} else if e.original.Start < r.End && r.Start < e.original.End {
			continue
		}
		start, startOK := e.point(r.Start)
		end, endOK := e.point(r.End)
		if !startOK || !endOK {
			continue
		}
		checkbox.Marker = model.NewUTF16Range(start, end)
		bytes, err := flow.byteRange(checkbox.Marker)
		if err != nil || len(next) > 0 && !strings.Contains(flow.text[previousEnd:bytes.Start], "\n") {
			continue
		}
		next = append(next, checkbox)
		previousEnd = bytes.End
	}
	return next
}

func replacementRuns(
	runs []StyleRun,
	original model.ByteRange,
	text string,
	style TextStyle,
) []StyleRun {
	next := make([]StyleRun, 0, len(runs)+1)
	for _, run := range runs {
		end := min(int(run.Bytes.End), int(original.Start))
		if int(run.Bytes.Start) < end {
			next = append(next, StyleRun{
				Bytes: model.NewByteRange(run.Bytes.Start, model.ByteOffset(end)),
				Style: run.Style,
			})
		}
	}
	if text != "" {
		next = append(next, StyleRun{
			Bytes: model.NewByteRange(
				original.Start,
				original.Start+model.ByteOffset(len(text)),
			),
			Style: style,
		})
	}
	shift := len(text) - original.Len()
	for _, run := range runs {
		start := max(int(run.Bytes.Start), int(original.End))
		if start >= int(run.Bytes.End) {
			continue
		}
		next = append(next, StyleRun{
			Bytes: model.NewByteRange(
				model.ByteOffset(start+shift),
				model.ByteOffset(int(run.Bytes.End)+shift),
			),
			Style: run.Style,
		})
	}
	return next
}

// rangeEdit maps old UTF-16 positions to the flow after one replacement.
// Cell endpoints include text inserted at either boundary, including an empty
// cell's sole caret position. Cells are separated by generated line breaks, so
// one cell owns each such position.
type rangeEdit struct {
	original model.UTF16Range
	replaced model.UTF16Offset
}

func (e rangeEdit) delta() model.UTF16Offset {
	return e.replaced - model.UTF16Offset(e.original.Len())
}

func (e rangeEdit) newEnd() model.UTF16Offset {
	return e.original.Start + e.replaced
}

func (e rangeEdit) point(at model.UTF16Offset) (model.UTF16Offset, bool) {
	switch {
	case at < e.original.Start:
		return at, true
	case at >= e.original.End:
		return at + e.delta(), true
	default:
		return 0, false
	}
}

func (e rangeEdit) rangeStart(at model.UTF16Offset) model.UTF16Offset {
	switch {
	case at <= e.original.Start:
		return at
	case at >= e.original.End:
		return at + e.delta()
	default:
		return e.original.Start
	}
}

func (e rangeEdit) rangeEnd(at model.UTF16Offset) model.UTF16Offset {
	switch {
	case at < e.original.Start:
		return at
	case at == e.original.Start && !e.original.Empty():
		return at
	case at >= e.original.End:
		return at + e.delta()
	default:
		return e.newEnd()
	}
}

func (e rangeEdit) figures(flow *Flow, figures []Figure) []Figure {
	next := make([]Figure, 0, len(figures))
	for _, figure := range figures {
		at, ok := e.point(figure.At)
		if !ok || at != 0 && !flow.lineStart(at) {
			continue
		}
		figure.At = at
		next = append(next, figure)
	}
	return next
}

func (e rangeEdit) tables(flow *Flow, tables []Table) []Table {
	next := make([]Table, 0, len(tables))
	for _, table := range tables {
		if !e.tableRepresentable(table) {
			continue
		}
		remapped := table
		remapped.Rows = make([][]TableCell, len(table.Rows))
		for row, cells := range table.Rows {
			remapped.Rows[row] = make([]TableCell, len(cells))
			for column, cell := range cells {
				remapped.Rows[row][column] = TableCell{
					Node: cell.Node,
					Range: model.NewUTF16Range(
						e.rangeStart(cell.Range.Start),
						e.rangeEnd(cell.Range.End),
					),
				}
			}
		}
		if validateTables(flow, []Table{remapped}) != nil {
			continue
		}
		next = append(next, remapped)
	}
	return next
}

func (e rangeEdit) tableRepresentable(table Table) bool {
	r := table.Range()
	if e.original.End <= r.Start || e.original.Start >= r.End {
		return true
	}
	if e.original.Empty() {
		return r.Start <= e.original.Start && e.original.Start <= r.End
	}
	for _, row := range table.Rows {
		for _, cell := range row {
			if cell.Range.Start <= e.original.Start && e.original.End <= cell.Range.End {
				return true
			}
		}
	}
	return false
}

func (e rangeEdit) rooms(flow *Flow, rooms []Room) []Room {
	next := make([]Room, 0, len(rooms))
	for _, room := range rooms {
		if room.At == -1 {
			next = append(next, room)
			continue
		}
		at, ok := e.point(room.At)
		if !ok || at != 0 && !flow.lineStart(at) {
			continue
		}
		room.At = at
		next = append(next, room)
	}
	return next
}
