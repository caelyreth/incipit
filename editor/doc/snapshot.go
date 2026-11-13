package doc

import (
	"errors"
	"fmt"
	"sync/atomic"
)

// Reader reads a document: a published snapshot, or a transaction's
// current state.
type Reader interface {
	Schema() *Schema
	Marks() *Interner
	Root() *Node
	// Node returns the current version of the node id.
	Node(id ID) (*Node, bool)
	// Parent returns where id stands: its parent and the parent's slot.
	// The root has no parent.
	Parent(id ID) (parent ID, slot int, ok bool)
	// Shape names the page's block structure: two readers of one shape
	// hold the same blocks in the same block slots, in the same order,
	// with the same kinds and attributes. Text, marks and inline content
	// leave it as it is; a change to the structure may give a new one.
	Shape() uint64
}

// shapes numbers block structures across every page and transaction.
var shapes atomic.Uint64

func newShape() uint64 { return shapes.Add(1) }

// Snapshot is a published, immutable document: a root pointer, the id
// index and the page's mark-set intern table. Taking one costs nothing.
type Snapshot struct {
	schema *Schema
	marks  *Interner
	root   *Node
	index  *trie
	// seed drives node id generation, so a replay allocates the same ids.
	seed uint64
	// shape is the block structure's (Reader.Shape).
	shape uint64
}

var _ Reader = (*Snapshot)(nil)

// ErrInvalid wraps every validity failure.
var ErrInvalid = errors.New("doc: invalid document")

// NewSnapshot checks a whole tree and indexes it. Every node must carry an
// id, and block children their order keys in order.
func NewSnapshot(schema *Schema, marks *Interner, root *Node, seed uint64) (*Snapshot, error) {
	if root == nil || root.kind != schema.Root() {
		return nil, fmt.Errorf("%w: the root is not a %s", ErrInvalid, schema.KindName(schema.Root()))
	}
	s := &Snapshot{schema: schema, marks: marks, root: root, seed: seed, shape: newShape()}
	var index *trie
	var err error
	var visit func(n *Node, parent ID, slot int, isRoot bool)
	visit = func(n *Node, parent ID, slot int, isRoot bool) {
		if err != nil {
			return
		}
		if n.id == 0 {
			err = fmt.Errorf("%w: a %s has no id", ErrInvalid, schema.KindName(n.kind))
			return
		}
		if _, dup := index.get(n.id); dup {
			err = fmt.Errorf("%w: id %s is used twice", ErrInvalid, n.id)
			return
		}
		index = index.set(n.id, entry{node: n, parent: parent, slot: slot, root: isRoot})
		if verr := validateNode(schema, marks, n); verr != nil {
			err = verr
			return
		}
		for i, sl := range n.slots {
			if sl.inline {
				sl.Content().Nodes(func(_ int, child *Node) bool {
					visit(child, n.id, i, false)
					return err == nil
				})
				continue
			}
			for j, child := range sl.children {
				if child.ord == "" || (j > 0 && sl.children[j-1].ord >= child.ord) {
					err = fmt.Errorf("%w: the children of %s are out of order", ErrInvalid, n.id)
					return
				}
				visit(child, n.id, i, false)
			}
		}
	}
	visit(root, 0, 0, true)
	if err != nil {
		return nil, err
	}
	s.index = index
	return s, nil
}

func (s *Snapshot) Schema() *Schema  { return s.schema }
func (s *Snapshot) Marks() *Interner { return s.marks }
func (s *Snapshot) Root() *Node      { return s.root }
func (s *Snapshot) Shape() uint64    { return s.shape }

// Len is the number of nodes, block and inline.
func (s *Snapshot) Len() int { return s.index.Len() }

func (s *Snapshot) Node(id ID) (*Node, bool) {
	e, ok := s.index.get(id)
	return e.node, ok
}

func (s *Snapshot) Parent(id ID) (ID, int, bool) {
	e, ok := s.index.get(id)
	if !ok || e.root {
		return 0, 0, false
	}
	return e.parent, e.slot, true
}

// Seed is the state of the id stream, which storage keeps with the page.
func (s *Snapshot) Seed() uint64 { return s.seed }

// Ancestors returns id's ancestors, nearest first, ending at the root.
func Ancestors(r Reader, id ID) []ID {
	var out []ID
	for {
		parent, _, ok := r.Parent(id)
		if !ok {
			return out
		}
		out = append(out, parent)
		id = parent
	}
}

// Contains reports whether id is outer or below it.
func Contains(r Reader, outer, id ID) bool {
	for {
		if id == outer {
			return true
		}
		parent, _, ok := r.Parent(id)
		if !ok {
			return false
		}
		id = parent
	}
}
