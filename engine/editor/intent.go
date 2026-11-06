package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// An intent is what a person means, with no key in it: every keystroke,
// typed or composed text, paste, menu item and toolbar button is one. The
// keymap and the host translate input into intents; the session resolves
// each against the scopes at the caret (place.go) and applies what answers.
// Nothing about a key, a kind or a policy's order is in an intent.

// IntentKind is what an intent asks for.
type IntentKind uint8

const (
	// IntentInsert writes Text at the selection, or at Range when set.
	IntentInsert IntentKind = iota + 1
	// IntentDelete removes one By in Dir, or the selection.
	IntentDelete
	// IntentMove moves the caret, or its head when Extend, one By in Dir.
	IntentMove
	// IntentSelect selects the By around the selection.
	IntentSelect
	// IntentDismiss lets go of what the selection holds, or the scope it
	// stands in.
	IntentDismiss
	// IntentBreak ends the line; Soft breaks within the block.
	IntentBreak
	// IntentLeave goes out of the innermost scope at the caret.
	IntentLeave
	// IntentReturn goes back into the scope just left, or the one before
	// the caret.
	IntentReturn
	// IntentHistory steps the page's history in Dir: Backward undoes.
	IntentHistory
	// IntentFormat toggles emphasis of Level over the selection.
	IntentFormat
	// IntentNest moves the blocks selected a level in Dir: Forward deeper.
	IntentNest
	// IntentTable runs Table on the table at the caret.
	IntentTable
)

// Reach is how much of the page an intent moves over, deletes or selects.
type Reach uint8

const (
	ByGrapheme Reach = iota + 1
	ByWord
	BySentence
	ByParagraph
	ByLine
	// ByRow is a drawn row, which only the page's geometry knows.
	ByRow
	ByScope
	ByPage
)

// Direction is the way an intent goes along the page.
type Direction int8

const (
	Backward Direction = -1
	Forward  Direction = 1
)

// Intent is one thing a person means.
type Intent struct {
	Kind   IntentKind
	By     Reach
	Dir    Direction
	Extend bool
	Soft   bool
	Level  uint8
	Table  TableOp
	Text   string
	Origin kernel.EventOrigin
	// Range, when set, is the projected range an Insert replaces instead
	// of the selection.
	Range *model.UTF16Range
}

// Insert is text written at the selection.
func Insert(text string, origin kernel.EventOrigin) Intent {
	return Intent{Kind: IntentInsert, Text: text, Origin: origin}
}

// InsertAt is text written over a projected range.
func InsertAt(range16 model.UTF16Range, text string, origin kernel.EventOrigin) Intent {
	return Intent{Kind: IntentInsert, Text: text, Origin: origin, Range: &range16}
}

// Delete is removing one unit in a direction.
func Delete(by Reach, dir Direction) Intent {
	return Intent{Kind: IntentDelete, By: by, Dir: dir}
}

// Move is moving the caret one unit in a direction, or its head when extend.
func Move(by Reach, dir Direction, extend bool) Intent {
	return Intent{Kind: IntentMove, By: by, Dir: dir, Extend: extend}
}

// SelectUnit is selecting the unit around the selection.
func SelectUnit(by Reach) Intent { return Intent{Kind: IntentSelect, By: by} }

// Simple intents with nothing more to say.
var (
	Dismiss   = Intent{Kind: IntentDismiss}
	Break     = Intent{Kind: IntentBreak}
	SoftBreak = Intent{Kind: IntentBreak, Soft: true}
	Leave     = Intent{Kind: IntentLeave}
	Return    = Intent{Kind: IntentReturn}
	Undo      = Intent{Kind: IntentHistory, Dir: Backward}
	Redo      = Intent{Kind: IntentHistory, Dir: Forward}
)

// Format is toggling emphasis of a level: 1 emphasis, 2 strong.
func Format(level uint8) Intent { return Intent{Kind: IntentFormat, Level: level} }

// Nest is moving the selected blocks a level deeper (Forward) or out.
func Nest(dir Direction) Intent { return Intent{Kind: IntentNest, Dir: dir} }

// TableIntent is running an operation on the table at the caret.
func TableIntent(op TableOp) Intent { return Intent{Kind: IntentTable, Table: op} }

// Key is the trigger key responders to this intent declare.
func (i Intent) Key() string { return intentKey(i.Kind) }

func intentKey(kind IntentKind) string { return "intent:" + kind.String() }

// String is the intent kind's stable name.
func (k IntentKind) String() string {
	switch k {
	case IntentInsert:
		return "insert"
	case IntentDelete:
		return "delete"
	case IntentMove:
		return "move"
	case IntentSelect:
		return "select"
	case IntentDismiss:
		return "dismiss"
	case IntentBreak:
		return "break"
	case IntentLeave:
		return "leave"
	case IntentReturn:
		return "return"
	case IntentHistory:
		return "history"
	case IntentFormat:
		return "format"
	case IntentNest:
		return "nest"
	case IntentTable:
		return "table"
	default:
		return "none"
	}
}

// Repeats reports whether a held key may repeat the intent: motion,
// deletion and history do; toggles and structure do not.
func (i Intent) Repeats() bool {
	switch i.Kind {
	case IntentMove, IntentDelete, IntentHistory, IntentInsert, IntentBreak, IntentSelect, IntentDismiss:
		return true
	default:
		return false
	}
}
