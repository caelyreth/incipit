package doc

import (
	"fmt"
	"slices"
)

// Op is a primitive operation. Applying one captures what its inverse
// needs, and defines how positions map through it.
type Op interface {
	apply(tx *Tx) (*Step, error)
	// rebase carries the operation through a later change, for undo over
	// other origins; false when its target is gone.
	rebase(m Mapper) (Op, bool)
	String() string
}

// Step is an applied operation: the operation, its inverse, its mapping
// and the records it produced.
type Step struct {
	op      Op
	inverse []Op
	mapping Mapper
	records []Record
	origin  Origin
	// slots and nodes are what the step touched, for rebasing.
	slots []SlotRef
	nodes []ID
}

func (s *Step) Op() Op            { return s.op }
func (s *Step) Inverse() []Op     { return s.inverse }
func (s *Step) Records() []Record { return s.records }
func (s *Step) Origin() Origin    { return s.origin }

// Map maps a position through the step.
func (s *Step) Map(p Pos, bias Bias) (Pos, bool) { return s.mapping.Map(p, bias) }

// Touches reports whether the step changed the slot or node.
func (s *Step) Touches(ref SlotRef, node ID) bool {
	return slices.Contains(s.slots, ref) || (node != 0 && slices.Contains(s.nodes, node))
}

// Steps maps through a sequence of steps.
type Steps []*Step

func (ss Steps) Map(p Pos, bias Bias) (Pos, bool) {
	alive := true
	for _, s := range ss {
		var ok bool
		p, ok = s.Map(p, bias)
		alive = alive && ok
	}
	return p, alive
}

type identity struct{}

func (identity) Map(p Pos, _ Bias) (Pos, bool) { return p, true }

// removedMap sends positions inside removed nodes to where they were.
type removedMap map[ID]Pos

func (m removedMap) lookup(r Pos) (Pos, bool) {
	gap, ok := m[r.Node]
	return gap, ok
}

// --- EditText ---------------------------------------------------------

// EditText replaces [From, To) of an inline slot with items. Inline nodes
// among the items with id 0 are new and get ids.
type EditText struct {
	At       SlotRef
	From, To int
	Items    []Item
}

func (o EditText) String() string {
	return fmt.Sprintf("EditText(%s, %d..%d, %d items)", o.At, o.From, o.To, len(o.Items))
}

func itemsLen(items []Item) int {
	n := 0
	for _, it := range items {
		n += it.Len()
	}
	return n
}

func (o EditText) apply(tx *Tx) (*Step, error) {
	_, content, err := tx.inlineSlot(o.At)
	if err != nil {
		return nil, err
	}
	if o.From < 0 || o.From > o.To || o.To > content.Len() {
		return nil, fmt.Errorf("%w: %d..%d of %d", ErrRange, o.From, o.To, content.Len())
	}
	items := make([]Item, len(o.Items))
	for i, it := range o.Items {
		if it.Node != nil {
			n, err := tx.prepare(it.Node)
			if err != nil {
				return nil, err
			}
			it.Node = n
		} else {
			it.Marks = tx.own(it.Marks)
		}
		items[i] = it
	}
	next, removed := content.replace(o.From, o.To, items)
	tx.setContent(o.At, next)
	gone := removedMap{}
	for _, it := range removed {
		if it.Node != nil {
			tx.unregister(it.Node, func(id ID) { gone[id] = At(o.At, o.From) })
		}
	}
	for _, it := range items {
		if it.Node != nil {
			tx.register(it.Node, o.At.Node, o.At.Slot)
		}
	}
	inserted := itemsLen(items)
	step := &Step{
		op:      o,
		inverse: []Op{EditText{At: o.At, From: o.From, To: o.From + inserted, Items: removed}},
		mapping: editTextMap{at: o.At, from: o.From, to: o.To, inserted: inserted, gone: gone},
		slots:   []SlotRef{o.At},
	}
	if o.To > o.From {
		step.records = append(step.records, Record{Kind: RecordTextDeleted, Slot: o.At, From: o.From, To: o.From,
			Text: content.TextRange(o.From, o.To)})
	}
	if inserted > 0 {
		step.records = append(step.records, Record{Kind: RecordTextInserted, Slot: o.At, From: o.From, To: o.From + inserted,
			Text: next.TextRange(o.From, o.From+inserted)})
	}
	return step, nil
}

