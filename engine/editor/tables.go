package editor

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// A table is a run of cell blocks, row after row, each cell a block of its
// own: typing, marks, history and input work in a cell as in any text. The
// page stays flat; a table is only where its cells begin a row. The first
// cell of a table starts it, the first of each other row starts that row,
// and the cells of the first row keep their column's alignment and width,
// the first cell whether the first row heads the table. No cell keeps its
// row or column, so no two facts about a table can disagree; the editor
// keeps every row as long as the others (writeTable).
//
// A cell's notation is nothing: the grid is drawn, not written. What is
// copied out of a table is its cells between tabs and its rows between
// lines, as a spreadsheet reads it.
const (
	// CellNode is one cell of a table; its text is what the cell holds.
	CellNode model.NodeTypeID = "cell"

	cellStartsAttribute = "starts"
	cellStartsTable     = "table"
	cellStartsRow       = "row"
	// cellAlignAttribute and cellWidthAttribute are a column's, on its cell
	// in the first row: "center" or "end", and the share of the table's
	// width it takes in thousandths.
	cellAlignAttribute = "align"
	cellWidthAttribute = "width"
	// tablePlainAttribute, on a table's first cell, says its first row is
	// a row like the others, not its header.
	tablePlainAttribute = "plain"

	tablesSetting = "tables"
	tableTrigger  = "writing:table"

	// newTableColumns and newTableRows are the size Insert Table makes, a
	// header and two rows, to grow from where it shows.
	newTableColumns = 3
	newTableRows    = 3
	// leastColumnWidth is the narrowest a column is set, as a share.
	leastColumnWidth = 0.05
)

// cellAttributes are the attributes only a cell has.
var cellAttributes = []string{cellStartsAttribute, cellAlignAttribute, cellWidthAttribute, tablePlainAttribute}

// Table is a table on a page: its cells row by row, each row as long as
// the others once written, and its columns' shape.
type Table struct {
	Rows   [][]model.NodeID
	Header bool
	Align  []surface.CellAlign
	// Widths are each column's share of the table's width, 0 for an even
	// share of what the others leave.
	Widths []float32
}

// Columns is how many cells the longest row has.
func (t Table) Columns() int {
	columns := 0
	for _, row := range t.Rows {
		columns = max(columns, len(row))
	}
	return columns
}

// First and Last are the table's first and last cells.
func (t Table) First() model.NodeID { return t.Rows[0][0] }
func (t Table) Last() model.NodeID {
	row := t.Rows[len(t.Rows)-1]
	return row[len(row)-1]
}

// Find is where node stands in the table.
func (t Table) Find(node model.NodeID) (row, column int, ok bool) {
	for r, cells := range t.Rows {
		if c := slices.Index(cells, node); c >= 0 {
			return r, c, true
		}
	}
	return 0, 0, false
}

// Cells are the table's cells in reading order.
func (t Table) Cells() []model.NodeID {
	cells := make([]model.NodeID, 0, len(t.Rows)*t.Columns())
	for _, row := range t.Rows {
		cells = append(cells, row...)
	}
	return cells
}

// clone is a copy of t that changes nothing of t's.
func (t Table) clone() Table {
	next := t
	next.Rows = make([][]model.NodeID, len(t.Rows))
	for index, row := range t.Rows {
		next.Rows[index] = slices.Clone(row)
	}
	next.Align, next.Widths = slices.Clone(t.Align), slices.Clone(t.Widths)
	return next
}

// blockReader reads a page, its snapshot or a draft of it.
type blockReader interface {
	Root() model.NodeID
	Node(model.NodeID) (*model.Node, bool)
}

func textAttribute(node *model.Node, key string) string {
	value, _ := node.Attribute(key)
	text, _ := value.Text()
	return text
}

