package profile

import (
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// The product's feature intents: what its commands, menus and controls
// ask for. Each is answered by the policy owning the feature; with that
// policy disabled, the intent does nothing.
var (
	intentFormat     = vocab.NewIntentKind("Format")
	intentRetype     = vocab.NewIntentKind("Retype")
	intentWrapQuote  = vocab.NewIntentKind("WrapQuote")
	intentMakeList   = vocab.NewIntentKind("MakeList")
	intentNest       = vocab.NewIntentKind("Nest")
	intentToggleTask = vocab.NewIntentKind("ToggleTask")
	intentPick       = vocab.NewIntentKind("PickPictures")
	intentPlace      = vocab.NewIntentKind("PlaceFigures")
	intentResize     = vocab.NewIntentKind("ResizeFigure")
	intentAlign      = vocab.NewIntentKind("AlignFigure")
	intentUse        = vocab.NewIntentKind("UsePicture")
	intentReplace    = vocab.NewIntentKind("ReplacePicture")
	intentTable      = vocab.NewIntentKind("InsertTable")
	intentGrid       = vocab.NewIntentKind("EditGrid")
	intentResizeT    = vocab.NewIntentKind("ResizeTable")
	intentColumn     = vocab.NewIntentKind("ShapeColumn")
	intentAlignCol   = vocab.NewIntentKind("AlignColumns")
	intentHeader     = vocab.NewIntentKind("ToggleHeader")
	intentDropTable  = vocab.NewIntentKind("DeleteTable")
	intentPickTag    = vocab.NewIntentKind("PickTag")
	intentSetDialect = vocab.NewIntentKind("SetMathDialect")
	intentTogDialect = vocab.NewIntentKind("ToggleMathDialect")
	intentHeading    = vocab.NewIntentKind("JumpHeading")
)

type (
	// Format toggles a mark over the selection, or the marks text typed at
	// a caret takes.
	Format struct{ Mark doc.MarkKind }
	// Retype gives the selected text blocks a kind and its attributes; the
	// blocks that already have them become paragraphs again.
	Retype struct {
		To    doc.Kind
		Attrs doc.Attrs
	}
	// JumpHeading puts the caret at the start of the next heading the way
	// Dir goes, of any level; with none that way, it stays.
	JumpHeading struct{ Dir vocab.Direction }
	// PickTag names the tag the caret is in, as a tag picker chooses one:
	// the tag reads "#" and Name, and the caret goes past the space ending
	// its word.
	PickTag struct{ Name string }
	// WrapQuote wraps the selected blocks in a quote.
	WrapQuote struct{}
	// MakeList makes each selected block an item of a list with attrs.
	MakeList struct{ Attrs doc.Attrs }
	// Nest moves the caret's item in (Forward) or out.
	Nest struct{ Dir vocab.Direction }
	// ToggleTask completes or reopens a task: item ID, or the caret's.
	ToggleTask struct{ ID doc.ID }
	// PickPictures asks the host for pictures to place where the selection
	// is; the host answers with PlaceFigures once it keeps them.
	PickPictures struct{}
	// PlaceFigures places kept pictures as figures, in order: at At, a
	// caret where they were dropped, or else where the selection is.
	PlaceFigures struct {
		Pictures []Picture
		At       *doc.Caret
	}
	// ResizeFigure sets a figure's span, as a resize gesture ends.
	ResizeFigure struct {
		ID   doc.ID
		Span int
	}
	// AlignFigure aligns the selected figure.
	AlignFigure struct{ To Align }
	// UsePicture asks the host to do Use with the selected figure's
	// picture; to replace it, the host answers with ReplacePicture once it
	// keeps the picture picked.
	UsePicture struct{ Use PictureUse }
	// ReplacePicture shows another picture in figure ID, keeping its
	// caption, alignment and span.
	ReplacePicture struct {
		ID      doc.ID
		Picture Picture
	}

	// InsertTable puts an empty table of Rows by Columns where the
	// selection is, as a figure is placed; the caret goes to its first
	// cell.
	InsertTable struct{ Rows, Columns int }
	// EditGrid inserts, removes or moves rows or columns of a table. With
	// Table set, it edits Count lines from At, a move putting them at To,
	// as a gesture ends; with none, the lines the selection's cells stand
	// in, an insert of as many going before them (Before) or after.
	EditGrid struct {
		Op     GridOp
		Axis   Axis
		Table  doc.ID
		At     int
		Count  int
		To     int
		Before bool
	}
	// ResizeTable makes a table Rows by Columns, as its corner's drag ends:
	// it grows by empty lines at its end, and shrinks only by empty ones.
	ResizeTable struct {
		Table         doc.ID
		Rows, Columns int
	}
	// ShapeColumn gives a table's column a shape, as its border's drag or
	// its menu does.
	ShapeColumn struct {
		Table  doc.ID
		Column int
		Shape  Column
	}
	// AlignColumns aligns the columns the selection's cells stand in.
	AlignColumns struct{ To Align }
	// ToggleHeader makes the first row of the selection's table head it,
	// or a row like the others.
	ToggleHeader struct{}
	// DeleteTable takes the selection's table away.
	DeleteTable struct{}
	// SetMathDialect sets a formula or math block's input dialect.
	SetMathDialect struct {
		ID      doc.ID
		Dialect MathDialect
	}
	// ToggleMathDialect toggles the active formula or math block between TeX
	// and Typst.
	ToggleMathDialect struct{}
)

// GridOp is what EditGrid does to a table's lines.
type GridOp uint8

const (
	GridInsert GridOp = iota + 1
	GridRemove
	GridMove
)

// Axis is a table's rows or its columns.
type Axis uint8

const (
	Rows Axis = iota + 1
	Columns
)

func (Format) Kind() vocab.IntentKind         { return intentFormat }
func (JumpHeading) Kind() vocab.IntentKind    { return intentHeading }
func (Retype) Kind() vocab.IntentKind         { return intentRetype }
func (WrapQuote) Kind() vocab.IntentKind      { return intentWrapQuote }
func (MakeList) Kind() vocab.IntentKind       { return intentMakeList }
func (Nest) Kind() vocab.IntentKind           { return intentNest }
func (ToggleTask) Kind() vocab.IntentKind     { return intentToggleTask }
func (PickPictures) Kind() vocab.IntentKind   { return intentPick }
func (PlaceFigures) Kind() vocab.IntentKind   { return intentPlace }
func (ResizeFigure) Kind() vocab.IntentKind   { return intentResize }
func (AlignFigure) Kind() vocab.IntentKind    { return intentAlign }
func (UsePicture) Kind() vocab.IntentKind     { return intentUse }
func (ReplacePicture) Kind() vocab.IntentKind { return intentReplace }
func (InsertTable) Kind() vocab.IntentKind    { return intentTable }
func (EditGrid) Kind() vocab.IntentKind       { return intentGrid }
func (ResizeTable) Kind() vocab.IntentKind    { return intentResizeT }
func (ShapeColumn) Kind() vocab.IntentKind    { return intentColumn }
func (AlignColumns) Kind() vocab.IntentKind   { return intentAlignCol }
func (ToggleHeader) Kind() vocab.IntentKind      { return intentHeader }
func (DeleteTable) Kind() vocab.IntentKind       { return intentDropTable }
func (PickTag) Kind() vocab.IntentKind           { return intentPickTag }
func (SetMathDialect) Kind() vocab.IntentKind    { return intentSetDialect }
func (ToggleMathDialect) Kind() vocab.IntentKind { return intentTogDialect }

// SourceIndent is one level of source text's indentation: what Nest adds
// at the start of each line of a slot that keeps literal lines, as a code
// block's, and takes from it again.
const SourceIndent = "    "