type editTextMap struct {
	at                 SlotRef
	from, to, inserted int
	gone               removedMap
}

func (m editTextMap) Map(p Pos, bias Bias) (Pos, bool) {
	if gap, ok := m.gone.lookup(p); ok {
		return gap, false
	}
	if p.Ref() != m.at {
		return p, true
	}
	switch {
	case p.Offset < m.from:
	case p.Offset > m.to:
		p.Offset += m.inserted - (m.to - m.from)
	case p.Offset == m.from && m.from < m.to:
	case p.Offset == m.to && m.from < m.to:
		p.Offset = m.from + m.inserted
	case bias == BiasAfter:
		p.Offset = m.from + m.inserted
	default:
		p.Offset = m.from
	}
	return p, true
}

func (o EditText) rebase(m Mapper) (Op, bool) {
	from, ok1 := m.Map(At(o.At, o.From), BiasAfter)
	to, ok2 := m.Map(At(o.At, o.To), BiasBefore)
	if !ok1 || !ok2 || from.Ref() != o.At || to.Ref() != o.At || to.Offset < from.Offset {
		return nil, false
	}
	o.From, o.To = from.Offset, to.Offset
	return o, true
}

// --- MarkText -------------------------------------------------------------

// MarkText is the Mark operation: it changes the mark sets of the text
// runs in [From, To), adding Add and removing Remove, or, when Restore is
// set, giving the runs exactly the sets listed, as an inverse does.
type MarkText struct {
	At       SlotRef
	From, To int
	Add      []Mark
	Remove   []MarkKind
	Restore  []MarkRun
}

func (o MarkText) String() string {
	return fmt.Sprintf("MarkText(%s, %d..%d, +%d -%d)", o.At, o.From, o.To, len(o.Add), len(o.Remove))
}

func (o MarkText) apply(tx *Tx) (*Step, error) {
	_, content, err := tx.inlineSlot(o.At)
	if err != nil {
		return nil, err
	}
	to := o.To
	if o.Restore != nil {
		to = o.From
		for _, r := range o.Restore {
			to += r.Len
		}
	}
	if o.From < 0 || o.From > to || to > content.Len() {
		return nil, fmt.Errorf("%w: %d..%d of %d", ErrRange, o.From, to, content.Len())
	}
	var next *Inline
	var before []MarkRun
	kinds := map[MarkKind]bool{}
	if o.Restore != nil {
		next, before = content.restoreMarks(o.From, o.Restore)
		for _, r := range o.Restore {
			for _, m := range r.Marks.Marks() {
				kinds[m.Kind] = true
			}
		}
		for _, r := range before {
			for _, m := range r.Marks.Marks() {
				kinds[m.Kind] = true
			}
		}
	} else {
		next, before = content.remark(o.From, to, func(set *MarkSet) *MarkSet {
			if set == nil {
				set = tx.marks.Empty()
			}
			for _, m := range o.Add {
				set = set.With(m)
			}
			for _, k := range o.Remove {
				set = set.Without(k)
			}
			return set
		})
		for _, m := range o.Add {
			kinds[m.Kind] = true
		}
		for _, k := range o.Remove {
			kinds[k] = true
		}
	}
	tx.setContent(o.At, next)
	changed := make([]MarkKind, 0, len(kinds))
	for k := range kinds {
		changed = append(changed, k)
	}
	slices.Sort(changed)
	return &Step{
		op:      o,
		inverse: []Op{MarkText{At: o.At, From: o.From, Restore: before}},
		mapping: identity{},
		records: []Record{{Kind: RecordMarks, Slot: o.At, From: o.From, To: to, Marks: changed}},
		slots:   []SlotRef{o.At},
	}, nil
}

func (o MarkText) rebase(m Mapper) (Op, bool) {
	from, ok1 := m.Map(At(o.At, o.From), BiasAfter)
	if !ok1 || from.Ref() != o.At {
		return nil, false
	}
	if o.Restore != nil {
		// Restored sets name exact runs; they rebase only where the text
		// they cover did not change length.
		o.From = from.Offset
		return o, true
	}
	to, ok2 := m.Map(At(o.At, o.To), BiasBefore)
	if !ok2 || to.Ref() != o.At {
		return nil, false
	}
	o.From, o.To = from.Offset, to.Offset
	return o, true
}