// readTables are the tables on a page, in order.
func readTables(document blockReader) []Table {
	root, ok := document.Node(document.Root())
	if !ok {
		return nil
	}
	var tables []Table
	open := false
	for _, id := range root.Children() {
		node, ok := document.Node(id)
		if !ok || node.Kind() != CellNode {
			open = false
			continue
		}
		starts := textAttribute(node, cellStartsAttribute)
		if !open || starts == cellStartsTable {
			plain, _ := node.Attribute(tablePlainAttribute)
			isPlain, _ := plain.Bool()
			tables = append(tables, Table{Header: !isPlain})
			open, starts = true, cellStartsRow
		}
		table := &tables[len(tables)-1]
		if starts == cellStartsRow {
			table.Rows = append(table.Rows, nil)
		}
		table.Rows[len(table.Rows)-1] = append(table.Rows[len(table.Rows)-1], id)
	}
	for index := range tables {
		table := &tables[index]
		columns := table.Columns()
		table.Align, table.Widths = make([]surface.CellAlign, columns), make([]float32, columns)
		for column, id := range table.Rows[0] {
			node, _ := document.Node(id)
			switch textAttribute(node, cellAlignAttribute) {
			case "center":
				table.Align[column] = surface.CellCenter
			case "end":
				table.Align[column] = surface.CellEnd
			}
			if value, ok := node.Attribute(cellWidthAttribute); ok {
				if width, ok := value.Integer(); ok {
					table.Widths[column] = float32(max(0, min(1000, width))) / 1000
				}
			}
		}
	}
	return tables
}

// tableAround is the table node is a cell of.
func tableAround(document blockReader, node model.NodeID) (Table, bool) {
	for _, table := range readTables(document) {
		if _, _, ok := table.Find(node); ok {
			return table, true
		}
	}
	return Table{}, false
}

// TableOf is the table node is a cell of, on the page.
func TableOf(document model.DocumentSnapshot, node model.NodeID) (Table, bool) {
	return tableAround(document, node)
}

// blockIndex is node's place among the page's blocks.
func blockIndex(document blockReader, node model.NodeID) int {
	root, _ := document.Node(document.Root())
	return slices.Index(root.Children(), node)
}

// keepAttribute sets key on node when keep, and takes it away otherwise.
func keepAttribute(document *model.DocumentDraft, node model.NodeID, key string, keep bool, value model.AttributeValue) error {
	current, ok := document.Node(node)
	if !ok {
		return &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: node}
	}
	if _, has := current.Attribute(key); keep {
		return document.SetAttribute(node, key, value)
	} else if has {
		return document.RemoveAttribute(node, key)
	}
	return nil
}

// newCell makes an empty cell at index among the page's blocks.
func newCell(document *model.DocumentDraft, index int, text string) (model.NodeID, error) {
	id, err := document.CreateNode(document.Root(), index, CellNode)
	if err == nil {
		err = document.SetTextValue(id, text)
	}
	return id, err
}

// writeTable gives the cells of t, standing in the page in its order, the
// attributes that make them t: where each row starts, and the columns'
// shape and the header on the first row. A short row is made as long as
// the others with empty cells.
func writeTable(document *model.DocumentDraft, t Table) error {
	columns := t.Columns()
	for len(t.Align) < columns {
		t.Align = append(t.Align, surface.CellStart)
	}
	for len(t.Widths) < columns {
		t.Widths = append(t.Widths, 0)
	}
	for r := range t.Rows {
		for len(t.Rows[r]) < columns {
			id, err := newCell(document, blockIndex(document, t.Rows[r][len(t.Rows[r])-1])+1, "")
			if err != nil {
				return err
			}
			t.Rows[r] = append(t.Rows[r], id)
		}
		for c, id := range t.Rows[r] {
			starts := ""
			switch {
			case r == 0 && c == 0:
				starts = cellStartsTable
			case c == 0:
				starts = cellStartsRow
			}
			align := ""
			switch t.Align[c] {
			case surface.CellCenter:
				align = "center"
			case surface.CellEnd:
				align = "end"
			}
			width := int64(max(0, min(1, t.Widths[c])) * 1000)
			if err := keepAttribute(document, id, cellStartsAttribute, starts != "", model.TextValue(starts)); err != nil {
				return err
			}
			if err := keepAttribute(document, id, tablePlainAttribute, r == 0 && c == 0 && !t.Header, model.BoolValue(true)); err != nil {
				return err
			}
			if err := keepAttribute(document, id, cellAlignAttribute, r == 0 && align != "", model.TextValue(align)); err != nil {
				return err
			}
			if err := keepAttribute(document, id, cellWidthAttribute, r == 0 && width > 0, model.IntegerValue(width)); err != nil {
				return err
			}
		}
	}
	return nil
}

