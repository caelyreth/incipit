package vocab

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
)

// IntentKind is the kind of an intent: the closed vocabulary all input
// reaches the engine as.
type IntentKind uint8

const (
	IntentType IntentKind = iota + 1
	IntentDelete
	IntentBreak
	IntentSoftBreak
	IntentPaste
	IntentCut
	IntentCompose
	IntentCancelComposition
	IntentMove
	IntentVisualMove
	IntentPlace
	IntentLeave
	IntentReturn
	IntentSelectScope
	IntentDismiss
	IntentFormat
	IntentNest
	IntentReshape
	IntentMoveNode
	IntentToggleTask
	IntentTable
	IntentFigure
	IntentMedia
	IntentTimer
	IntentUndo
	IntentRedo
	IntentCommand
	numIntents
)

// IntentKinds lists every intent kind.
func IntentKinds() []IntentKind {
	out := make([]IntentKind, 0, numIntents-1)
	for k := IntentType; k < numIntents; k++ {
		out = append(out, k)
	}
	return out
}

var intentNames = [...]string{"", "Type", "Delete", "Break", "SoftBreak", "Paste", "Cut", "Compose",
	"CancelComposition", "Move", "VisualMove", "Place", "Leave", "Return", "SelectScope", "Dismiss",
	"Format", "Nest", "Reshape", "MoveNode", "ToggleTask", "Table", "Figure", "Media", "Timer",
	"Undo", "Redo", "Command"}

func (k IntentKind) String() string {
	if int(k) < len(intentNames) {
		return intentNames[k]
	}
	return "intent?"
}

// Intent is one input, carrying every host answer it needs: geometry the
// renderer resolved, clipboard contents, composition text, media results.
type Intent interface {
	Kind() IntentKind
}

// Unit is what a logical motion or deletion moves over.
type Unit uint8

const (
	ByGrapheme Unit = iota + 1
	ByWord
	BySentence
	ByParagraph
	// ToSlotEdge goes to the start or end of the caret's slot.
	ToSlotEdge
	// ToPageEdge goes to the start or end of the page.
	ToPageEdge
)

// Direction is logical order: backward or forward.
type Direction int8

const (
	Backward Direction = -1
	Forward  Direction = 1
)

// VisualDirection is a motion the renderer resolves from geometry.
type VisualDirection uint8

const (
	Up VisualDirection = iota + 1
	Down
	LineStart
	LineEnd
	PageUp
	PageDown
	Left
	Right
)

// Text intents.
type (
	Type   struct{ Text string }
	Delete struct {
		Unit Unit
		Dir  Direction
	}
	Break     struct{}
	SoftBreak struct{}
	// Paste carries the clipboard: a slice when it held structure, and its
	// plain text.
	Paste struct {
		Slice Slice
		Text  string
	}
	Cut struct{}
)

// Composition intents. A commit arrives as Type with origin IME.
type (
	// Compose sets the preedit; Selected is a byte range within it.
	Compose struct {
		Preedit  string
		Selected [2]int
	}
	CancelComposition struct{}
)

// Motion intents.
type (
	Move struct {
		Unit   Unit
		Dir    Direction
		Extend bool
	}
	// VisualMove carries the stop the renderer resolved for Dir, the goal
	// x to keep across lines, and whether the caret stood at the edge of
	// its surface.
	VisualMove struct {
		Dir           VisualDirection
		Target        doc.Caret
		GoalX         float32
		AtSurfaceEdge bool
		Extend        bool
	}
	// Place puts the caret at a stop the renderer hit, as a click does;
	// Clicks counts a double or triple click, Toggle adds to a selection.
	// Node, when set, is an object the pointer hit.
	Place struct {
		Stop   doc.Caret
		Node   doc.ID
		Clicks int
		Extend bool
		Toggle bool
	}
)

// Scope intents.
type (
	Leave       struct{}
	Return      struct{}
	SelectScope struct{}
	Dismiss     struct{}
)