// --- Transfer ---------------------------------------------------------

// Transfer moves the items in [Start, End) of one inline slot to At in
// another, keeping their runs and marks. Splitting and joining are built
// on it, so carets and annotations follow the text exactly.
type Transfer struct {
	From       SlotRef
	Start, End int
	To         SlotRef
	At         int
}

func (o Transfer) String() string {
	return fmt.Sprintf("Transfer(%s %d..%d -> %s@%d)", o.From, o.Start, o.End, o.To, o.At)
}

func (o Transfer) apply(tx *Tx) (*Step, error) {
	if o.From == o.To {
		return nil, fmt.Errorf("%w: a transfer within one slot", ErrStructure)
	}
	_, src, err := tx.inlineSlot(o.From)
	if err != nil {
		return nil, err
	}
	_, dst, err := tx.inlineSlot(o.To)
	if err != nil {
		return nil, err
	}
	if o.Start < 0 || o.Start > o.End || o.End > src.Len() || o.At < 0 || o.At > dst.Len() {
		return nil, ErrRange
	}
	items := src.Slice(o.Start, o.End)
	for _, it := range items {
		if it.Node != nil && Contains(tx, it.Node.id, o.To.Node) {
			return nil, fmt.Errorf("%w: moving a node into itself", ErrStructure)
		}
	}
	nextSrc, _ := src.replace(o.Start, o.End, nil)
	tx.setContent(o.From, nextSrc)
	_, dst, _ = tx.inlineSlot(o.To)
	nextDst, _ := dst.replace(o.At, o.At, items)
	tx.setContent(o.To, nextDst)
	for _, it := range items {
		if it.Node != nil {
			e, _ := tx.index.get(it.Node.id)
			e.parent, e.slot = o.To.Node, o.To.Slot
			tx.index = tx.index.set(it.Node.id, e)
		}
	}
	n := o.End - o.Start
	return &Step{
		op:      o,
		inverse: []Op{Transfer{From: o.To, Start: o.At, End: o.At + n, To: o.From, At: o.Start}},
		mapping: transferMap(o),
		records: []Record{{Kind: RecordTransferred, Slot: o.From, From: o.Start, To: o.End,
			Text: src.TextRange(o.Start, o.End), Target: o.To, At: o.At}},
		slots: []SlotRef{o.From, o.To},
	}, nil
}

type transferMap Transfer

func (m transferMap) Map(p Pos, bias Bias) (Pos, bool) {
	n := m.End - m.Start
	switch p.Ref() {
	case m.From:
		switch {
		case p.Offset < m.Start:
		case p.Offset > m.Start && p.Offset < m.End,
			p.Offset == m.Start && n > 0 && bias == BiasAfter,
			p.Offset == m.End && n > 0 && bias == BiasBefore:
			return At(m.To, m.At+p.Offset-m.Start), true
		case p.Offset >= m.End:
			p.Offset -= n
		}
		if p.Offset > m.Start && p.Offset < m.End {
			p.Offset = m.Start
		}
		return p, true
	case m.To:
		if p.Offset > m.At || (p.Offset == m.At && bias == BiasAfter) {
			p.Offset += n
		}
	}
	return p, true
}

func (o Transfer) rebase(m Mapper) (Op, bool) {
	start, ok1 := m.Map(At(o.From, o.Start), BiasAfter)
	end, ok2 := m.Map(At(o.From, o.End), BiasBefore)
	at, ok3 := m.Map(At(o.To, o.At), BiasAfter)
	if !ok1 || !ok2 || !ok3 || start.Ref() != o.From || end.Ref() != o.From || at.Ref() != o.To {
		return nil, false
	}
	o.Start, o.End, o.At = start.Offset, end.Offset, at.Offset
	return o, true
}

// --- SetNode ----------------------------------------------------------

// SetNode changes a node's kind and attributes. The new kind's slots must
// have the same shape.
type SetNode struct {
	ID    ID
	Kind  Kind
	Attrs Attrs
}

