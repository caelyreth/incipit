// Package profile is the product's document profile: its node and mark
// kinds with their slots, fields and validity, their notation, highlight
// groups and the roles they provide. Generic code never imports it; the
// composition root hands it to the framework as a vocab.Profile, and
// kind-aware policies name its kinds.
package profile

import (
	"fmt"
	"sync"

	"github.com/caelyreth/incipit/internal/editor/doc"
)

// Node kinds. Values are persisted in notebook storage and must stay invariant.
const (
	Page      doc.Kind = 1
	Paragraph doc.Kind = 2
	Heading   doc.Kind = 3
	Quote     doc.Kind = 4
	List      doc.Kind = 5
	Item      doc.Kind = 6
	Code      doc.Kind = 7
	Math      doc.Kind = 8
	Divider   doc.Kind = 9
	Figure    doc.Kind = 10
	Table     doc.Kind = 11
	Row       doc.Kind = 12
	Cell      doc.Kind = 13
	Formula   doc.Kind = 14
)

// Mark kinds. Values are persisted in notebook storage and must stay invariant.
// Emphasis and strong are independent: bold italic is the set {strong, emphasis}.
const (
	Emphasis doc.MarkKind = 1
	Strong   doc.MarkKind = 2
	CodeMark doc.MarkKind = 3
	Strike   doc.MarkKind = 4
)

// MarkKinds are every mark kind, in kind order.
var MarkKinds = []doc.MarkKind{Emphasis, Strong, CodeMark, Strike}

// Field numbers. They are persisted, so they never change meaning.
const (
	FieldLevel = 1 // heading

	FieldListStyle = 1 // list
	FieldListStart = 2

	FieldTask = 1 // item

	FieldInfo = 1 // code

	FieldDialect = 1 // math, formula

	FieldMedia  = 1 // figure: its picture's media id
	FieldWidth  = 2 // the picture's size in pixels
	FieldAlign  = 3
	FieldHeight = 4
	FieldSpan   = 5 // the share of the column it takes, in thousandths

	FieldColumns = 1 // table
	FieldHeader  = 2

	FieldColumnAlign = 1 // a table column record
	FieldColumnWidth = 2 // in DIPs; 0 sets it from its text
	FieldColumnFit   = 3 // its text kept on one line
)

// ListStyle is a list's style. Values are persisted.
type ListStyle int64

const (
	Bullet  ListStyle = 0
	Ordered ListStyle = 1
)

// Task is an item's task state. Values are persisted.
type Task int64

const (
	NoTask   Task = 0
	TaskOpen Task = 1
	TaskDone Task = 2
)

// MathDialect is the input syntax of a formula or math block. Values are persisted.
type MathDialect int64

const (
	DialectTeX   MathDialect = 0
	DialectTypst MathDialect = 1
)

// Label is the short dialect label shown on open formula controls.
func (d MathDialect) Label() string {
	if d == DialectTypst {
		return "Typst"
	}
	return "TeX"
}

// Align is a figure's or a column's alignment. Values are persisted.
type Align int64

const (
	AlignStart  Align = 0
	AlignCenter Align = 1
	AlignEnd    Align = 2
)

func intIn(lo, hi int64) func(doc.Value) bool {
	return func(v doc.Value) bool { return v.Kind() == doc.ValueInt && v.Int() >= lo && v.Int() <= hi }
}

func isText(v doc.Value) bool { return v.Kind() == doc.ValueText }
func isBool(v doc.Value) bool { return v.Kind() == doc.ValueBool }

func columnFieldKnown(num uint16) bool {
	return num == FieldColumnAlign || num == FieldColumnWidth || num == FieldColumnFit
}

func columnsValid(v doc.Value) bool {
	if v.Kind() != doc.ValueList || v.Len() == 0 {
		return false
	}
	for _, column := range v.Records() {
		for _, f := range column.Fields() {
			switch f.Num {
			case FieldColumnAlign:
				if !intIn(0, 2)(f.Value) {
					return false
				}
			case FieldColumnWidth:
				if !intIn(0, 1<<16)(f.Value) {
					return false
				}
			case FieldColumnFit:
				if !isBool(f.Value) {
					return false
				}
			default:
				if f.Value.Kind() != doc.ValueRaw {
					return false
				}
			}
		}
	}
	return true
}

