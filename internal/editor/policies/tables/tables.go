// Package tables is the tables policy: tables placed, their rows and
// columns inserted, removed, moved and shaped, and the keys that walk
// their cells. A table is an object and its cells its text, so typing,
// marks and lines in a cell are core's, deleting into or out of a table
// holds it as any whole, and a cell's edge neither merges nor lifts cells
// (vocab.CanJoin, vocab.CanLift). What the tables policy answers is the
// grid: Tab and Shift+Tab step from cell to cell, Return goes down a
// column, and the table commands edit the lines the selection stands in.
package tables

import (
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "tables"

// Policy is the tables policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	// Whole rows selected are offered what acts on rows, whole columns
	// what acts on columns; anything else in a table, both.
	rows, columns := policy.Not(policy.WholeColumns), policy.Not(policy.WholeRows)
	lineOffers, formatOffers, removeOffers := profile.TableLineOffers, profile.TableFormatOffers, profile.TableRemoveOffers
	p.Command("table.insert", "commands.insertTable", profile.InsertTable{Rows: 2, Columns: 3}, policy.Offer(profile.InsertOffers, 1, ""))
	p.Command("table.row-above", "commands.tableRowAbove", profile.EditGrid{Op: profile.GridInsert, Axis: profile.Rows, Before: true},
		policy.Offer(lineOffers, 0, "", rows))
	p.Command("table.row-below", "commands.tableRowBelow", profile.EditGrid{Op: profile.GridInsert, Axis: profile.Rows},
		policy.Offer(lineOffers, 1, "", rows))
	p.Command("table.column-left", "commands.tableColumnLeft", profile.EditGrid{Op: profile.GridInsert, Axis: profile.Columns, Before: true},
		policy.Offer(lineOffers, 2, "", columns))
	p.Command("table.column-right", "commands.tableColumnRight", profile.EditGrid{Op: profile.GridInsert, Axis: profile.Columns},
		policy.Offer(lineOffers, 3, "", columns))
	p.Command("table.delete-row", "commands.tableDeleteRow", profile.EditGrid{Op: profile.GridRemove, Axis: profile.Rows},
		policy.Offer(removeOffers, 0, "", rows))
	p.Command("table.delete-column", "commands.tableDeleteColumn", profile.EditGrid{Op: profile.GridRemove, Axis: profile.Columns},
		policy.Offer(removeOffers, 1, "", columns))
	p.Command("table.delete", "commands.deleteTable", profile.DeleteTable{}, policy.Offer(removeOffers, 2, "", rows, columns))
	p.Command("table.header", "commands.tableHeader", profile.ToggleHeader{}, policy.Offer(formatOffers, 3, "", rows))
	p.Command("table.align-start", "commands.tableAlignStart", profile.AlignColumns{To: profile.AlignStart},
		policy.Offer(formatOffers, 0, "", columns))
	p.Command("table.align-center", "commands.tableAlignCenter", profile.AlignColumns{To: profile.AlignCenter},
		policy.Offer(formatOffers, 1, "", columns))
	p.Command("table.align-end", "commands.tableAlignEnd", profile.AlignColumns{To: profile.AlignEnd},
		policy.Offer(formatOffers, 2, "", columns))

	// What edits a table finds it by the selection, or by the id a
	// gesture names: none of it is a scope's.
	in := policy.Slot.Inside()
	policy.On(p, "insert", policy.Intent[profile.InsertTable](), in, insert).Any()
	policy.On(p, "grid", policy.Intent[profile.EditGrid](), in, grid).Any()
	policy.On(p, "resize", policy.Intent[profile.ResizeTable](), in, resize).Any()
	policy.On(p, "shape-column", policy.Intent[profile.ShapeColumn](), in, shapeColumn).Any()
	policy.On(p, "align", policy.Intent[profile.AlignColumns](), in, align).Any()
	policy.On(p, "header", policy.Intent[profile.ToggleHeader](), in, header).Any()
	policy.On(p, "delete", policy.Intent[profile.DeleteTable](), in, remove).Any()
	// A table pasted in a table fills its cells, as a spreadsheet's do.
	policy.On(p, "fill", policy.Intent[vocab.Paste](), in, fill).Any()

	// A cell range moves by cells: extended, its head goes a cell or a
	// line of cells; not, it collapses into its head cell.
	policy.On(p, "move-cells", policy.Intent[vocab.Move](), in, moveCells).Cells()
	policy.On(p, "visual-cells", policy.Intent[vocab.VisualMove](), in, visualCells).Cells()

	// The keys that walk the grid, in a cell's own text: an inner scope,
	// a formula's source or a pair, is left first.
	cells := policy.Node(profile.Cell).Own()
	policy.On(p, "next-cell", policy.Intent[vocab.Leave](), cells, nextCell)
	policy.On(p, "previous-cell", policy.Intent[vocab.Return](), cells, previousCell)
	policy.On(p, "cell-below", policy.Intent[vocab.Break](), cells, cellBelow)

	// The card's demonstration, played by the engine itself.
	p.Showcase(2600*time.Millisecond,
		policy.Presses(profile.InsertTable{Rows: 2, Columns: 2}),
		policy.Types("Tide"),
		policy.Presses(vocab.Leave{}),
		policy.Types("Height"),
		policy.Waits(400*time.Millisecond),
		policy.Presses(vocab.Break{}),
		policy.Types("High"),
	)
}, policy.Label("policies.tables"), policy.Description("policies.tables.description"))