// Structure intents.
type (
	// Format toggles a mark over the selection.
	Format struct{ Mark doc.MarkKind }
	// Nest moves the caret's container entry in (Forward) or out.
	Nest struct{ Dir Direction }
	// Reshape gives the caret's block another shape: a kind and its
	// attributes.
	Reshape struct {
		Shape doc.Kind
		Attrs doc.Attrs
	}
	MoveNode struct {
		ID     doc.ID
		Target doc.Pos
	}
	ToggleTask struct{ ID doc.ID }
	// TableOp is one of the table operations.
	TableOp struct {
		Op    TableAction
		Table doc.ID
		// Index is the row or column acted on; Value an alignment or width.
		Index int
		Value int64
	}
	// FigureOp is one of the figure operations.
	FigureOp struct {
		Op     FigureAction
		Figure doc.ID
		Width  int
		Height int
		Align  Align
		Media  string
	}
)

// TableAction is a table operation.
type TableAction uint8

const (
	InsertRowAbove TableAction = iota + 1
	InsertRowBelow
	InsertColumnBefore
	InsertColumnAfter
	RemoveRow
	RemoveColumn
	ToggleHeader
	AlignColumn
	ResizeColumn
	ClearCells
)

// FigureAction is a figure operation.
type FigureAction uint8

const (
	ResizeFigure FigureAction = iota + 1
	AlignFigure
	ReplaceMedia
)

// Host results.
type (
	// Media answers a media request the engine made with token.
	Media struct {
		Token  uint64
		Result MediaResult
	}
	// Timer answers a deadline the engine published.
	Timer struct{ Deadline time.Time }
)

// MediaResult is what the host made of a media request.
type MediaResult struct {
	ID            string
	Width, Height int
	Err           string
}

// History intents. TopLayer undoes only the newest entry's top layer: the
// automatic correction, keeping what was typed.
type (
	Undo struct{ TopLayer bool }
	Redo struct{}
)

// CommandID is a command's stable id; user keymaps store it.
type CommandID string

// Command runs a command a policy declared.
type Command struct{ ID CommandID }

func (Type) Kind() IntentKind              { return IntentType }
func (Delete) Kind() IntentKind            { return IntentDelete }
func (Break) Kind() IntentKind             { return IntentBreak }
func (SoftBreak) Kind() IntentKind         { return IntentSoftBreak }
func (Paste) Kind() IntentKind             { return IntentPaste }
func (Cut) Kind() IntentKind               { return IntentCut }
func (Compose) Kind() IntentKind           { return IntentCompose }
func (CancelComposition) Kind() IntentKind { return IntentCancelComposition }
func (Move) Kind() IntentKind              { return IntentMove }
func (VisualMove) Kind() IntentKind        { return IntentVisualMove }
func (Place) Kind() IntentKind             { return IntentPlace }
func (Leave) Kind() IntentKind             { return IntentLeave }
func (Return) Kind() IntentKind            { return IntentReturn }
func (SelectScope) Kind() IntentKind       { return IntentSelectScope }
func (Dismiss) Kind() IntentKind           { return IntentDismiss }
func (Format) Kind() IntentKind            { return IntentFormat }
func (Nest) Kind() IntentKind              { return IntentNest }
func (Reshape) Kind() IntentKind           { return IntentReshape }
func (MoveNode) Kind() IntentKind          { return IntentMoveNode }
func (ToggleTask) Kind() IntentKind        { return IntentToggleTask }
func (TableOp) Kind() IntentKind           { return IntentTable }
func (FigureOp) Kind() IntentKind          { return IntentFigure }
func (Media) Kind() IntentKind             { return IntentMedia }
func (Timer) Kind() IntentKind             { return IntentTimer }
func (Undo) Kind() IntentKind              { return IntentUndo }
func (Redo) Kind() IntentKind              { return IntentRedo }
func (Command) Kind() IntentKind           { return IntentCommand }