func (o SetNode) String() string { return fmt.Sprintf("SetNode(%s, %d)", o.ID, o.Kind) }

func (o SetNode) apply(tx *Tx) (*Step, error) {
	n, err := tx.node(o.ID)
	if err != nil {
		return nil, err
	}
	oldSpec, _ := tx.schema.Kind(n.kind)
	spec, ok := tx.schema.Kind(o.Kind)
	if !ok || oldSpec == nil || len(spec.Slots) != len(oldSpec.Slots) || spec.Placement != oldSpec.Placement {
		return nil, fmt.Errorf("%w: %s cannot become a %s", ErrStructure, tx.schema.KindName(n.kind), tx.schema.KindName(o.Kind))
	}
	for i := range spec.Slots {
		if spec.Slots[i].Blocks != oldSpec.Slots[i].Blocks {
			return nil, fmt.Errorf("%w: slot %d changes type", ErrStructure, i)
		}
	}
	old := SetNode{ID: o.ID, Kind: n.kind, Attrs: n.attrs}
	c := tx.mutable(o.ID)
	c.kind, c.attrs = o.Kind, canonicalAttrs(spec.Fields, o.Attrs)
	tx.touched[o.ID] = true
	for i := range c.slots {
		if c.slots[i].inline {
			tx.slots[SlotRef{Node: o.ID, Slot: i}] = true
		}
	}
	if parent, _, ok := tx.Parent(o.ID); ok {
		tx.touched[parent] = true
	}
	return &Step{
		op:      o,
		inverse: []Op{old},
		mapping: identity{},
		records: []Record{{Kind: RecordNodeChanged, Node: o.ID, OldKind: old.Kind, NewKind: o.Kind}},
		nodes:   []ID{o.ID},
	}, nil
}

func (o SetNode) rebase(m Mapper) (Op, bool) {
	if _, ok := m.Map(Pos{Node: o.ID}, BiasAfter); !ok {
		return nil, false
	}
	return o, true
}

// --- Insert -----------------------------------------------------------

// Insert places subtrees in a block slot at gap Index. New nodes (id 0)
// get ids; a subtree that has ids keeps them, as an undone removal does.
type Insert struct {
	Parent ID
	Slot   int
	Index  int
	Nodes  []*Node
}

func (o Insert) String() string {
	return fmt.Sprintf("Insert(%s.%d@%d, %d nodes)", o.Parent, o.Slot, o.Index, len(o.Nodes))
}

func (o Insert) apply(tx *Tx) (*Step, error) {
	ref := SlotRef{Node: o.Parent, Slot: o.Slot}
	_, slot, err := tx.blockSlot(ref)
	if err != nil {
		return nil, err
	}
	if o.Index < 0 || o.Index > slot.Len() {
		return nil, fmt.Errorf("%w: gap %d of %d", ErrRange, o.Index, slot.Len())
	}
	prepared := make([]*Node, len(o.Nodes))
	for i, n := range o.Nodes {
		if prepared[i], err = tx.prepare(n); err != nil {
			return nil, err
		}
	}
	parent := tx.mutable(o.Parent)
	placed := tx.place(parent, o.Slot, o.Index, prepared)
	ids := make([]ID, len(placed))
	for i, n := range placed {
		tx.register(n, o.Parent, o.Slot)
		ids[i] = n.id
	}
	tx.touched[o.Parent] = true
	return &Step{
		op:      Insert{Parent: o.Parent, Slot: o.Slot, Index: o.Index, Nodes: placed},
		inverse: []Op{Remove{IDs: ids}},
		mapping: gapShift{ref: ref, at: o.Index, by: len(placed)},
		records: []Record{{Kind: RecordNodesInserted, Slot: ref, From: o.Index, To: o.Index + len(placed), Nodes: ids}},
		slots:   []SlotRef{ref},
		nodes:   ids,
	}, nil
}

// gapShift moves the gaps of a block slot at or after at by by.
type gapShift struct {
	ref SlotRef
	at  int
	by  int
}

func (m gapShift) Map(p Pos, bias Bias) (Pos, bool) {
	if p.Ref() == m.ref && (p.Offset > m.at || (p.Offset == m.at && bias == BiasAfter)) {
		p.Offset += m.by
	}
	return p, true
}