// flowBlocks is what a page, a quote and an item's body hold.
func flowBlocks(k doc.Kind) bool {
	switch k {
	case Paragraph, Heading, Quote, List, Code, Math, Divider, Figure, Table:
		return true
	}
	return false
}

func richMarks(doc.MarkKind) bool { return true }

func inlineObjects(k doc.Kind) bool { return k == Formula }

func only(kind doc.Kind) func(doc.Kind) bool { return func(k doc.Kind) bool { return k == kind } }

func richSlot(name string, lines bool) doc.SlotSpec {
	return doc.SlotSpec{Name: name, Marks: richMarks, Accepts: inlineObjects, Lines: lines}
}

var kindSpecs = []doc.KindSpec{
	{Kind: Page, Name: "page", Placement: doc.PlaceRoot,
		Slots: []doc.SlotSpec{{Name: "blocks", Blocks: true, Accepts: flowBlocks}}},
	{Kind: Paragraph, Name: "paragraph", Placement: doc.PlaceBlock, Plain: true,
		Slots: []doc.SlotSpec{richSlot("text", true)}},
	{Kind: Heading, Name: "heading", Placement: doc.PlaceBlock,
		Slots:  []doc.SlotSpec{richSlot("text", false)},
		Fields: []doc.FieldSpec{{Num: FieldLevel, Name: "level", Default: doc.Int(1), Valid: intIn(1, 6)}}},
	{Kind: Quote, Name: "quote", Placement: doc.PlaceBlock,
		Slots: []doc.SlotSpec{{Name: "body", Blocks: true, Accepts: flowBlocks}}},
	{Kind: List, Name: "list", Placement: doc.PlaceBlock,
		Slots: []doc.SlotSpec{{Name: "items", Blocks: true, Accepts: only(Item)}},
		Fields: []doc.FieldSpec{
			{Num: FieldListStyle, Name: "style", Default: doc.Int(int64(Bullet)), Valid: intIn(0, 1)},
			{Num: FieldListStart, Name: "start", Default: doc.Int(1), Valid: intIn(0, 1<<31)},
		}},
	{Kind: Item, Name: "item", Placement: doc.PlaceBlock, Entry: true,
		Slots:  []doc.SlotSpec{richSlot("head", true), {Name: "body", Blocks: true, Accepts: flowBlocks}},
		Fields: []doc.FieldSpec{{Num: FieldTask, Name: "task", Default: doc.Int(int64(NoTask)), Valid: intIn(0, 2)}}},
	{Kind: Code, Name: "code", Placement: doc.PlaceBlock,
		Slots:  []doc.SlotSpec{{Name: "text", Literal: true, Lines: true}},
		Fields: []doc.FieldSpec{{Num: FieldInfo, Name: "info", Default: doc.Text(""), Valid: isText}}},
	{Kind: Math, Name: "math", Placement: doc.PlaceBlock, Object: true,
		Slots:  []doc.SlotSpec{{Name: "source", Literal: true, Lines: true}},
		Fields: []doc.FieldSpec{{Num: FieldDialect, Name: "dialect", Default: doc.Int(int64(DialectTeX)), Valid: intIn(0, 1)}}},
	{Kind: Divider, Name: "divider", Placement: doc.PlaceBlock, Object: true},
	{Kind: Figure, Name: "figure", Placement: doc.PlaceBlock, Object: true,
		Slots: []doc.SlotSpec{richSlot("caption", false)},
		Fields: []doc.FieldSpec{
			{Num: FieldMedia, Name: "media", Default: doc.Text(""), Valid: isText},
			{Num: FieldWidth, Name: "width", Default: doc.Int(0), Valid: intIn(0, 1<<16)},
			{Num: FieldAlign, Name: "align", Default: doc.Int(int64(AlignCenter)), Valid: intIn(0, 2)},
			{Num: FieldHeight, Name: "height", Default: doc.Int(0), Valid: intIn(0, 1<<16)},
			{Num: FieldSpan, Name: "span", Default: doc.Int(0), Valid: intIn(0, SpanFull)},
		}},
	{Kind: Table, Name: "table", Placement: doc.PlaceBlock, Object: true,
		Slots: []doc.SlotSpec{{Name: "rows", Blocks: true, Accepts: only(Row)}},
		Fields: []doc.FieldSpec{
			{Num: FieldColumns, Name: "columns", Default: doc.List(), Valid: columnsValid, NestedFields: columnFieldKnown},
			{Num: FieldHeader, Name: "header", Default: doc.Bool(true), Valid: isBool},
		},
		Check: checkTable},
	{Kind: Row, Name: "row", Placement: doc.PlaceBlock,
		Slots: []doc.SlotSpec{{Name: "cells", Blocks: true, Accepts: only(Cell)}}},
	{Kind: Cell, Name: "cell", Placement: doc.PlaceBlock,
		Slots: []doc.SlotSpec{richSlot("text", true)}},
	{Kind: Formula, Name: "formula", Placement: doc.PlaceInline, Object: true,
		Slots:  []doc.SlotSpec{{Name: "source", Literal: true}},
		Fields: []doc.FieldSpec{{Num: FieldDialect, Name: "dialect", Default: doc.Int(int64(DialectTeX)), Valid: intIn(0, 1)}}},
}

