package vocab

import "github.com/caelyreth/neverland/editor/doc"

// Edit is a semantic helper's result: primitive operations applied to a
// transaction, reading its state as they go. Handlers return edits; they
// never touch a transaction themselves.
type Edit func(tx *doc.Tx) error

// Ops is an edit of fixed primitive operations.
func Ops(ops ...doc.Op) Edit {
	return func(tx *doc.Tx) error { return tx.ApplyAll(ops...) }
}

// Sequence runs edits in order.
func Sequence(edits ...Edit) Edit {
	return func(tx *doc.Tx) error {
		for _, e := range edits {
			if e == nil {
				continue
			}
			if err := e(tx); err != nil {
				return err
			}
		}
		return nil
	}
}

// Side is a side of a pair.
type Side uint8

const (
	Open Side = iota + 1
	Close
)

// Span is a stretch of one inline slot.
type Span struct {
	Slot     doc.SlotRef
	From, To int
}

// Contains reports whether p is inside the span, its edges included.
func (s Span) Contains(p doc.Pos) bool {
	return p.Ref() == s.Slot && p.Offset >= s.From && p.Offset <= s.To
}

// Role interfaces. A stack entry providing a role implements its
// interface; rules reach it through the match.

// Edge is a stretch between two carets of one slot: a side of a pair, its
// text or its ghost sign, which spans stops but no bytes.
type Edge struct{ From, To doc.Caret }

// Spelling is one way to write a mark or a block marker.
type Spelling string

// Pair has two sides with spellings. For ghost notation EditSide
// reinterprets the spelling through the notation set; for a text pair it
// is a text edit. The policy editing it never knows which.
type Pair interface {
	Sides() (open, close Edge)
	EditSide(side Side, spelling Spelling) Edit
}

// RoleEdits are the records an edit made through a node role may
// produce: a role's edit reshapes, nests, lifts and joins blocks, so a
// rule that delegates to a role declares these, whatever node it holds.
var RoleEdits = []doc.RecordKind{doc.RecordNodeChanged, doc.RecordNodesInserted, doc.RecordNodesRemoved,
	doc.RecordNodeMoved, doc.RecordTransferred, doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks}

// Leveled steps up or down one level: heading levels, emphasis spelling,
// nesting depth.
type Leveled interface {
	Step(dir Direction) Edit
}

// Leavable has an exit for Leave and an entry for Return.
type Leavable interface {
	Exit() doc.Caret
	Entry() doc.Caret
}

// Structure is a whole deleted as a unit.
type Structure interface {
	Node() doc.ID
}

// Role is a shared interface that fact kinds and node kinds provide.
// Rules match on roles, never on providers.
type Role uint8

const (
	RolePair Role = iota + 1
	RoleLeveled
	RoleLeavable
	RoleStructure
)

var roleNames = [...]string{"", "Pair", "Leveled", "Leavable", "Structure"}

func (r Role) String() string { return roleNames[r] }

// Profile is the document profile a program edits: its schema, the roles
// its node kinds provide, and what a slot's place in the structure adds to
// its content. The composition root supplies it; generic code reads kinds
// only through it.
type Profile interface {
	Schema() *doc.Schema
	// Roles are the roles nodes of kind k provide.
	Roles(k doc.Kind) []Role
	// Role is n's implementation of role.
	Role(r doc.Reader, n *doc.Node, role Role) (any, bool)
	// Context is what ref's place in the structure adds to its content,
	// such as the number an item's position gives it, or nil. Analyzers
	// read it, so it keys their caches.
	Context(r doc.Reader, ref doc.SlotRef) any
}