// makeTable writes a table of rows, each the texts of its cells, at index
// among the page's blocks, and returns it. A block reuse, at index, becomes
// its first cell, keeping its identity for the caret in it.
func makeTable(document *model.DocumentDraft, index int, rows [][]string, header bool, reuse model.NodeID) (Table, error) {
	table := Table{Header: header}
	for r, texts := range rows {
		row := make([]model.NodeID, 0, len(texts))
		for c, text := range texts {
			if r == 0 && c == 0 && reuse != 0 {
				if err := shapeBlock(document, reuse, blockShape{kind: CellNode}); err != nil {
					return Table{}, err
				}
				current, _ := document.Node(reuse)
				if err := document.ReplaceTextMarked(reuse, 0, blockLength(current), text, nil); err != nil {
					return Table{}, err
				}
				index++
				row = append(row, reuse)
				continue
			}
			id, err := newCell(document, index, text)
			if err != nil {
				return Table{}, err
			}
			index++
			row = append(row, id)
		}
		table.Rows = append(table.Rows, row)
	}
	return table, writeTable(document, table)
}

// emptyRows are rows by columns of empty cells.
func emptyRows(rows, columns int) [][]string {
	texts := make([][]string, rows)
	for index := range texts {
		texts[index] = make([]string, columns)
	}
	return texts
}

// insertRows makes count empty rows before row at, at len(Rows) after
// the last, and returns the table they leave.
func insertRows(document *model.DocumentDraft, t Table, at, count int) (Table, error) {
	t = t.clone()
	index := blockIndex(document, t.Last()) + 1
	if at < len(t.Rows) {
		index = blockIndex(document, t.Rows[at][0])
	}
	rows := make([][]model.NodeID, count)
	for r := range rows {
		for range t.Columns() {
			id, err := newCell(document, index, "")
			if err != nil {
				return Table{}, err
			}
			index++
			rows[r] = append(rows[r], id)
		}
	}
	t.Rows = slices.Insert(t.Rows, at, rows...)
	return t, writeTable(document, t)
}

// insertColumns makes count empty columns before column at, at Columns()
// after the last.
func insertColumns(document *model.DocumentDraft, t Table, at, count int) (Table, error) {
	t = t.clone()
	for r, row := range t.Rows {
		index := blockIndex(document, row[len(row)-1]) + 1
		if at < len(row) {
			index = blockIndex(document, row[at])
		}
		cells := make([]model.NodeID, count)
		for c := range cells {
			id, err := newCell(document, index+c, "")
			if err != nil {
				return Table{}, err
			}
			cells[c] = id
		}
		t.Rows[r] = slices.Insert(row, at, cells...)
	}
	t.Align = slices.Insert(t.Align, at, make([]surface.CellAlign, count)...)
	t.Widths = slices.Insert(t.Widths, at, make([]float32, count)...)
	return t, writeTable(document, t)
}

// removeRows takes the rows from first to last away, both included.
func removeRows(document *model.DocumentDraft, t Table, first, last int) (Table, error) {
	t = t.clone()
	for _, row := range t.Rows[first : last+1] {
		for _, id := range row {
			if err := document.RemoveNode(id); err != nil {
				return Table{}, err
			}
		}
	}
	t.Rows = slices.Delete(t.Rows, first, last+1)
	return t, writeTable(document, t)
}

// removeColumns takes the columns from first to last away, both included.
func removeColumns(document *model.DocumentDraft, t Table, first, last int) (Table, error) {
	t = t.clone()
	for r, row := range t.Rows {
		for _, id := range row[first : last+1] {
			if err := document.RemoveNode(id); err != nil {
				return Table{}, err
			}
		}
		t.Rows[r] = slices.Delete(row, first, last+1)
	}
	t.Align = slices.Delete(t.Align, first, last+1)
	t.Widths = slices.Delete(t.Widths, first, last+1)
	return t, writeTable(document, t)
}