func (o Insert) rebase(m Mapper) (Op, bool) {
	at, ok := m.Map(Pos{Node: o.Parent, Slot: o.Slot, Offset: o.Index}, BiasBefore)
	if !ok || at.Node != o.Parent || at.Slot != o.Slot {
		return nil, false
	}
	o.Index = at.Offset
	return o, true
}

// --- Remove -----------------------------------------------------------

// Remove takes subtrees out of their block slots. Positions inside them go
// to the gap they leave.
type Remove struct {
	IDs []ID
}

func (o Remove) String() string { return fmt.Sprintf("Remove(%d nodes)", len(o.IDs)) }

type removal struct {
	ref   SlotRef
	index int
	node  *Node
}

func (o Remove) apply(tx *Tx) (*Step, error) {
	var removals []removal
	for _, id := range o.IDs {
		n, err := tx.node(id)
		if err != nil {
			return nil, err
		}
		parent, slot, ok := tx.Parent(id)
		if !ok {
			return nil, fmt.Errorf("%w: removing the root", ErrStructure)
		}
		covered := false
		for _, other := range o.IDs {
			if other != id && Contains(tx, other, id) {
				covered = true
			}
		}
		if covered {
			continue
		}
		pn, _ := tx.Node(parent)
		s := pn.slots[slot]
		if s.inline {
			return nil, fmt.Errorf("%w: %s is inline; remove it with EditText", ErrStructure, id)
		}
		index, _ := s.IndexOf(n)
		removals = append(removals, removal{ref: SlotRef{Node: parent, Slot: slot}, index: index, node: n})
	}
	slices.SortFunc(removals, func(a, b removal) int {
		if a.ref != b.ref {
			if a.ref.Node != b.ref.Node {
				if a.ref.Node < b.ref.Node {
					return -1
				}
				return 1
			}
			return a.ref.Slot - b.ref.Slot
		}
		return a.index - b.index
	})
	gone := removedMap{}
	shifts := map[SlotRef][]int{}
	var inverse []Op
	var refs []SlotRef
	ids := make([]ID, 0, len(removals))
	for i, r := range removals {
		// Inverse inserts go back in ascending order at their own indices.
		if last := len(inverse) - 1; last >= 0 && i > 0 && removals[i-1].ref == r.ref && removals[i-1].index == r.index-1 {
			ins := inverse[last].(Insert)
			ins.Nodes = append(ins.Nodes, r.node)
			inverse[last] = ins
		} else {
			inverse = append(inverse, Insert{Parent: r.ref.Node, Slot: r.ref.Slot, Index: r.index, Nodes: []*Node{r.node}})
		}
		shifts[r.ref] = append(shifts[r.ref], r.index)
		if !slices.Contains(refs, r.ref) {
			refs = append(refs, r.ref)
		}
		ids = append(ids, r.node.id)
	}
	// Remove from the highest index down so indices stay valid.
	for i := len(removals) - 1; i >= 0; i-- {
		r := removals[i]
		parent := tx.mutable(r.ref.Node)
		s := &parent.slots[r.ref.Slot]
		tx.ownChildren(s)
		s.children = slices.Delete(s.children, r.index, r.index+1)
		tx.touched[r.ref.Node] = true
		before := 0
		for _, j := range shifts[r.ref] {
			if j < r.index {
				before++
			}
		}
		gap := Pos{Node: r.ref.Node, Slot: r.ref.Slot, Offset: r.index - before}
		tx.unregister(r.node, func(id ID) { gone[id] = gap })
	}
	for id := range tx.touched {
		if _, ok := gone[id]; ok {
			delete(tx.touched, id)
		}
	}
	return &Step{
		op:      o,
		inverse: inverse,
		mapping: removeMap{gone: gone, shifts: shifts},
		records: []Record{{Kind: RecordNodesRemoved, Nodes: ids}},
		slots:   refs,
		nodes:   ids,
	}, nil
}

type removeMap struct {
	gone   removedMap
	shifts map[SlotRef][]int
}

func (m removeMap) Map(p Pos, _ Bias) (Pos, bool) {
	if gap, ok := m.gone.lookup(p); ok {
		return gap, false
	}
	if removed, ok := m.shifts[p.Ref()]; ok {
		before := 0
		for _, i := range removed {
			if i < p.Offset {
				before++
			}
		}
		p.Offset -= before
	}
	return p, true
}