// checkTable holds every row to as many cells as the table has columns.
func checkTable(n *doc.Node) error {
	columns, _ := n.Attrs().Get(FieldColumns)
	for _, row := range n.Slot(0).Children() {
		if got := row.Slot(0).Len(); got != columns.Len() {
			return fmt.Errorf("a row has %d cells for %d columns", got, columns.Len())
		}
	}
	return nil
}

var markSpecs = []doc.MarkSpec{
	{Kind: Emphasis, Name: "emphasis"},
	{Kind: Strong, Name: "strong"},
	{Kind: CodeMark, Name: "code", Bounded: true, Literal: true, Excludes: func(doc.MarkKind) bool { return true }},
	{Kind: Strike, Name: "strike"},
}

var schema = sync.OnceValue(func() *doc.Schema {
	s, err := doc.NewSchema(Page, kindSpecs, markSpecs)
	if err != nil {
		panic(err)
	}
	return s
})

// Schema is the document schema of every page.
func Schema() *doc.Schema { return schema() }

// HeadingLevel is a heading's level.
func HeadingLevel(n *doc.Node) int { return int(Schema().Attr(n, FieldLevel).Int()) }

// ListStyleOf is a list's style.
func ListStyleOf(n *doc.Node) ListStyle { return ListStyle(Schema().Attr(n, FieldListStyle).Int()) }

// ListStart is an ordered list's first number.
func ListStart(n *doc.Node) int { return int(Schema().Attr(n, FieldListStart).Int()) }

// TaskOf is an item's task state.
func TaskOf(n *doc.Node) Task { return Task(Schema().Attr(n, FieldTask).Int()) }

// MathDialectOf is a formula or math block's input dialect.
func MathDialectOf(n *doc.Node) MathDialect {
	if n == nil {
		return DialectTeX
	}
	return MathDialect(Schema().Attr(n, FieldDialect).Int())
}

// MathDialectAttrs returns the node attributes for dialect.
func MathDialectAttrs(d MathDialect) doc.Attrs {
	if d == DialectTypst {
		return doc.MakeAttrs(doc.Field{Num: FieldDialect, Value: doc.Int(int64(DialectTypst))})
	}
	return doc.Attrs{}
}

// EmptyPage is a page holding one empty paragraph. Its node ids come from
// seed's stream, so the same seed builds the same page.
func EmptyPage(root doc.ID, seed uint64, marks *doc.Interner) (*doc.Snapshot, error) {
	bare, err := doc.NewSnapshot(Schema(), marks, doc.NewNodeWithID(root, Page, doc.Attrs{}, "", doc.BlockSlot()), seed)
	if err != nil {
		return nil, err
	}
	tx := bare.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.Insert{Parent: root, Nodes: []*doc.Node{NewParagraph()}}); err != nil {
		return nil, err
	}
	return tx.Commit()
}

// NewParagraph is an empty paragraph to insert.
func NewParagraph(items ...doc.Item) *doc.Node {
	return doc.NewNode(Paragraph, doc.Attrs{}, doc.InlineSlot(items...))
}