// span is the lines of a table a selection stands in, both ends included.
type span struct {
	grid       vocab.Grid
	rows, cols [2]int
}

// spanAt is the table the selection stands in and its lines there: a
// cell range's, a selected table's whole, or the caret's cell.
func spanAt(c *policy.Ctx) (span, bool) {
	r := c.Doc()
	sel := c.Selection()
	switch sel.Kind {
	case doc.SelectCells:
		cells := vocab.Cells(r, sel.Cells)
		g, ok := vocab.GridOf(r, sel.Cells.Table)
		if !ok || len(cells) == 0 {
			return span{}, false
		}
		_, first, _ := vocab.Locate(r, cells[0].ID())
		_, last, _ := vocab.Locate(r, cells[len(cells)-1].ID())
		return span{g, [2]int{first.Row, last.Row}, [2]int{first.Column, last.Column}}, true
	case doc.SelectNode:
		g, ok := vocab.GridOf(r, sel.Node)
		if !ok {
			return span{}, false
		}
		rows, cols := g.Size()
		return span{g, [2]int{0, rows - 1}, [2]int{0, cols - 1}}, true
	}
	for id := c.Head().Pos.Node; ; {
		if g, at, ok := vocab.Locate(r, id); ok {
			return span{g, [2]int{at.Row, at.Row}, [2]int{at.Column, at.Column}}, true
		}
		parent, _, ok := r.Parent(id)
		if !ok {
			return span{}, false
		}
		id = parent
	}
}

// tableOf is the table an intent names, or the selection's.
func tableOf(c *policy.Ctx, id doc.ID) (vocab.Grid, bool) {
	if id != 0 {
		return vocab.GridOf(c.Doc(), id)
	}
	s, ok := spanAt(c)
	return s.grid, ok
}

// landIn puts the caret at the end of the text of table's cell at, once
// the edits are made.
func landIn(table doc.ID, at doc.CellCoord) func(doc.Reader) doc.Selection {
	return func(r doc.Reader) doc.Selection {
		g, ok := vocab.GridOf(r, table)
		if !ok {
			return doc.Selection{}
		}
		rows, cols := g.Size()
		at.Row, at.Column = min(max(at.Row, 0), rows-1), min(max(at.Column, 0), cols-1)
		e, _ := vocab.TextEdge(g.Cell(at))
		return doc.Collapsed(e.To)
	}
}

