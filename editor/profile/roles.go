package profile

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Profile is the product profile as the framework reads it.
var Profile vocab.Profile = product{}

type product struct{}

func (product) Schema() *doc.Schema           { return Schema() }
func (product) Roles(k doc.Kind) []vocab.Role { return NodeRoles(k) }
func (product) Role(r doc.Reader, n *doc.Node, role vocab.Role) (any, bool) {
	return NodeRole(r, n, role)
}

// Context is a block's marker, for its first slot: what its structure
// spells before its text.
func (product) Context(r doc.Reader, ref doc.SlotRef) any {
	if ref.Slot != 0 {
		return nil
	}
	n, ok := r.Node(ref.Node)
	if !ok {
		return nil
	}
	if m, ok := MarkerOf(r, n); ok {
		return m
	}
	return nil
}

// NodeRoles lists the roles a node kind provides.
func NodeRoles(k doc.Kind) []vocab.Role {
	switch k {
	case Heading:
		return []vocab.Role{vocab.RoleLeveled}
	case List, Quote:
		return []vocab.Role{vocab.RoleStructure}
	case Item:
		return []vocab.Role{vocab.RoleLeveled, vocab.RoleContinued}
	case Formula:
		return []vocab.Role{vocab.RoleLeavable, vocab.RoleStructure}
	case Cell:
		return []vocab.Role{vocab.RoleLeavable}
	case Math, Code:
		return []vocab.Role{vocab.RoleLeveled, vocab.RoleStructure}
	case Table, Figure, Divider:
		return []vocab.Role{vocab.RoleStructure}
	}
	return nil
}

// NodeRole returns n's implementation of role.
func NodeRole(r doc.Reader, n *doc.Node, role vocab.Role) (any, bool) {
	switch role {
	case vocab.RoleLeveled:
		switch n.Kind() {
		case Heading:
			return headingLevel{id: n.ID(), level: HeadingLevel(n)}, true
		case Item:
			return itemLevel(n.ID()), true
		case Code, Math:
			return plainLevel(n.ID()), true
		}
	case vocab.RoleContinued:
		if n.Kind() == Item {
			return itemNext{task: TaskOf(n)}, true
		}
	case vocab.RoleLeavable:
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
	case vocab.RoleStructure:
		for _, provided := range NodeRoles(n.Kind()) {
			if provided == vocab.RoleStructure {
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
func (h headingLevel) Step(dir vocab.Direction) vocab.Edit {
	next := h.level + int(dir)
	switch {
	case next < 1:
		return vocab.Ops(doc.SetNode{ID: h.id, Kind: Paragraph})
	case next > 6:
		next = 6
	}
	return vocab.Ops(doc.SetNode{ID: h.id, Kind: Heading, Attrs: doc.MakeAttrs(doc.Field{Num: FieldLevel, Value: doc.Int(int64(next))})})
}

// itemLevel is an item's nesting depth.
type itemLevel doc.ID

// Step nests the item one level deeper, into the item before it
// (Forward), or one level out (Backward): out of a nested list to follow
// the item holding it, and out of a top-level list as a paragraph.
func (i itemLevel) Step(dir vocab.Direction) vocab.Edit {
	id := doc.ID(i)
	if dir > 0 {
		return func(tx *doc.Tx) error {
			if _, _, index, ok := vocab.Index(tx, id); !ok || index == 0 {
				return nil
			}
			return vocab.Sink(id)(tx)
		}
	}
	return vocab.Outdent(id)
}

// plainLevel is a block one level above a paragraph: stepping it down
// makes it one, keeping its text; nothing is above it.
type plainLevel doc.ID

func (p plainLevel) Step(dir vocab.Direction) vocab.Edit {
	if dir > 0 {
		return vocab.Sequence()
	}
	return vocab.Unmark(doc.ID(p))
}

// itemNext continues a list: an item after an item, an open task after a
// task.
type itemNext struct{ task Task }

func (i itemNext) Next() (doc.Kind, doc.Attrs) {
	if i.task != NoTask {
		return Item, doc.MakeAttrs(doc.Field{Num: FieldTask, Value: doc.Int(int64(TaskOpen))})
	}
	return Item, doc.Attrs{}
}

type leavable struct{ exit, entry doc.Caret }

func (l leavable) Exit() doc.Caret  { return l.exit }
func (l leavable) Entry() doc.Caret { return l.entry }

type structure doc.ID

func (s structure) Node() doc.ID { return doc.ID(s) }