// removeTable takes the table away, leaving an empty paragraph in its
// first cell's place, which keeps its identity for the caret.
func removeTable(document *model.DocumentDraft, t Table) error {
	for _, id := range t.Cells()[1:] {
		if err := document.RemoveNode(id); err != nil {
			return err
		}
	}
	first := t.First()
	if err := shapeBlock(document, first, blockShape{kind: ParagraphNode}); err != nil {
		return err
	}
	current, _ := document.Node(first)
	return document.ReplaceTextMarked(first, 0, blockLength(current), "", nil)
}

// moveRow moves row from to stand at to, the rows between making room.
func moveRow(document *model.DocumentDraft, t Table, from, to int) (Table, error) {
	t = t.clone()
	row := t.Rows[from]
	t.Rows = slices.Insert(slices.Delete(t.Rows, from, from+1), to, row)
	return t, arrange(document, t)
}

// moveColumn moves column from to stand at to.
func moveColumn(document *model.DocumentDraft, t Table, from, to int) (Table, error) {
	t = t.clone()
	for r, row := range t.Rows {
		cell := row[from]
		t.Rows[r] = slices.Insert(slices.Delete(row, from, from+1), to, cell)
	}
	align, width := t.Align[from], t.Widths[from]
	t.Align = slices.Insert(slices.Delete(t.Align, from, from+1), to, align)
	t.Widths = slices.Insert(slices.Delete(t.Widths, from, from+1), to, width)
	return t, arrange(document, t)
}

// arrange stands t's cells in the page in its order, where its first
// cell's block was, and writes it.
func arrange(document *model.DocumentDraft, t Table) error {
	start := len(draftBlocks(document))
	for _, id := range t.Cells() {
		start = min(start, blockIndex(document, id))
	}
	for offset, id := range t.Cells() {
		if err := document.MoveNode(id, start+offset); err != nil {
			return err
		}
	}
	return writeTable(document, t)
}

// draftBlocks is a draft page's block identities, in order.
func draftBlocks(document *model.DocumentDraft) []model.NodeID {
	root, _ := document.Node(document.Root())
	return root.Children()
}

// fillCells writes texts into the table from row, column on, the rows and
// columns it needs made first, and returns the table.
func fillCells(document *model.DocumentDraft, t Table, row, column int, texts [][]string) (Table, error) {
	columns := 0
	for _, line := range texts {
		columns = max(columns, len(line))
	}
	var err error
	if more := row + len(texts) - len(t.Rows); more > 0 {
		if t, err = insertRows(document, t, len(t.Rows), more); err != nil {
			return Table{}, err
		}
	}
	if more := column + columns - t.Columns(); more > 0 {
		if t, err = insertColumns(document, t, t.Columns(), more); err != nil {
			return Table{}, err
		}
	}
	for r, line := range texts {
		for c, text := range line {
			id := t.Rows[row+r][column+c]
			current, _ := document.Node(id)
			if err := document.ReplaceTextMarked(id, 0, blockLength(current), text, nil); err != nil {
				return Table{}, err
			}
		}
	}
	return t, nil
}