func (o Remove) rebase(m Mapper) (Op, bool) {
	for _, id := range o.IDs {
		if _, ok := m.Map(Pos{Node: id}, BiasAfter); !ok {
			return nil, false
		}
	}
	return o, true
}

// --- Move -------------------------------------------------------------

// Move moves a subtree to gap Index of a block slot, keeping its identity.
// Index is counted with the node still in its old place.
type Move struct {
	ID     ID
	Parent ID
	Slot   int
	Index  int
	// ord is the key an inverse restores, so undo rebuilds the same tree.
	ord string
}

func (o Move) String() string {
	return fmt.Sprintf("Move(%s -> %s.%d@%d)", o.ID, o.Parent, o.Slot, o.Index)
}

func (o Move) apply(tx *Tx) (*Step, error) {
	n, err := tx.node(o.ID)
	if err != nil {
		return nil, err
	}
	oldParent, oldSlot, ok := tx.Parent(o.ID)
	if !ok {
		return nil, fmt.Errorf("%w: moving the root", ErrStructure)
	}
	if Contains(tx, o.ID, o.Parent) {
		return nil, fmt.Errorf("%w: moving a node into itself", ErrStructure)
	}
	target := SlotRef{Node: o.Parent, Slot: o.Slot}
	_, slot, err := tx.blockSlot(target)
	if err != nil {
		return nil, err
	}
	if o.Index < 0 || o.Index > slot.Len() {
		return nil, ErrRange
	}
	source := SlotRef{Node: oldParent, Slot: oldSlot}
	op, _ := tx.Node(oldParent)
	if op.slots[oldSlot].inline {
		return nil, fmt.Errorf("%w: %s is inline", ErrStructure, o.ID)
	}
	oldIndex, _ := op.slots[oldSlot].IndexOf(n)
	oldOrd := n.ord
	from := tx.mutable(oldParent)
	s := &from.slots[oldSlot]
	tx.ownChildren(s)
	s.children = slices.Delete(s.children, oldIndex, oldIndex+1)
	index := o.Index
	if source == target && oldIndex < index {
		index--
	}
	to := tx.mutable(o.Parent)
	moved := n
	if o.ord != "" {
		moved = tx.withOrd(n, o.ord)
	}
	placed := tx.place(to, o.Slot, index, []*Node{moved})[0]
	e, _ := tx.index.get(o.ID)
	e.node, e.parent, e.slot = placed, o.Parent, o.Slot
	tx.index = tx.index.set(o.ID, e)
	tx.touched[oldParent] = true
	tx.touched[o.Parent] = true
	back := oldIndex
	if source == target && oldIndex > index {
		back = oldIndex + 1
	}
	return &Step{
		op:      o,
		inverse: []Op{Move{ID: o.ID, Parent: oldParent, Slot: oldSlot, Index: back, ord: oldOrd}},
		mapping: moveMap{from: source, oldIndex: oldIndex, to: target, index: index},
		records: []Record{{Kind: RecordNodeMoved, Nodes: []ID{o.ID}, Slot: source, From: oldIndex, Target: target, At: index}},
		slots:   []SlotRef{source, target},
		nodes:   []ID{o.ID},
	}, nil
}

type moveMap struct {
	from     SlotRef
	oldIndex int
	to       SlotRef
	index    int
}

func (m moveMap) Map(p Pos, bias Bias) (Pos, bool) {
	if p.Ref() == m.from && p.Offset > m.oldIndex {
		p.Offset--
	}
	if p.Ref() == m.to && (p.Offset > m.index || (p.Offset == m.index && bias == BiasAfter)) {
		p.Offset++
	}
	return p, true
}

func (o Move) rebase(m Mapper) (Op, bool) {
	if _, ok := m.Map(Pos{Node: o.ID}, BiasAfter); !ok {
		return nil, false
	}
	at, ok := m.Map(Pos{Node: o.Parent, Slot: o.Slot, Offset: o.Index}, BiasBefore)
	if !ok || at.Node != o.Parent {
		return nil, false
	}
	o.Index = at.Offset
	return o, true
}
