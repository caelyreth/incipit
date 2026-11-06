package surface

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

// replacement is one checked UTF-16 range edit over a flow. All metadata
// positions are mapped in UTF-16 units from the same edit, so runs, blocks,
// objects, and tables cannot drift relative to the replacement text.
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
// the inserted text. Range endpoints are clamped to the replacement. Blocks
// an edit joins are one block after it; each object says how an edit moves
// it, and table shape is kept only when the resulting flow still satisfies
// the WithTables contract.
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
	edit := next.edit()
	flow, err := NewFlow(f.version, text, runs, edit.blocks(f.blocks))
	if err != nil {
		return nil, err
	}
	// An object the edit moves stays as long as it still stands where it
	// may, as a marker joined into the block before it does not.
	previous := model.UTF16Offset(-1)
	for _, object := range f.objects {
		if shifted, ok := object.shifted(edit); ok && flow.admits(shifted, previous) == nil {
			flow.objects = append(flow.objects, shifted)
			previous = shifted.Run().End
		}
	}
	flow.tables = edit.tables(flow, f.tables)
	return flow, nil
}

// edit is how the replacement maps the flow's positions.
func (r replacement) edit() rangeEdit {
	return rangeEdit{original: r.original, replaced: model.UTF16Offset(model.StringLenUTF16(r.text))}
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

// whole is r moved whole past an edit beside it, or false for an edit
// within it.
func (e rangeEdit) whole(r model.UTF16Range) (model.UTF16Range, bool) {
	if e.original.Start < r.End && r.Start < e.original.End ||
		e.original.Empty() && r.Start < e.original.Start && e.original.Start < r.End {
		return r, false
	}
	from, ok := e.point(r.Start)
	if !ok {
		return r, false
	}
	return model.NewUTF16Range(from, from+model.UTF16Offset(r.Len())), true
}

// blocks are the blocks after the edit: each stretched over what is typed
// within or at its ends, and those an edit joins one, the first's.
func (e rangeEdit) blocks(blocks []Block) []Block {
	next := make([]Block, 0, len(blocks))
	for index, block := range blocks {
		start, end := e.rangeStart(block.Range.Start), e.rangeEnd(block.Range.End)
		// An edit taking the break before a block joins it to the one
		// before.
		if index > 0 {
			at := blocks[index-1].Range.End
			if e.original.Start <= at && at < e.original.End {
				next[len(next)-1].Range.End = end
				continue
			}
		}
		block.Range = model.NewUTF16Range(start, max(start, end))
		next = append(next, block)
	}
	return next
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
