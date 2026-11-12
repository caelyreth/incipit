package vocab

import (
	"sync"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
)

// IntentKind is the kind of an intent. The primitives below are the
// editing vocabulary every program answers; a profile or policy adds the
// kinds of its features with NewIntentKind.
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
	IntentMoveNode
	IntentTimer
	IntentUndo
	IntentRedo
	IntentCommand
	numIntents
)

// IntentKinds lists the primitive intent kinds.
func IntentKinds() []IntentKind {
	out := make([]IntentKind, 0, numIntents-1)
	for k := IntentType; k < numIntents; k++ {
		out = append(out, k)
	}
	return out
}

// Primitive reports whether k is an editing primitive, which every
// program answers, rather than a feature's.
func (k IntentKind) Primitive() bool { return k > 0 && k < numIntents }

var intentNames = struct {
	sync.Mutex
	names []string
}{names: []string{"", "Type", "Delete", "Break", "SoftBreak", "Paste", "Cut", "Compose",
	"CancelComposition", "Move", "VisualMove", "Place", "Leave", "Return", "SelectScope", "Dismiss",
	"MoveNode", "Timer", "Undo", "Redo", "Command"}}

// NewIntentKind declares a feature's intent kind. The name is its
// identity: declaring a name again returns the same kind.
func NewIntentKind(name string) IntentKind {
	intentNames.Lock()
	defer intentNames.Unlock()
	for i, n := range intentNames.names {
		if i > 0 && n == name {
			return IntentKind(i)
		}
	}
	if len(intentNames.names) > 255 {
		panic("vocab: too many intent kinds")
	}
	intentNames.names = append(intentNames.names, name)
	return IntentKind(len(intentNames.names) - 1)
}

func (k IntentKind) String() string {
	intentNames.Lock()
	defer intentNames.Unlock()
	if int(k) < len(intentNames.names) {
		return intentNames.names[k]
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
	// ToSlotEdge goes to the caret's line edge, or the slot's edge where the
	// slot keeps no lines; already at a line edge, it goes to the slot's edge.
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

// Composition intents. A commit arrives as Type: composing is view
// state, not a way of entering text of its own.
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
		// Anchor, when set with Extend, is the place the range extends
		// from. A pointer gesture carries its press place here so the
		// engine need not own the gesture in progress.
		Anchor *doc.Caret
		// Cells, when set, is a cell range the pointer picks whole, as a
		// row's or a column's grip does.
		Cells *doc.CellRange
	}
)

// Scope intents.
type (
	Leave       struct{}
	Return      struct{}
	SelectScope struct{}
	Dismiss     struct{}
)

// MoveNode moves a node to a position, as a drag does.
type MoveNode struct {
	ID     doc.ID
	Target doc.Pos
}

// Timer answers a deadline the engine published.
type Timer struct{ Deadline time.Time }

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
func (MoveNode) Kind() IntentKind          { return IntentMoveNode }
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
	case IntentType, IntentDelete, IntentBreak, IntentSoftBreak, IntentCompose, IntentCancelComposition:
		return doc.OriginTyping
	case IntentPaste:
		return doc.OriginPaste
	case IntentPlace:
		return doc.OriginPointer
	case IntentUndo, IntentRedo:
		return doc.OriginHistory
	case IntentTimer:
		return doc.OriginTimer
	}
	return doc.OriginCommand
}

// Directed is an intent that acts toward the caret's next stop in a
// direction: what it crosses there answers it first (§11.7).
type Directed interface {
	Intent
	Toward() Direction
}

// Toward is the delete's direction, backward unless set.
func (d Delete) Toward() Direction {
	if d.Dir == 0 {
		return Backward
	}
	return d.Dir
}

// Toward is backward: Return enters what ends before the caret.
func (Return) Toward() Direction { return Backward }
