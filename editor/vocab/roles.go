package vocab

import (
	"reflect"

	"github.com/caelyreth/neverland/editor/doc"
)

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

// Continued is a block a break at its end continues: the kind and
// attributes of the block opened after it. A block without it is
// continued by a plain block.
type Continued interface {
	Next() (doc.Kind, doc.Attrs)
}

// Closer is a scope a typed character closes: typed at its inner end, the
// character is the closing side already there, and the caret steps over
// it. Which characters close it is the provider's: a pair's closer, any
// delimiter of a mark's kind.
type Closer interface {
	Closes(text string) bool
}

// Syntax is text still spelling structure to come, such as a rule
// waiting for Enter. Rewrites that spare it leave it as typed.
type Syntax interface {
	Syntax() Span
}

// Structure is a whole: what is deleted as one is first shown as one.
// Each provider says what it is: a node, a pair's two sides, a mark's
// signs.
type Structure interface {
	// Extent is what the whole covers, as it is shown: its pieces, each
	// between two carets.
	Extent() []Edge
	// Remove is the edit deleting the whole: a node goes, a pair's sides
	// go and what they held stays.
	Remove() Edit
}

// Spanned is an object whose text is laid out in a shape of its own, such
// as a table's grid: a range between two of its text slots selects in that
// shape, as a cell range does, and what such a selection copies keeps it.
type Spanned interface {
	Span(anchor, head doc.Caret) (doc.Selection, bool)
	Copy(sel doc.Selection) (Slice, bool)
}

// RoleID identifies a role, as declarations list it.
type RoleID uint8

// Role is a shared interface, T, that fact kinds and node kinds provide.
// Rules match on roles, never on providers, and bind a role's provider as
// its interface, so a rule cannot bind the wrong one.
type Role[T any] struct{ ID RoleID }

// AnyRole is a role of any interface, as declarations list them.
type AnyRole interface{ RoleID() RoleID }

func (r Role[T]) RoleID() RoleID { return r.ID }
func (r Role[T]) String() string { return r.ID.String() }
func (r RoleID) RoleID() RoleID  { return r }

var roles [9]struct {
	name string
	typ  reflect.Type
}

func newRole[T any](id RoleID, name string) Role[T] {
	roles[id].name, roles[id].typ = name, reflect.TypeFor[T]()
	return Role[T]{ID: id}
}

func (r RoleID) String() string { return roles[r].name }

// Type is the interface the role's providers implement.
func (r RoleID) Type() reflect.Type { return roles[r].typ }

// The roles.
var (
	RolePair      = newRole[Pair](1, "Pair")
	RoleLeveled   = newRole[Leveled](2, "Leveled")
	RoleLeavable  = newRole[Leavable](3, "Leavable")
	RoleStructure = newRole[Structure](4, "Structure")
	RoleContinued = newRole[Continued](5, "Continued")
	RoleCloser    = newRole[Closer](6, "Closer")
	RoleSyntax    = newRole[Syntax](7, "Syntax")
	RoleSpanned   = newRole[Spanned](8, "Spanned")
)

// Profile is the document profile a program edits: its schema, the roles
// its node kinds provide, and what a slot's place in the structure adds to
// its content. The composition root supplies it; generic code reads kinds
// only through it.
type Profile interface {
	Schema() *doc.Schema
	// Roles are the roles nodes of kind k provide.
	Roles(k doc.Kind) []RoleID
	// Role is n's implementation of role.
	Role(r doc.Reader, n *doc.Node, role RoleID) (any, bool)
	// Context is what ref's place in the structure adds to its content,
	// such as the number an item's position gives it, or nil. Analyzers
	// read it, so it keys their caches.
	Context(r doc.Reader, ref doc.SlotRef) any
}