// clearCells empties cells.
func clearCells(document *model.DocumentDraft, cells []model.NodeID) (bool, error) {
	changed := false
	for _, id := range cells {
		current, ok := document.Node(id)
		if !ok || blockLength(current) == 0 {
			continue
		}
		if err := document.ReplaceTextMarked(id, 0, blockLength(current), "", nil); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

// emptyRow reports whether every cell of row holds nothing.
func emptyRow(document blockReader, row []model.NodeID) bool {
	for _, id := range row {
		if node, ok := document.Node(id); ok && blockLength(node) > 0 {
			return false
		}
	}
	return true
}

// parseTableRow reads a paragraph written as a table's row, "| a | b |",
// as the texts of its cells.
func parseTableRow(text string) ([]string, bool) {
	text = strings.TrimSpace(text)
	if len(text) < 2 || !strings.HasPrefix(text, "|") || !strings.HasSuffix(text, "|") {
		return nil, false
	}
	parts := strings.Split(text[1:len(text)-1], "|")
	cells := make([]string, len(parts))
	for index, part := range parts {
		cells[index] = strings.TrimSpace(part)
	}
	return cells, true
}

// ParseTable reads text as a table's rows when it is one: lines of cells
// between tabs, as a spreadsheet copies them, or a Markdown table, its
// delimiter row left out. A single line without a tab is no table.
func ParseTable(text string) ([][]string, bool) {
	text = strings.TrimRight(model.NormalizeLineEndings(text), "\n")
	lines := strings.Split(text, "\n")
	var rows [][]string
	if cells, ok := parseTableRow(lines[0]); ok && len(lines) > 1 {
		for _, line := range lines {
			cells, ok = parseTableRow(line)
			if !ok {
				return nil, false
			}
			if strings.Trim(strings.Join(cells, ""), "-: ") == "" {
				continue
			}
			rows = append(rows, cells)
		}
		return rows, len(rows) > 0
	}
	if !strings.Contains(text, "\t") {
		return nil, false
	}
	for _, line := range lines {
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows, true
}

// cellBreak is Return in a cell: the caret goes to the cell below, a row
// made for it below the last. On a last row that holds nothing, Return
// leaves the table: the row goes, and the caret goes on writing below.
func (i *EditorInvoker) cellBreak(document *model.DocumentDraft, node model.NodeID) (kernel.Outcome, error) {
	table, ok := tableAround(document, node)
	if !ok {
		return kernel.OutcomePass, nil
	}
	fail := func(err error) (kernel.Outcome, error) {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	row, column, _ := table.Find(node)
	if row+1 < len(table.Rows) {
		below := table.Rows[row+1][column]
		current, _ := document.Node(below)
		i.placeHead(below, blockLength(current), document)
		return kernel.OutcomeHandled, nil
	}
	if len(table.Rows) > 1 && emptyRow(document, table.Rows[row]) {
		left, err := removeRows(document, table, row, row)
		if err != nil {
			return fail(err)
		}
		next := blockIndex(document, left.Last()) + 1
		if blocks := draftBlocks(document); next < len(blocks) {
			if after, _ := document.Node(blocks[next]); after.Kind() == ParagraphNode && blockLength(after) == 0 {
				i.placeHead(after.ID(), 0, document)
				return kernel.OutcomeHandled, nil
			}
		}
		return i.paragraphAfter(document, left.Last())
	}
	grown, err := insertRows(document, table, len(table.Rows), 1)
	if err != nil {
		return fail(err)
	}
	i.placeHead(grown.Rows[len(grown.Rows)-1][column], 0, document)
	return kernel.OutcomeHandled, nil
}

// openTable is Return after a paragraph written as a table's row: the
// paragraph becomes the table's header, and the caret goes to the first
// cell of an empty row below it.
func (i *EditorInvoker) openTable(document *model.DocumentDraft, node *model.Node) (kernel.Outcome, bool, error) {
	cells, ok := parseTableRow(node.Text().String())
	if !ok {
		return 0, false, nil
	}
	fail := func(err error) (kernel.Outcome, bool, error) {
		return 0, true, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	index := blockIndex(document, node.ID())
	table, err := makeTable(document, index, [][]string{cells, make([]string, len(cells))}, true, node.ID())
	if err != nil {
		return fail(err)
	}
	if err := i.writingRoom(document, table); err != nil {
		return fail(err)
	}
	i.placeHead(table.Rows[1][0], 0, document)
	return kernel.OutcomeHandled, true, nil
}

// writingRoom opens a paragraph after a table that ends the page, so the
// page goes on below it.
func (i *EditorInvoker) writingRoom(document *model.DocumentDraft, table Table) error {
	if blockIndex(document, table.Last())+1 < len(draftBlocks(document)) {
		return nil
	}
	_, err := newParagraph(document, blockIndex(document, table.Last())+1)
	return err
}

func newParagraph(document *model.DocumentDraft, index int) (model.NodeID, error) {
	id, err := document.CreateNode(document.Root(), index, ParagraphNode)
	if err == nil {
		err = document.SetTextValue(id, "")
	}
	return id, err
}

// tableSpan replaces a span with a table among its blocks, as joinSpan
// does elsewhere, but no cell joins another block nor goes alone: a cell
// the span covers empties, and a table it covers whole, reaching past it,
// goes. A block wholly between the ends goes as it would. The text goes
// in at the span's start, or where the start's table was. Backspace at a
// cell's start, the span only the break before it, goes to the end of the
// cell before; at the start of a block after a table, it takes the block
// only when it is empty.
func (i *EditorInvoker) tableSpan(
	document *model.DocumentDraft,
	first model.NodeID,
	start model.UTF16Offset,
	last model.NodeID,
	end model.UTF16Offset,
	text string,
) (kernel.Outcome, error) {
	fail := func(err error) (kernel.Outcome, error) {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	order := draftBlocks(document)
	from, to := slices.Index(order, first), slices.Index(order, last)
	head, _ := document.Node(first)
	tail, _ := document.Node(last)
	// The span is only the break between two blocks.
	if to == from+1 && start == blockLength(head) && end == 0 && text == "" {
		switch {
		case head.Kind() != CellNode && blockLength(head) == 0:
			if err := document.RemoveNode(first); err != nil {
				return fail(err)
			}
			i.placeHead(last, 0, document)
		case tail.Kind() != CellNode && blockLength(tail) == 0:
			if err := document.RemoveNode(last); err != nil {
				return fail(err)
			}
			i.placeHead(first, start, document)
		default:
			// Backspace and Delete alike step across the break; which way
			// is the caret's, and the session's guard (guardTable) handles
			// the delete that would merge, so here the caret goes to the
			// start of the cell or block after.
			i.placeHead(last, 0, document)
		}
		return kernel.OutcomeHandled, nil
	}
	whole := map[model.NodeID]bool{}
	for _, table := range readTables(document) {
		lo, hi := blockIndex(document, table.First()), blockIndex(document, table.Last())
		lastNode, _ := document.Node(table.Last())
		startsBefore := from < lo || from == lo && start == 0
		endsAfter := to > hi || to == hi && end == blockLength(lastNode)
		if startsBefore && endsAfter && hi >= from && lo <= to {
			for _, id := range table.Cells() {
				whole[id] = true
			}
		}
	}
	// With only whole tables in it between two other blocks, the span
	// joins them as any span does, once the tables are gone.
	if head.Kind() != CellNode && tail.Kind() != CellNode {
		for _, id := range order[from+1 : to] {
			if whole[id] {
				if err := document.RemoveNode(id); err != nil {
					return fail(err)
				}
			}
		}
		return i.joinSpan(document, first, start, last, end, text)
	}
	target, at := first, start
	if whole[first] {
		target, at = last, 0
		if whole[last] {
			// The span is a whole table and nothing more: it leaves a
			// paragraph in its place.
			table, _ := tableAround(document, first)
			if err := removeTable(document, table); err != nil {
				return fail(err)
			}
			if err := document.ReplaceTextMarked(first, 0, 0, text, nil); err != nil {
				return fail(err)
			}
			i.placeHead(first, model.UTF16Offset(model.StringLenUTF16(text)), document)
			return kernel.OutcomeHandled, nil
		}
	}
	for index := to; index >= from; index-- {
		id := order[index]
		node, _ := document.Node(id)
		lo, hi := model.UTF16Offset(0), blockLength(node)
		if index == from {
			lo = start
		}
		if index == to {
			hi = end
		}
		switch {
		case whole[id] || index != from && index != to && node.Kind() != CellNode:
			err := document.RemoveNode(id)
			if err != nil {
				return fail(err)
			}
		default:
			if err := document.ReplaceTextMarked(id, lo, hi, "", nil); err != nil {
				return fail(err)
			}
		}
	}
	if err := document.ReplaceTextMarked(target, at, at, text, nil); err != nil {
		return fail(err)
	}
	i.placeHead(target, at+model.UTF16Offset(model.StringLenUTF16(text)), document)
	return kernel.OutcomeHandled, nil
}

// spansTable reports whether a block from first to last is a cell.
func spansTable(document *model.DocumentDraft, first, last model.NodeID) bool {
	order := draftBlocks(document)
	from, to := slices.Index(order, first), slices.Index(order, last)
	if from < 0 || to < from {
		return false
	}
	for _, id := range order[from : to+1] {
		if node, ok := document.Node(id); ok && node.Kind() == CellNode {
			return true
		}
	}
	return false
}
