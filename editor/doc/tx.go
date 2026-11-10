package doc

import (
	"errors"
	"fmt"
	"slices"
)

// Tx is a transaction: operations applied to a transient copy of a
// snapshot. Nodes the transaction created or copied are changed in place;
// everything it did not touch stays shared with the snapshot.
type Tx struct {
	base   *Snapshot
	schema *Schema
	marks  *Interner
	root   *Node
	index  *trie
	seed   uint64

	fresh   map[*Node]bool
	touched map[ID]bool
	slots   map[SlotRef]bool

	origin  Origin
	policy  string
	steps   []*Step
	journal Journal
}

var _ Reader = (*Tx)(nil)

// Begin starts a transaction on s.
func (s *Snapshot) Begin(origin Origin) *Tx {
	return &Tx{
		base: s, schema: s.schema, marks: s.marks, root: s.root, index: s.index, seed: s.seed,
		fresh: map[*Node]bool{}, touched: map[ID]bool{}, slots: map[SlotRef]bool{},
		origin: origin,
	}
}

func (tx *Tx) Schema() *Schema  { return tx.schema }
func (tx *Tx) Marks() *Interner { return tx.marks }
func (tx *Tx) Root() *Node      { return tx.root }
func (tx *Tx) Base() *Snapshot  { return tx.base }

func (tx *Tx) Node(id ID) (*Node, bool) {
	e, ok := tx.index.get(id)
	return e.node, ok
}

func (tx *Tx) Parent(id ID) (ID, int, bool) {
	e, ok := tx.index.get(id)
	if !ok || e.root {
		return 0, 0, false
	}
	return e.parent, e.slot, true
}

// SetOrigin sets the origin of the operations applied from now on, as the
// transaction passes from the handler to each rewrite phase.
func (tx *Tx) SetOrigin(o Origin) { tx.origin = o }

// Origin is the current origin.
func (tx *Tx) Origin() Origin { return tx.origin }

// SetPolicy names the policy whose operations follow, for traces.
func (tx *Tx) SetPolicy(name string) { tx.policy = name }

// Steps returns the applied steps.
func (tx *Tx) Steps() []*Step { return tx.steps }

// Journal returns the records so far.
func (tx *Tx) Journal() *Journal { return &tx.journal }

// Note adds a record that is not an operation, such as a selection change.
func (tx *Tx) Note(r Record) {
	r.Origin = tx.origin
	r.Policy = tx.policy
	tx.journal.Add(r)
}

// NewID allocates an id no node of the page holds. The stream is part of
// the snapshot, so a replay allocates the same ids.
func (tx *Tx) NewID() ID {
	for {
		tx.seed += 0x9E3779B97F4A7C15
		z := tx.seed
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		id := ID(z ^ (z >> 31))
		if _, used := tx.index.get(id); id != 0 && !used {
			return id
		}
	}
}

// Apply applies one operation. A failed operation leaves the transaction
// as it was.
func (tx *Tx) Apply(op Op) error {
	step, err := op.apply(tx)
	if err != nil {
		return fmt.Errorf("doc: %s: %w", op, err)
	}
	// Journal ranges stand in the document at the end of the transaction,
	// not in the intermediate state where a step made them. Observers and
	// strict output checks read that one version of the records.
	for i := range tx.journal.Records {
		tx.journal.Records[i] = mapRecord(tx.journal.Records[i], step.mapping)
	}
	step.origin = tx.origin
	for i := range step.records {
		step.records[i].Origin = tx.origin
		step.records[i].Policy = tx.policy
		tx.journal.Add(step.records[i])
	}
	tx.steps = append(tx.steps, step)
	return nil
}

// mapRecord carries a record's places through a later step.
func mapRecord(r Record, m Mapper) Record {
	switch r.Kind {
	case RecordTextInserted, RecordTextDeleted, RecordMarks, RecordNodesInserted:
		return mapRecordRange(r, m)
	case RecordTransferred:
		r = mapRecordRange(r, m)
		if at, ok := m.Map(At(r.Target, r.At), BiasAfter); ok {
			r.Target, r.At = at.Ref(), at.Offset
		}
	case RecordNodeMoved:
		if at, ok := m.Map(At(r.Target, r.At), BiasAfter); ok {
			r.Target, r.At = at.Ref(), at.Offset
		}
	default:
		return r
	}
	return r
}

