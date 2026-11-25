package doc

import (
	"slices"
	"strings"
)

// Slot is a node's typed content area: ordered block children, or inline
// content. The node's kind says which. Block children are one slice while
// they are few, and chunked above chunkAt (blocks.go).
type Slot struct {
	inline   bool
	children []*Node
	big      *chunked
	content  *Inline
	// owned marks children private to a transaction, which may replace
	// them in place, each where it stands. A published slot is never
	// owned.
	owned bool
}

// splice replaces del children from i with ins, in new children: a slice
// once read from a slot never shifts under whoever holds it, even within
// the transaction changing the slot, so a loop over a slot's children may
// move them out of it.
func (s *Slot) splice(tx uint64, i, del int, ins ...*Node) {
	s.owned = true
	if s.big == nil {
		s.children = slices.Concat(s.children[:i:i], ins, s.children[i+del:])
		if len(s.children) > chunkAt {
			s.big, s.children = chunkNodes(s.children, tx), nil
		}
		return
	}
	s.big = s.big.splice(tx, i, del, ins)
	if s.big.len() < chunkAt/2 {
		s.children, s.big = slices.Clone(s.big.all()), nil
	}
}

// set replaces child i where it stands.
func (s *Slot) set(tx uint64, i int, n *Node) {
	if s.big == nil {
		if !s.owned {
			s.children = slices.Clone(s.children)
		}
		s.children[i] = n
	} else {
		if !s.owned {
			s.big = s.big.own()
		}
		s.big.set(tx, i, n)
	}
	s.owned = true
}

// setAll replaces every child.
func (s *Slot) setAll(tx uint64, nodes []*Node) {
	s.children, s.big, s.owned = nodes, nil, true
	if len(nodes) > chunkAt {
		s.big, s.children = chunkNodes(nodes, tx), nil
	}
}

// BlockSlot holds block children.
func BlockSlot(children ...*Node) Slot {
	var s Slot
	s.setAll(0, slices.Clone(children))
	s.owned = false
	return s
}

// InlineSlot holds inline items.
func InlineSlot(items ...Item) Slot {
	return Slot{inline: true, content: NewInline(items...)}
}

// IsInline reports whether the slot holds inline content.
func (s Slot) IsInline() bool { return s.inline }

// Children returns a block slot's children. The slice must not change, and
// no edit shifts it: one inserting or removing children makes a new one.
// A slot with many children makes the slice once per version; Child and
// Len read them without it.
func (s Slot) Children() []*Node {
	if s.big != nil {
		return s.big.all()
	}
	return s.children
}

// Child returns a block slot's i-th child.
func (s Slot) Child(i int) *Node {
	if s.big != nil {
		return s.big.at(i)
	}
	return s.children[i]
}

// each calls fn with each child in order, until fn returns false, without
// making one slice of them.
func (s Slot) each(fn func(i int, n *Node) bool) {
	if s.big == nil {
		for i, n := range s.children {
			if !fn(i, n) {
				return
			}
		}
		return
	}
	i := 0
	for _, ch := range s.big.chunks {
		for _, n := range ch.nodes {
			if !fn(i, n) {
				return
			}
			i++
		}
	}
}

// Content returns an inline slot's content.
func (s Slot) Content() *Inline {
	if s.content == nil {
		return emptyInline
	}
	return s.content
}

// Len is the number of children, or of inline positions.
func (s Slot) Len() int {
	if s.inline {
		return s.Content().Len()
	}
	if s.big != nil {
		return s.big.len()
	}
	return len(s.children)
}

// IndexOf returns where child stands, found by its order key.
func (s Slot) IndexOf(child *Node) (int, bool) {
	var i int
	var ok bool
	if s.big != nil {
		i, ok = s.big.indexOrd(child.ord)
	} else {
		i, ok = slices.BinarySearchFunc(s.children, child.ord, func(n *Node, ord string) int {
			return strings.Compare(n.ord, ord)
		})
	}
	if ok && s.Child(i).id == child.id {
		return i, true
	}
	at, found := 0, false
	s.each(func(j int, n *Node) bool {
		at, found = j, n.id == child.id
		return !found
	})
	if !found {
		return 0, false
	}
	return at, true
}

// Node is one node of the tree. It is immutable once published.
type Node struct {
	id    ID
	kind  Kind
	attrs Attrs
	slots []Slot
	// ord is the node's fractional order key among its siblings in a block
	// slot. Inline nodes have none.
	ord string
}

// NewNode builds a node to insert. Its id and order key are given when it
// is inserted.
func NewNode(kind Kind, attrs Attrs, slots ...Slot) *Node {
	return &Node{kind: kind, attrs: attrs, slots: slots}
}

// NewNodeWithID builds a node with a known id, as storage does.
func NewNodeWithID(id ID, kind Kind, attrs Attrs, ord string, slots ...Slot) *Node {
	return &Node{id: id, kind: kind, attrs: attrs, slots: slots, ord: ord}
}

func (n *Node) ID() ID          { return n.id }
func (n *Node) Kind() Kind      { return n.kind }
func (n *Node) Attrs() Attrs    { return n.attrs }
func (n *Node) Ord() string     { return n.ord }
func (n *Node) NumSlots() int   { return len(n.slots) }
func (n *Node) Slot(i int) Slot { return n.slots[i] }

// Slots returns the node's slots. The slice must not change.
func (n *Node) Slots() []Slot { return n.slots }

// clone is a shallow copy whose slots may then be replaced; the children
// slices stay shared until a transaction takes ownership of one.
func (n *Node) clone() *Node {
	c := *n
	c.slots = slices.Clone(n.slots)
	for i := range c.slots {
		c.slots[i].owned = false
	}
	return &c
}

// Text is the plain text of the node's inline slots, in reading order,
// including the slots of nested inline nodes.
func (n *Node) Text() string {
	var b strings.Builder
	n.writeText(&b)
	return b.String()
}

func (n *Node) writeText(b *strings.Builder) {
	for _, s := range n.slots {
		if !s.inline {
			continue
		}
		s.Content().Each(func(_ int, it Item) bool {
			if it.Node != nil {
				it.Node.writeText(b)
			} else {
				b.WriteString(it.Text)
			}
			return true
		})
	}
}

// Walk calls fn with n and every node below it, block and inline, parents
// before children. fn returning false skips the node's descendants.
func (n *Node) Walk(fn func(n *Node) bool) {
	if !fn(n) {
		return
	}
	for _, s := range n.slots {
		if s.inline {
			s.Content().Nodes(func(_ int, child *Node) bool {
				child.Walk(fn)
				return true
			})
			continue
		}
		s.each(func(_ int, child *Node) bool {
			child.Walk(fn)
			return true
		})
	}
}