// Input is an intent as the host delivers it: when it happened, and its
// origin class. The engine never reads a clock.
type Input struct {
	Intent Intent
	At     time.Time
	// Class is the origin class; zero takes the intent's own.
	Class doc.OriginClass
}

// ClassOf is an input's origin class.
func (in Input) ClassOf() doc.OriginClass {
	if in.Class != 0 {
		return in.Class
	}
	switch in.Intent.Kind() {
	case IntentType, IntentDelete, IntentBreak, IntentSoftBreak:
		return doc.OriginTyping
	case IntentCompose, IntentCancelComposition:
		return doc.OriginIME
	case IntentPaste:
		return doc.OriginPaste
	case IntentPlace:
		return doc.OriginPointer
	case IntentUndo, IntentRedo:
		return doc.OriginHistory
	case IntentTimer, IntentMedia:
		return doc.OriginTimer
	}
	return doc.OriginCommand
}

// Pattern matches an intent kind and, optionally, its parameters: an
// exact text or a character class for Type, a unit and direction for
// Delete and Move.
type Pattern struct {
	Kind  IntentKind
	Text  string
	Class CharClass
	Unit  Unit
	Dir   Direction
}

// On matches every intent of kind k.
func On(k IntentKind) Pattern { return Pattern{Kind: k} }

// Typed matches typing exactly text.
func Typed(text string) Pattern { return Pattern{Kind: IntentType, Text: text} }

// TypedClass matches typing one character of class c.
func TypedClass(c CharClass) Pattern { return Pattern{Kind: IntentType, Class: c} }

// Deleting matches deleting by unit in dir; a zero unit or direction
// matches any.
func Deleting(u Unit, d Direction) Pattern { return Pattern{Kind: IntentDelete, Unit: u, Dir: d} }

// Moving matches a logical motion by unit in dir.
func Moving(u Unit, d Direction) Pattern { return Pattern{Kind: IntentMove, Unit: u, Dir: d} }

// Specific reports whether the pattern names parameters, which ranks it
// above a pattern of the same kind that names none.
func (p Pattern) Specific() bool { return p.Text != "" || p.Class != 0 || p.Unit != 0 || p.Dir != 0 }

// Match reports whether in matches the pattern.
func (p Pattern) Match(in Intent) bool {
	if in.Kind() != p.Kind {
		return false
	}
	switch v := in.(type) {
	case Type:
		if p.Text != "" && v.Text != p.Text {
			return false
		}
		if p.Class != 0 {
			r, n := firstRune(v.Text)
			if n != len(v.Text) || ClassOf(r)&p.Class == 0 {
				return false
			}
		}
	case Delete:
		return (p.Unit == 0 || p.Unit == v.Unit) && (p.Dir == 0 || p.Dir == v.Dir)
	case Move:
		return (p.Unit == 0 || p.Unit == v.Unit) && (p.Dir == 0 || p.Dir == v.Dir)
	}
	return true
}

// Overlaps reports whether some intent matches both patterns.
func (p Pattern) Overlaps(o Pattern) bool {
	if p.Kind != o.Kind {
		return false
	}
	if p.Text != "" && o.Text != "" && p.Text != o.Text {
		return false
	}
	if p.Text != "" && o.Class != 0 {
		r, n := firstRune(p.Text)
		return n == len(p.Text) && ClassOf(r)&o.Class != 0
	}
	if o.Text != "" && p.Class != 0 {
		return o.Overlaps(p)
	}
	if p.Class != 0 && o.Class != 0 && p.Class&o.Class == 0 {
		return false
	}
	if p.Unit != 0 && o.Unit != 0 && p.Unit != o.Unit {
		return false
	}
	if p.Dir != 0 && o.Dir != 0 && p.Dir != o.Dir {
		return false
	}
	return true
}