func mapRecordRange(r Record, m Mapper) Record {
	from, ok1 := m.Map(At(r.Slot, r.From), BiasAfter)
	to, ok2 := m.Map(At(r.Slot, r.To), BiasBefore)
	if !ok1 || !ok2 || to.Ref() != from.Ref() {
		return r
	}
	r.Slot, r.From, r.To = from.Ref(), from.Offset, to.Offset
	return r
}

// ApplyAll applies operations in order, stopping at the first failure.
func (tx *Tx) ApplyAll(ops ...Op) error {
	for _, op := range ops {
		if err := tx.Apply(op); err != nil {
			return err
		}
	}
	return nil
}

// Commit normalises the inline slots the transaction touched, validates
// every node it touched, and returns the new snapshot. An invalid
// transaction returns an error and publishes nothing.
func (tx *Tx) Commit() (*Snapshot, error) {
	for ref := range tx.slots {
		n, ok := tx.Node(ref.Node)
		if !ok {
			continue
		}
		s := &n.slots[ref.Slot]
		s.content = s.Content().normalized(tx.marks.Empty())
	}
	for id := range tx.touched {
		n, ok := tx.Node(id)
		if !ok {
			continue
		}
		if err := validateNode(tx.schema, tx.marks, n); err != nil {
			return nil, err
		}
	}
	for n := range tx.fresh {
		for i := range n.slots {
			n.slots[i].owned = false
		}
	}
	tx.fresh = nil
	return &Snapshot{schema: tx.schema, marks: tx.marks, root: tx.root, index: tx.index, seed: tx.seed}, nil
}

// errors operations report.
var (
	ErrNoNode    = errors.New("no such node")
	ErrSlot      = errors.New("no such slot, or the wrong type of slot")
	ErrRange     = errors.New("range outside the slot")
	ErrStructure = errors.New("the change breaks the tree")
)

func (tx *Tx) node(id ID) (*Node, error) {
	n, ok := tx.Node(id)
	if !ok {
		return nil, fmt.Errorf("%w %s", ErrNoNode, id)
	}
	return n, nil
}

func (tx *Tx) inlineSlot(ref SlotRef) (*Node, *Inline, error) {
	n, err := tx.node(ref.Node)
	if err != nil {
		return nil, nil, err
	}
	if ref.Slot < 0 || ref.Slot >= len(n.slots) || !n.slots[ref.Slot].inline {
		return nil, nil, fmt.Errorf("%w %s", ErrSlot, ref)
	}
	return n, n.slots[ref.Slot].Content(), nil
}

func (tx *Tx) blockSlot(ref SlotRef) (*Node, Slot, error) {
	n, err := tx.node(ref.Node)
	if err != nil {
		return nil, Slot{}, err
	}
	if ref.Slot < 0 || ref.Slot >= len(n.slots) || n.slots[ref.Slot].inline {
		return nil, Slot{}, fmt.Errorf("%w %s", ErrSlot, ref)
	}
	return n, n.slots[ref.Slot], nil
}

// mutable returns a version of id the transaction owns, copying it and the
// path above it on first use.
func (tx *Tx) mutable(id ID) *Node {
	e, _ := tx.index.get(id)
	n := e.node
	if tx.fresh[n] {
		return n
	}
	c := n.clone()
	tx.fresh[c] = true
	if e.root {
		tx.root = c
	} else {
		p := tx.mutable(e.parent)
		s := &p.slots[e.slot]
		if s.inline {
			s.content = s.Content().withNode(id, c)
		} else {
			i, _ := s.IndexOf(n)
			tx.ownChildren(s)
			s.children[i] = c
		}
	}
	e.node = c
	tx.index = tx.index.set(id, e)
	return c
}

func (tx *Tx) ownChildren(s *Slot) {
	if !s.owned {
		s.children = slices.Clone(s.children)
		s.owned = true
	}
}

// setContent replaces an inline slot's content.
func (tx *Tx) setContent(ref SlotRef, in *Inline) {
	n := tx.mutable(ref.Node)
	n.slots[ref.Slot].content = in
	tx.slots[ref] = true
	tx.touched[ref.Node] = true
}

// register indexes a subtree placed at parent's slot.
func (tx *Tx) register(n *Node, parent ID, slot int) {
	tx.index = tx.index.set(n.id, entry{node: n, parent: parent, slot: slot})
	tx.touched[n.id] = true
	for i, s := range n.slots {
		if s.inline {
			s.Content().Nodes(func(_ int, child *Node) bool {
				tx.register(child, n.id, i)
				return true
			})
			continue
		}
		for _, child := range s.children {
			tx.register(child, n.id, i)
		}
	}
}

