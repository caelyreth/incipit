package profile

import (
	"slices"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// A table is a grid (vocab.Grid) that keeps its columns' shape and its
// header on itself. Its grid edits that change the columns change that
// shape in the same edit, so the two never disagree at any step.

// Column is a table column's shape.
type Column struct {
	Align Align
	// Width is the column's width in DIPs; 0 sets it from its text.
	Width int
	// Fit keeps the column's text on one line.
	Fit bool
}

// TableShape is a table's columns, in order, and whether its first row
// heads it.
type TableShape struct {
	Columns []Column
	Header  bool
}

// TableShapeOf is a table's shape.
func TableShapeOf(n *doc.Node) TableShape {
	s := Schema()
	shape := TableShape{Header: s.Attr(n, FieldHeader).Bool()}
	for _, rec := range s.Attr(n, FieldColumns).Records() {
		var c Column
		if v, ok := rec.Get(FieldColumnAlign); ok {
			c.Align = Align(v.Int())
		}
		if v, ok := rec.Get(FieldColumnWidth); ok {
			c.Width = int(v.Int())
		}
		if v, ok := rec.Get(FieldColumnFit); ok {
			c.Fit = v.Bool()
		}
		shape.Columns = append(shape.Columns, c)
	}
	return shape
}

// Attrs are the attributes of a table of this shape, in canonical form:
// a column at its defaults keeps no field.
func (t TableShape) Attrs() doc.Attrs {
	recs := make([]doc.Attrs, len(t.Columns))
	for i, c := range t.Columns {
		var fields []doc.Field
		if a := min(max(c.Align, AlignStart), AlignEnd); a != AlignStart {
			fields = append(fields, doc.Field{Num: FieldColumnAlign, Value: doc.Int(int64(a))})
		}
		if w := min(max(c.Width, 0), 1<<16); w != 0 {
			fields = append(fields, doc.Field{Num: FieldColumnWidth, Value: doc.Int(int64(w))})
		}
		if c.Fit {
			fields = append(fields, doc.Field{Num: FieldColumnFit, Value: doc.Bool(true)})
		}
		recs[i] = doc.MakeAttrs(fields...)
	}
	return Schema().Canonical(Table, doc.MakeAttrs(
		doc.Field{Num: FieldColumns, Value: doc.List(recs...)},
		doc.Field{Num: FieldHeader, Value: doc.Bool(t.Header)},
	))
}

// NewTable is an empty table of rows by columns, its first row heading it.
func NewTable(rows, columns int) *doc.Node {
	texts := make([][]string, max(rows, 1))
	for i := range texts {
		texts[i] = make([]string, max(columns, 1))
	}
	return TableOf(texts)
}

// TableOf is a table of rows of its cells' texts, each row as long as the
// longest, its first row heading it.
func TableOf(rows [][]string) *doc.Node {
	s := Schema()
	columns := 1
	for _, row := range rows {
		columns = max(columns, len(row))
	}
	shape := TableShape{Columns: make([]Column, columns), Header: true}
	rs := make([]*doc.Node, len(rows))
	for i, row := range rows {
		cells := make([]*doc.Node, columns)
		for j := range cells {
			var text []doc.Item
			if j < len(row) && row[j] != "" {
				text = []doc.Item{doc.TextItem(row[j], nil)}
			}
			cells[j] = vocab.NewBlock(s, Cell, doc.Attrs{}, text...)
		}
		rs[i] = doc.NewNode(Row, doc.Attrs{}, doc.BlockSlot(cells...))
	}
	return doc.NewNode(Table, shape.Attrs(), doc.BlockSlot(rs...))
}

// SetTableShape gives a table a shape of as many columns as it has.
func SetTableShape(id doc.ID, shape TableShape) vocab.Edit {
	return vocab.SetAttrs(id, shape.Attrs())
}

// reshape runs a grid edit and changes the table's columns as it does.
func reshape(table doc.ID, grid vocab.Edit, columns func([]Column) []Column) vocab.Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(table)
		if !ok {
			return doc.ErrNoNode
		}
		shape := TableShapeOf(n)
		if err := grid(tx); err != nil {
			return err
		}
		shape.Columns = columns(slices.Clone(shape.Columns))
		return SetTableShape(table, shape)(tx)
	}
}

// InsertColumns puts count empty columns before column at, at the column
// count after the last, each shaped as the column it stands by.
func InsertColumns(table doc.ID, at, count int) vocab.Edit {
	return reshape(table, vocab.InsertColumns(table, at, count), func(cs []Column) []Column {
		like := cs[min(at, len(cs)-1)]
		like.Width = 0
		return slices.Insert(cs, at, slices.Repeat([]Column{like}, count)...)
	})
}

// RemoveColumns takes columns from up to to away.
func RemoveColumns(table doc.ID, from, to int) vocab.Edit {
	return reshape(table, vocab.RemoveColumns(table, from, to), func(cs []Column) []Column {
		return slices.Delete(cs, from, to)
	})
}

// MoveColumn moves column from to stand at to, its shape with it.
func MoveColumn(table doc.ID, from, to int) vocab.Edit {
	return reshape(table, vocab.MoveColumn(table, from, to), func(cs []Column) []Column {
		c := cs[from]
		return slices.Insert(slices.Delete(cs, from, from+1), to, c)
	})
}