// land is a result that edits and then puts the caret in a cell.
func land(table doc.ID, at doc.CellCoord, edits ...vocab.Edit) policy.Result {
	res := policy.Do(edits...)
	res.Then = landIn(table, at)
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// insert places a table where the selection is, replacing what it covers,
// as a figure is placed; the caret goes to its first cell.
func insert(c *policy.Ctx, in profile.InsertTable, _ policy.At[doc.SlotRef]) policy.Result {
	clear, at, _ := c.Replacing()
	var table doc.ID
	res := policy.Do(func(tx *doc.Tx) error {
		if clear != nil {
			if err := clear(tx); err != nil {
				return err
			}
			at, _ = doc.Steps(tx.Steps()).Map(at, doc.BiasBefore)
		}
		gap, err := vocab.PlaceGap(tx, at, profile.Table)
		if err != nil {
			return err
		}
		if err := tx.Apply(doc.Insert{Parent: gap.Node, Slot: gap.Slot, Index: gap.Offset, Nodes: []*doc.Node{profile.NewTable(in.Rows, in.Columns)}}); err != nil {
			return err
		}
		pn, _ := tx.Node(gap.Node)
		table = pn.Slot(gap.Slot).Child(gap.Offset).ID()
		return nil
	})
	res.Then = func(r doc.Reader) doc.Selection { return landIn(table, doc.CellCoord{})(r) }
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// grid inserts, removes or moves a table's lines; removing them all takes
// the table.
func grid(c *policy.Ctx, in profile.EditGrid, _ policy.At[doc.SlotRef]) policy.Result {
	var s span
	if in.Table != 0 {
		g, ok := vocab.GridOf(c.Doc(), in.Table)
		if !ok {
			return policy.Swallow
		}
		n := max(in.Count, 1)
		s = span{grid: g}
		if in.Axis == profile.Rows {
			s.rows = [2]int{in.At, in.At + n - 1}
		} else {
			s.cols = [2]int{in.At, in.At + n - 1}
		}
	} else {
		var ok bool
		if s, ok = spanAt(c); !ok {
			return policy.Swallow
		}
	}
	id := s.grid.Table.ID()
	rows, cols := s.grid.Size()
	lines, total := s.rows, rows
	if in.Axis == profile.Columns {
		lines, total = s.cols, cols
	}
	at, count := lines[0], lines[1]-lines[0]+1
	if at < 0 || at+count > total {
		return policy.Swallow
	}
	// here is the caret's cell on the other axis, k along this one.
	here := func(k int) doc.CellCoord {
		if in.Axis == profile.Rows {
			return doc.CellCoord{Row: k, Column: s.cols[0]}
		}
		return doc.CellCoord{Row: s.rows[0], Column: k}
	}
	switch in.Op {
	case profile.GridInsert:
		if !in.Before {
			at += count
		}
		if in.Axis == profile.Rows {
			return land(id, here(at), vocab.InsertRows(id, at, count))
		}
		return land(id, here(at), profile.InsertColumns(id, at, count))
	case profile.GridRemove:
		if count == total {
			return removeTable(c, s.grid.Table)
		}
		if in.Axis == profile.Rows {
			return land(id, here(at), vocab.RemoveRows(id, at, at+count))
		}
		return land(id, here(at), profile.RemoveColumns(id, at, at+count))
	case profile.GridMove:
		to := in.To
		if to < 0 || to+count > total || to == at {
			return policy.Swallow
		}
		move := vocab.MoveRow
		if in.Axis == profile.Columns {
			move = profile.MoveColumn
		}
		edits := make([]vocab.Edit, count)
		for k := range edits {
			// Each line in turn, the run keeping its order.
			if to < at {
				edits[k] = move(id, at+k, to+k)
			} else {
				edits[k] = move(id, at, to+count-1)
			}
		}
		// The caret's text moves with its cell.
		return policy.Do(edits...)
	}
	return policy.Swallow
}

// fill writes a pasted table's cells into the selection's table from its
// first cell on, making the rows and columns they need at its end; the
// caret goes to the last cell written. Anything else pasted is core's.
func fill(c *policy.Ctx, in vocab.Paste, _ policy.At[doc.SlotRef]) policy.Result {
	s, ok := spanAt(c)
	if !ok || len(in.Slice.Nodes) != 1 {
		return policy.Pass
	}
	src, ok := vocab.GridNode(c.Doc().Schema(), in.Slice.Nodes[0])
	if !ok || src.Table.Kind() != profile.Table {
		return policy.Pass
	}
	id := s.grid.Table.ID()
	rows, cols := s.grid.Size()
	srows, scols := src.Size()
	at := doc.CellCoord{Row: s.rows[0], Column: s.cols[0]}
	var edits []vocab.Edit
	if more := at.Row + srows - rows; more > 0 {
		edits = append(edits, vocab.InsertRows(id, rows, more))
	}
	if more := at.Column + scols - cols; more > 0 {
		edits = append(edits, profile.InsertColumns(id, cols, more))
	}
	edits = append(edits, vocab.FillCells(id, at, src))
	return land(id, doc.CellCoord{Row: at.Row + srows - 1, Column: at.Column + scols - 1}, edits...)
}

// moveCells moves a cell range's head a cell across, a row for a
// paragraph, or to its row's end for a slot's edge; not extending, the
// range collapses to its head cell's start or end.
func moveCells(c *policy.Ctx, in vocab.Move, _ policy.At[doc.SlotRef]) policy.Result {
	cr := c.Selection().Cells
	if !in.Extend {
		cells := vocab.Cells(c.Doc(), doc.CellRange{Table: cr.Table, Anchor: cr.Head, Head: cr.Head})
		if len(cells) == 0 {
			return policy.Swallow
		}
		e, _ := vocab.TextEdge(cells[0])
		if in.Dir == vocab.Backward {
			return policy.Select(doc.Collapsed(e.From))
		}
		return policy.Select(doc.Collapsed(e.To))
	}
	switch in.Unit {
	case vocab.ByGrapheme, vocab.ByWord:
		return stretch(c, 0, int(in.Dir), false)
	case vocab.ByParagraph:
		return stretch(c, int(in.Dir), 0, false)
	case vocab.ToSlotEdge:
		return stretch(c, 0, int(in.Dir), true)
	}
	return policy.Pass
}

// visualCells moves an extended cell range's head a row up or down, or
// to its row's start or end.
func visualCells(c *policy.Ctx, in vocab.VisualMove, _ policy.At[doc.SlotRef]) policy.Result {
	if !in.Extend {
		return policy.Pass
	}
	switch in.Dir {
	case vocab.Up:
		return stretch(c, -1, 0, false)
	case vocab.Down:
		return stretch(c, 1, 0, false)
	case vocab.LineStart:
		return stretch(c, 0, -1, true)
	case vocab.LineEnd:
		return stretch(c, 0, 1, true)
	}
	return policy.Pass
}

// stretch moves a cell range's head by rows and columns, or to the edge
// of its line that way, kept in the table.
func stretch(c *policy.Ctx, rows, cols int, edge bool) policy.Result {
	cr := c.Selection().Cells
	g, ok := vocab.GridOf(c.Doc(), cr.Table)
	if !ok {
		return policy.Swallow
	}
	n, m := g.Size()
	if edge {
		rows, cols = rows*n, cols*m
	}
	head := doc.CellCoord{Row: min(max(cr.Head.Row+rows, 0), n-1), Column: min(max(cr.Head.Column+cols, 0), m-1)}
	if head == cr.Head {
		return policy.Swallow
	}
	cr.Head = head
	return policy.Select(doc.CellSelection(cr))
}

// resize makes a table Rows by Columns: empty lines go at its end to grow
// it, and only empty ones at its end go to shrink it, so no text goes.
func resize(c *policy.Ctx, in profile.ResizeTable, _ policy.At[doc.SlotRef]) policy.Result {
	g, ok := vocab.GridOf(c.Doc(), in.Table)
	if !ok {
		return policy.Swallow
	}
	id := g.Table.ID()
	rows, cols := g.Size()
	var edits []vocab.Edit
	switch want := max(in.Rows, 1); {
	case want > rows:
		edits = append(edits, vocab.InsertRows(id, rows, want-rows))
	case want < rows:
		keep := rows
		for keep > want && empty(g, profile.Rows, keep-1) {
			keep--
		}
		if keep < rows {
			edits = append(edits, vocab.RemoveRows(id, keep, rows))
		}
	}
	switch want := max(in.Columns, 1); {
	case want > cols:
		edits = append(edits, profile.InsertColumns(id, cols, want-cols))
	case want < cols:
		keep := cols
		for keep > want && empty(g, profile.Columns, keep-1) {
			keep--
		}
		if keep < cols {
			edits = append(edits, profile.RemoveColumns(id, keep, cols))
		}
	}
	if len(edits) == 0 {
		return policy.Swallow
	}
	return policy.Do(edits...)
}

// empty reports whether a table's line k holds no text.
func empty(g vocab.Grid, axis profile.Axis, k int) bool {
	rows, cols := g.Size()
	n := cols
	if axis == profile.Columns {
		n = rows
	}
	for i := range n {
		at := doc.CellCoord{Row: k, Column: i}
		if axis == profile.Columns {
			at = doc.CellCoord{Row: i, Column: k}
		}
		if e, ok := vocab.TextEdge(g.Cell(at)); ok && e.To.Offset > 0 {
			return false
		}
	}
	return true
}

// shapeColumn gives a column its shape.
func shapeColumn(c *policy.Ctx, in profile.ShapeColumn, _ policy.At[doc.SlotRef]) policy.Result {
	g, ok := vocab.GridOf(c.Doc(), in.Table)
	if !ok {
		return policy.Swallow
	}
	shape := profile.TableShapeOf(g.Table)
	if in.Column < 0 || in.Column >= len(shape.Columns) || shape.Columns[in.Column] == in.Shape {
		return policy.Swallow
	}
	shape.Columns[in.Column] = in.Shape
	return policy.Do(profile.SetTableShape(g.Table.ID(), shape))
}

// align aligns the columns the selection stands in.
func align(c *policy.Ctx, in profile.AlignColumns, _ policy.At[doc.SlotRef]) policy.Result {
	s, ok := spanAt(c)
	if !ok {
		return policy.Swallow
	}
	shape := profile.TableShapeOf(s.grid.Table)
	changed := false
	for k := s.cols[0]; k <= s.cols[1]; k++ {
		changed = changed || shape.Columns[k].Align != in.To
		shape.Columns[k].Align = in.To
	}
	if !changed {
		return policy.Swallow
	}
	return policy.Do(profile.SetTableShape(s.grid.Table.ID(), shape))
}

// header makes the first row of the selection's table head it, or not.
func header(c *policy.Ctx, _ profile.ToggleHeader, _ policy.At[doc.SlotRef]) policy.Result {
	g, ok := tableOf(c, 0)
	if !ok {
		return policy.Swallow
	}
	shape := profile.TableShapeOf(g.Table)
	shape.Header = !shape.Header
	return policy.Do(profile.SetTableShape(g.Table.ID(), shape))
}

// remove takes the selection's table away.
func remove(c *policy.Ctx, _ profile.DeleteTable, _ policy.At[doc.SlotRef]) policy.Result {
	g, ok := tableOf(c, 0)
	if !ok {
		return policy.Swallow
	}
	return removeTable(c, g.Table)
}

// removeTable takes a table away as a whole goes (vocab.Structure); the
// caret goes on in the block that takes its place, or the one before.
func removeTable(c *policy.Ctx, table *doc.Node) policy.Result {
	s, ok := policy.As(c, table, vocab.RoleStructure)
	if !ok {
		return policy.Swallow
	}
	gap, ok := vocab.Beside(c.Doc(), table.ID(), vocab.Backward)
	if !ok {
		return policy.Swallow
	}
	res := policy.Do(s.Remove())
	res.Then = func(r doc.Reader) doc.Selection {
		if at, ok := vocab.BlockCaret(r, gap, vocab.Backward); ok {
			return doc.Collapsed(at)
		}
		gap.Offset--
		if at, ok := vocab.BlockCaret(r, gap, vocab.Forward); ok {
			return doc.Collapsed(at)
		}
		return doc.Selection{}
	}
	return res
}

// nextCell is Tab in a cell: the next cell in reading order; from the
// last, a row made for it below.
func nextCell(c *policy.Ctx, _ vocab.Leave, at policy.At[*doc.Node]) policy.Result {
	g, here, ok := vocab.Locate(c.Doc(), at.Data.ID())
	if !ok {
		return policy.Swallow
	}
	rows, cols := g.Size()
	id := g.Table.ID()
	if here.Column+1 < cols {
		return land(id, doc.CellCoord{Row: here.Row, Column: here.Column + 1})
	}
	if here.Row+1 < rows {
		return land(id, doc.CellCoord{Row: here.Row + 1})
	}
	return land(id, doc.CellCoord{Row: rows}, vocab.InsertRows(id, rows, 1))
}

// previousCell is Shift+Tab in a cell: the cell before in reading order.
func previousCell(c *policy.Ctx, _ vocab.Return, at policy.At[*doc.Node]) policy.Result {
	g, here, ok := vocab.Locate(c.Doc(), at.Data.ID())
	if !ok {
		return policy.Swallow
	}
	_, cols := g.Size()
	switch {
	case here.Column > 0:
		return land(g.Table.ID(), doc.CellCoord{Row: here.Row, Column: here.Column - 1})
	case here.Row > 0:
		return land(g.Table.ID(), doc.CellCoord{Row: here.Row - 1, Column: cols - 1})
	}
	return policy.Swallow
}

// cellBelow is Return in a cell: the cell below; from the last row, a row
// made for it. On an empty last row Return leaves the table: the row
// goes, and writing goes on in a paragraph after the table.
func cellBelow(c *policy.Ctx, _ vocab.Break, at policy.At[*doc.Node]) policy.Result {
	g, here, ok := vocab.Locate(c.Doc(), at.Data.ID())
	if !ok {
		return policy.Swallow
	}
	rows, _ := g.Size()
	id := g.Table.ID()
	if here.Row+1 < rows {
		return land(id, doc.CellCoord{Row: here.Row + 1, Column: here.Column})
	}
	if !empty(g, profile.Rows, here.Row) {
		return land(id, doc.CellCoord{Row: rows, Column: here.Column}, vocab.InsertRows(id, rows, 1))
	}
	after, ok := vocab.Beside(c.Doc(), id, vocab.Forward)
	if !ok {
		return policy.Swallow
	}
	r := c.Doc()
	para := vocab.NewBlock(r.Schema(), r.Schema().Plain(), doc.Attrs{})
	edits := []vocab.Edit{vocab.Ops(doc.Insert{Parent: after.Node, Slot: after.Slot, Index: after.Offset, Nodes: []*doc.Node{para}})}
	gap := after
	if rows > 1 {
		edits = append(edits, vocab.RemoveRows(id, here.Row, here.Row+1))
	} else {
		// A table of one empty row goes, the paragraph in its place.
		edits = append(edits, vocab.Ops(doc.Remove{IDs: []doc.ID{id}}))
		gap.Offset--
	}
	res := policy.Do(edits...)
	res.Then = func(r doc.Reader) doc.Selection {
		at, _ := vocab.BlockCaret(r, gap, vocab.Backward)
		return doc.Collapsed(at)
	}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}
