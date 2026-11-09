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

// Reshapeable changes kind through a typed glyph.
type Reshapeable interface {
	Reshape(glyph string) Edit
}

// Box is a toggled state in a marker.
type Box interface {
	Toggle() Edit
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

// ItemEntry is a container entry Enter, Backspace and nesting act on.
type ItemEntry interface {
	Item() doc.ID
}

// NodeRoles lists the roles a node kind provides.
func NodeRoles(k doc.Kind) []Role {
	switch k {
	case Heading:
		return []Role{RoleLeveled}
	case List, Quote:
		return []Role{RoleLeveled, RoleStructure}
	case Item:
		return []Role{RoleItem, RoleLeveled}
	case Formula:
		return []Role{RoleLeavable, RoleStructure}
	case Cell:
		return []Role{RoleLeavable}
	case Table, Figure, Divider, Math, Code:
		return []Role{RoleStructure}
	}
	return nil
}

// NodeRole returns n's implementation of role.
func NodeRole(r doc.Reader, n *doc.Node, role Role) (any, bool) {
	switch role {
	case RoleLeveled:
		switch n.Kind() {
		case Heading:
			return headingLevel{id: n.ID(), level: HeadingLevel(n)}, true
		case Item:
			return itemLevel(n.ID()), true
		}
	case RoleItem:
		if n.Kind() == Item {
			return itemEntry(n.ID()), true
		}
	case RoleLeavable:
		switch n.Kind() {
		case Formula:
			parent, slot, _ := r.Parent(n.ID())
			pn, _ := r.Node(parent)
			at, _ := pn.Slot(slot).Content().NodeOffset(n.ID())
			return leavable{
				exit:  doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: at + 1}),
				entry: doc.CaretAt(doc.Pos{Node: n.ID(), Offset: n.Slot(0).Len()}),
			}, true
		case Cell:
			table := doc.Ancestors(r, n.ID())
			if len(table) >= 2 {
				t := table[1]
				parent, slot, _ := r.Parent(t)
				pn, _ := r.Node(parent)
				tn, _ := r.Node(t)
				i, _ := pn.Slot(slot).IndexOf(tn)
				return leavable{
					exit:  doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: i + 1}),
					entry: doc.CaretAt(doc.Pos{Node: n.ID(), Offset: n.Slot(0).Len()}),
				}, true
			}
		}
	case RoleStructure:
		for _, provided := range NodeRoles(n.Kind()) {
			if provided == RoleStructure {
				return structure(n.ID()), true
			}
		}
	}
	return nil, false
}

type headingLevel struct {
	id    doc.ID
	level int
}

// Step moves a heading one level, as one more "#" (Forward) or one fewer
// (Backward); below level 1 it becomes a paragraph, and 6 is the deepest.
func (h headingLevel) Step(dir Direction) Edit {
	next := h.level + int(dir)
	switch {
	case next < 1:
		return Ops(doc.SetNode{ID: h.id, Kind: Paragraph})
	case next > 6:
		next = 6
	}
	return Ops(doc.SetNode{ID: h.id, Kind: Heading, Attrs: doc.MakeAttrs(doc.Field{Num: FieldLevel, Value: doc.Int(int64(next))})})
}

// itemLevel is an item's nesting depth.
type itemLevel doc.ID

// Step nests the item one level deeper, into the item before it
// (Forward), or one level out (Backward): out of a nested list to follow
// the item holding it, and out of a top-level list as a paragraph.
func (i itemLevel) Step(dir Direction) Edit {
	id := doc.ID(i)
	if dir > 0 {
		return func(tx *doc.Tx) error {
			if _, _, index, ok := Index(tx, id); !ok || index == 0 {
				return nil
			}
			return Sink(id)(tx)
		}
	}
	return Outdent(id)
}

type itemEntry doc.ID

func (i itemEntry) Item() doc.ID { return doc.ID(i) }

type leavable struct{ exit, entry doc.Caret }

func (l leavable) Exit() doc.Caret  { return l.exit }
func (l leavable) Entry() doc.Caret { return l.entry }

type structure doc.ID

func (s structure) Node() doc.ID { return doc.ID(s) }