// unregister drops a subtree from the index, reporting each id.
func (tx *Tx) unregister(n *Node, each func(ID)) {
	n.Walk(func(d *Node) bool {
		tx.index = tx.index.delete(d.id)
		if each != nil {
			each(d.id)
		}
		return true
	})
}

// prepare makes a subtree ready to enter the document: new nodes (id 0)
// get ids, and children without increasing order keys get them. Nodes that
// already have ids are taken as they are, and must not be in the page.
func (tx *Tx) prepare(n *Node) (*Node, error) {
	if n.id != 0 {
		var err error
		n.Walk(func(d *Node) bool {
			if _, used := tx.index.get(d.id); used && err == nil {
				err = fmt.Errorf("%w: id %s is already in the page", ErrStructure, d.id)
			}
			return err == nil
		})
		return n, err
	}
	// The caller's node stays as it was: a slice can be pasted twice.
	n = n.clone()
	n.id = tx.NewID()
	tx.fresh[n] = true
	for i := range n.slots {
		s := &n.slots[i]
		if s.inline {
			var err error
			items := make([]Item, 0)
			s.Content().Each(func(_ int, it Item) bool {
				if it.Node != nil {
					it.Node, err = tx.prepare(it.Node)
				} else {
					it.Marks = tx.own(it.Marks)
				}
				items = append(items, it)
				return err == nil
			})
			if err != nil {
				return nil, err
			}
			s.content = NewInline(items...)
			tx.slots[SlotRef{Node: n.id, Slot: i}] = true
			continue
		}
		tx.ownChildren(s)
		ordered := true
		for j, child := range s.children {
			prepared, err := tx.prepare(child)
			if err != nil {
				return nil, err
			}
			s.children[j] = prepared
			if prepared.ord == "" || (j > 0 && s.children[j-1].ord >= prepared.ord) {
				ordered = false
			}
		}
		if !ordered {
			for j, key := range ordSpread(len(s.children)) {
				s.children[j] = tx.withOrd(s.children[j], key)
			}
		}
	}
	return n, nil
}

// own returns the page's own interned set for marks, which may come from
// another page, as pasted content does.
func (tx *Tx) own(marks *MarkSet) *MarkSet {
	if marks == nil {
		return tx.marks.Empty()
	}
	if tx.marks.Owns(marks) {
		return marks
	}
	return tx.marks.Intern(marks.Marks())
}

// withOrd returns child carrying key, copied when the transaction does not
// own it.
func (tx *Tx) withOrd(child *Node, key string) *Node {
	if child.ord == key {
		return child
	}
	if !tx.fresh[child] {
		child = child.clone()
		tx.fresh[child] = true
	}
	child.ord = key
	return child
}

// rekey spreads a slot's order keys again once one grew past its limit.
func (tx *Tx) rekey(parent *Node, slot int) {
	s := &parent.slots[slot]
	tx.ownChildren(s)
	for j, key := range ordSpread(len(s.children)) {
		child := tx.withOrd(s.children[j], key)
		s.children[j] = child
		e, _ := tx.index.get(child.id)
		e.node = child
		tx.index = tx.index.set(child.id, e)
	}
}

// place inserts prepared nodes into a block slot at index, giving them
// order keys between their neighbours.
func (tx *Tx) place(parent *Node, slot, index int, nodes []*Node) []*Node {
	s := &parent.slots[slot]
	tx.ownChildren(s)
	low := ""
	if index > 0 {
		low = s.children[index-1].ord
	}
	high := ""
	if index < len(s.children) {
		high = s.children[index].ord
	}
	placed := make([]*Node, len(nodes))
	long := false
	for i, n := range nodes {
		key := n.ord
		if key == "" || key <= low || (high != "" && key >= high) {
			key = ordBetween(low, high)
		}
		n = tx.withOrd(n, key)
		placed[i] = n
		low = key
		long = long || len(key) > ordLimit
	}
	s.children = slices.Insert(s.children, index, placed...)
	if long {
		tx.rekey(parent, slot)
		copy(placed, s.children[index:index+len(placed)])
	}
	return placed
}

// withNode returns content with the inline node id replaced by n.
func (in *Inline) withNode(id ID, n *Node) *Inline {
	out := &Inline{text: in.text, size: in.size, runs: slices.Clone(in.runs)}
	for i, r := range out.runs {
		if r.node != nil && r.node.id == id {
			out.runs[i].node = n
		}
	}
	return out
}
