package vocab

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
)

// The semantic helpers. Each returns an Edit made of primitive
// operations; policies edit only through helpers and primitives.
//
// TODO(tables): insert, remove and move rows and columns; resize the grid;
// set the header; set a column's attributes; clear and fill cells (§5.2).
// TODO(figures): insert a figure; change its shape; replace its media.

// ErrHelper reports a helper asked for something the document cannot do.
var ErrHelper = errors.New("vocab: the edit does not fit the document")

// --- marks ------------------------------------------------------------

// Coverage is how much of a range carries a mark.
type Coverage uint8

const (
	CoverNone Coverage = iota
	CoverPartial
	CoverFull
)

// TextSpans returns the stretches of text slots from from to to, in
// reading order.
func TextSpans(r doc.Reader, from, to doc.Pos) []Span {
	if from.Ref() == to.Ref() {
		return []Span{{Slot: from.Ref(), From: min(from.Offset, to.Offset), To: max(from.Offset, to.Offset)}}
	}
	var out []Span
	ref := from.Ref()
	start := from.Offset
	for {
		n, ok := r.Node(ref.Node)
		if !ok {
			return out
		}
		if ref == to.Ref() {
			out = append(out, Span{Slot: ref, From: start, To: to.Offset})
			return out
		}
		out = append(out, Span{Slot: ref, From: start, To: n.Slot(ref.Slot).Len()})
		next, ok := NextTextSlot(r, ref)
		if !ok {
			return out
		}
		ref, start = next, 0
	}
}

// CoverageOf is how much of spans carries a mark of kind k. Inline nodes
// do not count.
func CoverageOf(r doc.Reader, spans []Span, k doc.MarkKind) Coverage {
	with, without := 0, 0
	for _, s := range spans {
		n, ok := r.Node(s.Slot.Node)
		if !ok {
			continue
		}
		for _, it := range n.Slot(s.Slot.Slot).Content().Slice(s.From, s.To) {
			if it.Node != nil {
				continue
			}
			if it.Marks.Has(k) {
				with += len(it.Text)
			} else {
				without += len(it.Text)
			}
		}
	}
	switch {
	case with == 0:
		return CoverNone
	case without == 0:
		return CoverFull
	}
	return CoverPartial
}

// ApplyMark adds m over spans, skipping literal slots and slots that do
// not take it. Applying a mark never nests it.
func ApplyMark(spans []Span, m doc.Mark) Edit {
	return func(tx *doc.Tx) error {
		for _, s := range spans {
			spec, ok := SlotSpec(tx, s.Slot)
			if !ok || spec.Literal || spec.Marks == nil || !spec.Marks(m.Kind) || s.From == s.To {
				continue
			}
			op := doc.MarkText{At: s.Slot, From: s.From, To: s.To, Add: []doc.Mark{m}}
			if mspec, ok := Schema().Mark(m.Kind); ok && mspec.Excludes != nil {
				for _, other := range []doc.MarkKind{Emphasis, Strong, CodeMark, Strike} {
					if other != m.Kind && mspec.Excludes(other) {
						op.Remove = append(op.Remove, other)
					}
				}
			}
			if err := tx.Apply(op); err != nil {
				return err
			}
		}
		return nil
	}
}

// ClearMark removes marks of kind k over spans.
func ClearMark(spans []Span, k doc.MarkKind) Edit {
	return func(tx *doc.Tx) error {
		for _, s := range spans {
			if s.From == s.To {
				continue
			}
			if err := tx.Apply(doc.MarkText{At: s.Slot, From: s.From, To: s.To, Remove: []doc.MarkKind{k}}); err != nil {
				return err
			}
		}
		return nil
	}
}

// --- blocks -----------------------------------------------------------

// SetKind gives a node another kind, with attrs.
func SetKind(id doc.ID, kind doc.Kind, attrs doc.Attrs) Edit {
	return Ops(doc.SetNode{ID: id, Kind: kind, Attrs: attrs})
}

// SetAttrs changes a node's attributes, keeping its kind.
func SetAttrs(id doc.ID, attrs doc.Attrs) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		return tx.Apply(doc.SetNode{ID: id, Kind: n.Kind(), Attrs: attrs})
	}
}

// Split splits the block holding at in two: a new node of kind follows it,
// and the text after at moves into the new node's first text slot.
// Everything after at in the block's later slots stays.
func Split(at doc.Pos, kind doc.Kind, attrs doc.Attrs) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(at.Node)
		if !ok || at.Slot >= n.NumSlots() || !n.Slot(at.Slot).IsInline() {
			return fmt.Errorf("%w: split outside a text slot", ErrHelper)
		}
		parent, slot, index, ok := Index(tx, at.Node)
		if !ok {
			return fmt.Errorf("%w: split of the page", ErrHelper)
		}
		pn, _ := tx.Node(parent)
		if pn.Slot(slot).IsInline() {
			return fmt.Errorf("%w: split of an inline node", ErrHelper)
		}
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index + 1, Nodes: []*doc.Node{NewBlock(kind, attrs)}}); err != nil {
			return err
		}
		pn, _ = tx.Node(parent)
		created := pn.Slot(slot).Child(index + 1)
		target, ok := FirstTextSlot(created)
		if !ok {
			return nil
		}
		end := n.Slot(at.Slot).Len()
		if at.Offset == end {
			return nil
		}
		return tx.Apply(doc.Transfer{From: at.Ref(), Start: at.Offset, End: end, To: target, At: 0})
	}
}

// CanJoin reports whether second's text can move onto the end of first's:
// both are text slots of the same literalness, and line breaks are kept
// where second has them.
func CanJoin(r doc.Reader, first, second doc.SlotRef) bool {
	a, ok1 := SlotSpec(r, first)
	b, ok2 := SlotSpec(r, second)
	if !ok1 || !ok2 || a.Blocks || b.Blocks || a.Literal != b.Literal {
		return false
	}
	if !a.Lines {
		n, _ := r.Node(second.Node)
		if strings.ContainsRune(n.Slot(second.Slot).Content().Text(), '\n') {
			return false
		}
	}
	return true
}

// Join moves second's text onto the end of first's text slot, moves
// second's block children after first (into first's body when it has
// one), and removes second.
func Join(first doc.SlotRef, second doc.ID) Edit {
	return func(tx *doc.Tx) error {
		sn, ok := tx.Node(second)
		if !ok {
			return doc.ErrNoNode
		}
		fn, ok := tx.Node(first.Node)
		if !ok {
			return doc.ErrNoNode
		}
		src, ok := FirstTextSlot(sn)
		if ok && src.Node == second {
			if !CanJoin(tx, first, src) {
				return fmt.Errorf("%w: incompatible slots never merge", ErrHelper)
			}
			if l := sn.Slot(src.Slot).Len(); l > 0 {
				at := fn.Slot(first.Slot).Len()
				if err := tx.Apply(doc.Transfer{From: src, Start: 0, End: l, To: first, At: at}); err != nil {
					return err
				}
			}
		}
		sn, _ = tx.Node(second)
		if err := moveChildrenAfter(tx, sn, first.Node); err != nil {
			return err
		}
		return tx.Apply(doc.Remove{IDs: []doc.ID{second}})
	}
}

// moveChildrenAfter moves the block children of from into target's body
// slot, or after target when it has none.
func moveChildrenAfter(tx *doc.Tx, from *doc.Node, target doc.ID) error {
	var moving []doc.ID
	for _, s := range from.Slots() {
		if s.IsInline() {
			continue
		}
		for _, c := range s.Children() {
			moving = append(moving, c.ID())
		}
	}
	if len(moving) == 0 {
		return nil
	}
	tn, _ := tx.Node(target)
	body := -1
	for i, s := range tn.Slots() {
		if !s.IsInline() {
			body = i
		}
	}
	for _, id := range moving {
		var err error
		if body >= 0 {
			tn, _ = tx.Node(target)
			err = tx.Apply(doc.Move{ID: id, Parent: target, Slot: body, Index: tn.Slot(body).Len()})
		} else {
			parent, slot, index, _ := Index(tx, target)
			err = tx.Apply(doc.Move{ID: id, Parent: parent, Slot: slot, Index: index + 1})
			target = id
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Wrap puts contiguous siblings into a new container of kind.
func Wrap(ids []doc.ID, kind doc.Kind, attrs doc.Attrs) Edit {
	return func(tx *doc.Tx) error {
		if len(ids) == 0 {
			return nil
		}
		parent, slot, index, ok := Index(tx, ids[0])
		if !ok {
			return doc.ErrNoNode
		}
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{NewBlock(kind, attrs)}}); err != nil {
			return err
		}
		pn, _ := tx.Node(parent)
		container := pn.Slot(slot).Child(index).ID()
		body := bodySlot(kind)
		for i, id := range ids {
			if err := tx.Apply(doc.Move{ID: id, Parent: container, Slot: body, Index: i}); err != nil {
				return err
			}
		}
		return nil
	}
}

// bodySlot is the block slot of a container kind.
func bodySlot(kind doc.Kind) int {
	spec, _ := Schema().Kind(kind)
	for i, s := range spec.Slots {
		if s.Blocks {
			return i
		}
	}
	return 0
}

// Unwrap moves a container's children out in its place and removes it.
func Unwrap(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		parent, slot, index, ok := Index(tx, id)
		if !ok {
			return doc.ErrNoNode
		}
		at := index
		for _, s := range n.Slots() {
			if s.IsInline() {
				continue
			}
			for _, c := range s.Children() {
				if err := tx.Apply(doc.Move{ID: c.ID(), Parent: parent, Slot: slot, Index: at}); err != nil {
					return err
				}
				at++
			}
		}
		return tx.Apply(doc.Remove{IDs: []doc.ID{id}})
	}
}

// Lift moves a node out of its container, to just after it. A container
// left empty is removed.
func Lift(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		container, _, _, ok := Index(tx, id)
		if !ok {
			return doc.ErrNoNode
		}
		cn, _ := tx.Node(container)
		grand, cslot, cindex, ok := Index(tx, container)
		if !ok {
			return fmt.Errorf("%w: lifting out of the page", ErrHelper)
		}
		// Siblings after id stay in the container, which splits around it:
		// they move into a copy of the container after the lifted node.
		parent, slot, index, _ := Index(tx, id)
		pn, _ := tx.Node(parent)
		rest := pn.Slot(slot).Children()[index+1:]
		if err := tx.Apply(doc.Move{ID: id, Parent: grand, Slot: cslot, Index: cindex + 1}); err != nil {
			return err
		}
		if len(rest) > 0 {
			tail := NewBlock(cn.Kind(), cn.Attrs())
			if err := tx.Apply(doc.Insert{Parent: grand, Slot: cslot, Index: cindex + 2, Nodes: []*doc.Node{tail}}); err != nil {
				return err
			}
			gn, _ := tx.Node(grand)
			tailID := gn.Slot(cslot).Child(cindex + 2).ID()
			for i, r := range rest {
				if err := tx.Apply(doc.Move{ID: r.ID(), Parent: tailID, Slot: slot, Index: i}); err != nil {
					return err
				}
			}
		}
		return RemoveIfEmpty(container)(tx)
	}
}

// Sink moves an item into the body of the item before it, into the list
// that body ends with, or a new list of its own list's style.
func Sink(item doc.ID) Edit {
	return func(tx *doc.Tx) error {
		list, slot, index, ok := Index(tx, item)
		if !ok || index == 0 {
			return fmt.Errorf("%w: no item before to sink into", ErrHelper)
		}
		ln, _ := tx.Node(list)
		prev := ln.Slot(slot).Child(index - 1)
		body := prev.Slot(1)
		target := doc.ID(0)
		if n := body.Len(); n > 0 && body.Child(n-1).Kind() == List {
			target = body.Child(n - 1).ID()
		} else {
			if err := tx.Apply(doc.Insert{Parent: prev.ID(), Slot: 1, Index: n, Nodes: []*doc.Node{NewBlock(List, ln.Attrs())}}); err != nil {
				return err
			}
			pn, _ := tx.Node(prev.ID())
			target = pn.Slot(1).Child(n).ID()
		}
		tn, _ := tx.Node(target)
		return tx.Apply(doc.Move{ID: item, Parent: target, Slot: 0, Index: tn.Slot(0).Len()})
	}
}

// MoveTo moves a node to a block gap.
func MoveTo(id doc.ID, target doc.Pos) Edit {
	return Ops(doc.Move{ID: id, Parent: target.Node, Slot: target.Slot, Index: target.Offset})
}

// RemoveIfEmpty removes a container whose block slots hold nothing, and
// then its container if that is left empty too. The page is never
// removed, and a container with text is not empty.
func RemoveIfEmpty(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		for {
			n, ok := tx.Node(id)
			if !ok {
				return nil
			}
			parent, _, ok := tx.Parent(id)
			if !ok || !emptyContainer(n) {
				return nil
			}
			if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
				return err
			}
			id = parent
		}
	}
}

func emptyContainer(n *doc.Node) bool {
	blocks := false
	for _, s := range n.Slots() {
		if s.IsInline() {
			return false
		}
		blocks = true
		if s.Len() > 0 {
			return false
		}
	}
	return blocks
}

// --- ranges -----------------------------------------------------------

// liftOut moves an endpoint inside an inline node's slot out to the slot
// holding the node, so a range never cuts an inline node: the start goes
// before it, the end after it.
func liftOut(r doc.Reader, p doc.Pos, end bool) doc.Pos {
	for {
		parent, slot, ok := r.Parent(p.Node)
		if !ok {
			return p
		}
		pn, _ := r.Node(parent)
		s := pn.Slot(slot)
		if !s.IsInline() {
			return p
		}
		off, _ := s.Content().NodeOffset(p.Node)
		if end {
			off++
		}
		p = doc.Pos{Node: parent, Slot: slot, Offset: off}
	}
}

// DeleteRange deletes from from to to: the text of the end slots, every
// node wholly between, and then joins the end blocks when their slots are
// compatible. Containers left empty are removed.
func DeleteRange(from, to doc.Pos) Edit {
	return func(tx *doc.Tx) error {
		if doc.Compare(tx, doc.CaretAt(to), doc.CaretAt(from)) < 0 {
			from, to = to, from
		}
		if from.Ref() != to.Ref() {
			from, to = liftOut(tx, from, false), liftOut(tx, to, true)
		}
		if from.Ref() == to.Ref() {
			if from.Offset == to.Offset {
				return nil
			}
			n, _ := tx.Node(from.Node)
			if n.Slot(from.Slot).IsInline() {
				return tx.Apply(doc.EditText{At: from.Ref(), From: from.Offset, To: to.Offset})
			}
			children := n.Slot(from.Slot).Children()[from.Offset:to.Offset]
			ids := make([]doc.ID, len(children))
			for i, c := range children {
				ids[i] = c.ID()
			}
			return tx.Apply(doc.Remove{IDs: ids})
		}
		between := Between(tx, from, to)
		fromText := isInline(tx, from.Ref())
		toText := isInline(tx, to.Ref())
		if fromText {
			fn, _ := tx.Node(from.Node)
			if end := fn.Slot(from.Slot).Len(); end > from.Offset {
				if err := tx.Apply(doc.EditText{At: from.Ref(), From: from.Offset, To: end}); err != nil {
					return err
				}
			}
		}
		if toText && to.Offset > 0 {
			if err := tx.Apply(doc.EditText{At: to.Ref(), From: 0, To: to.Offset}); err != nil {
				return err
			}
		}
		if len(between) > 0 {
			if err := tx.Apply(doc.Remove{IDs: between}); err != nil {
				return err
			}
		}
		if !fromText || !toText {
			return nil
		}
		second, ok := Block(tx, to)
		if !ok || second.ID() == from.Node || doc.Contains(tx, second.ID(), from.Node) {
			return nil
		}
		ancestors := doc.Ancestors(tx, second.ID())
		if CanJoin(tx, from.Ref(), to.Ref()) {
			if err := Join(from.Ref(), second.ID())(tx); err != nil {
				return err
			}
		}
		for _, a := range ancestors {
			if doc.Contains(tx, a, from.Node) {
				break
			}
			if err := RemoveIfEmpty(a)(tx); err != nil {
				return err
			}
		}
		return nil
	}
}

func isInline(r doc.Reader, ref doc.SlotRef) bool {
	n, ok := r.Node(ref.Node)
	return ok && ref.Slot < n.NumSlots() && n.Slot(ref.Slot).IsInline()
}

// Between returns the largest subtrees lying wholly between from and to,
// neither holding an end.
func Between(r doc.Reader, from, to doc.Pos) []doc.ID {
	var out []doc.ID
	start, end := doc.CaretAt(from), doc.CaretAt(to)
	var visit func(n *doc.Node)
	visit = func(n *doc.Node) {
		for i, s := range n.Slots() {
			if s.IsInline() {
				continue
			}
			for j, c := range s.Children() {
				holds := doc.Contains(r, c.ID(), from.Node) || doc.Contains(r, c.ID(), to.Node)
				if holds {
					visit(c)
					continue
				}
				before := doc.CaretAt(doc.Pos{Node: n.ID(), Slot: i, Offset: j})
				afterGap := doc.CaretAt(doc.Pos{Node: n.ID(), Slot: i, Offset: j + 1})
				if doc.Compare(r, start, before) <= 0 && doc.Compare(r, afterGap, end) <= 0 {
					out = append(out, c.ID())
				}
			}
		}
	}
	visit(r.Root())
	return out
}

// --- slices -----------------------------------------------------------

// Slice is a copied fragment: block nodes with an open depth at each edge,
// or, for an inline-only fragment, inline items. Its nodes carry no ids,
// so pasting makes new ones.
type Slice struct {
	Nodes     []*doc.Node
	OpenStart int
	OpenEnd   int
	Inline    []doc.Item
}

// Empty reports a slice with nothing in it.
func (s Slice) Empty() bool { return len(s.Nodes) == 0 && len(s.Inline) == 0 }

// Fresh copies a subtree without its ids.
func Fresh(n *doc.Node) *doc.Node {
	slots := make([]doc.Slot, n.NumSlots())
	for i, s := range n.Slots() {
		if s.IsInline() {
			slots[i] = doc.InlineSlot(freshItems(s.Content().Slice(0, s.Len()))...)
			continue
		}
		children := make([]*doc.Node, s.Len())
		for j, c := range s.Children() {
			children[j] = Fresh(c)
		}
		slots[i] = doc.BlockSlot(children...)
	}
	return doc.NewNode(n.Kind(), n.Attrs(), slots...)
}

func freshItems(items []doc.Item) []doc.Item {
	out := make([]doc.Item, len(items))
	for i, it := range items {
		if it.Node != nil {
			it.Node = Fresh(it.Node)
		}
		out[i] = it
	}
	return out
}

// Copy returns the slice from from to to.
func Copy(r doc.Reader, from, to doc.Pos) Slice {
	if doc.Compare(r, doc.CaretAt(to), doc.CaretAt(from)) < 0 {
		from, to = to, from
	}
	if from.Ref() == to.Ref() && isInline(r, from.Ref()) {
		n, _ := r.Node(from.Node)
		return Slice{Inline: freshItems(n.Slot(from.Slot).Content().Slice(from.Offset, to.Offset))}
	}
	from, to = liftOut(r, from, false), liftOut(r, to, true)
	var nodes []*doc.Node
	if a, ok := Block(r, from); ok && isInline(r, from.Ref()) {
		nodes = append(nodes, trimmed(a, from.Slot, from.Offset, -1))
	}
	for _, id := range Between(r, from, to) {
		n, _ := r.Node(id)
		nodes = append(nodes, Fresh(n))
	}
	if b, ok := Block(r, to); ok && isInline(r, to.Ref()) {
		nodes = append(nodes, trimmed(b, to.Slot, 0, to.Offset))
	}
	return Slice{Nodes: nodes, OpenStart: 1, OpenEnd: 1}
}

// trimmed copies a text block keeping [from, to) of one slot; to -1 keeps
// to the end.
func trimmed(n *doc.Node, slot, from, to int) *doc.Node {
	c := Fresh(n)
	slots := c.Slots()
	out := make([]doc.Slot, len(slots))
	for i, s := range slots {
		out[i] = s
		if i == slot {
			end := to
			if end < 0 {
				end = s.Len()
			}
			out[i] = doc.InlineSlot(s.Content().Slice(from, end)...)
		} else if !s.IsInline() && to >= 0 {
			out[i] = doc.BlockSlot()
		}
	}
	return doc.NewNode(c.Kind(), c.Attrs(), out...)
}

// Fit adapts a slice to the slot it lands in. A rich slot without lines
// takes it as it is: inline items in place, blocks split around. Into a
// literal slot, or a cell, which keeps lines but holds no blocks, it is
// plain text, with line breaks only where the slot keeps them.
func Fit(spec doc.SlotSpec, s Slice) Slice {
	if !spec.Literal && (!spec.Lines || len(s.Nodes) == 0) {
		return s
	}
	var b strings.Builder
	for _, it := range s.Inline {
		if it.Node != nil {
			b.WriteString(it.Node.Text())
		} else {
			b.WriteString(it.Text)
		}
	}
	for i, n := range s.Nodes {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(n.Text())
	}
	text := b.String()
	if !spec.Lines {
		text = strings.ReplaceAll(text, "\n", " ")
	}
	return Slice{Inline: []doc.Item{doc.TextItem(text, nil)}}
}

// InsertSlice inserts a slice at a text position, fitted to its slot. A
// block slice splits the block there, places its nodes between the
// halves, and joins its open edges into them.
func InsertSlice(at doc.Pos, s Slice) Edit {
	return func(tx *doc.Tx) error {
		spec, ok := SlotSpec(tx, at.Ref())
		if !ok || spec.Blocks {
			return fmt.Errorf("%w: paste outside a text slot", ErrHelper)
		}
		s = Fit(spec, s)
		if len(s.Nodes) == 0 {
			items := s.Inline
			if spec.Literal {
				for i := range items {
					items[i].Marks = nil
				}
			}
			return tx.Apply(doc.EditText{At: at.Ref(), From: at.Offset, To: at.Offset, Items: items})
		}
		block, _ := Block(tx, at)
		if err := Split(at, block.Kind(), block.Attrs())(tx); err != nil {
			return err
		}
		parent, slot, index, _ := Index(tx, block.ID())
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index + 1, Nodes: s.Nodes}); err != nil {
			return err
		}
		pn, _ := tx.Node(parent)
		first := pn.Slot(slot).Child(index + 1)
		last := pn.Slot(slot).Child(index + len(s.Nodes))
		tail := pn.Slot(slot).Child(index + len(s.Nodes) + 1)
		if s.OpenEnd > 0 {
			if ref, ok := LastTextSlot(last); ok && CanJoin(tx, ref, doc.SlotRef{Node: tail.ID()}) {
				if err := Join(ref, tail.ID())(tx); err != nil {
					return err
				}
			}
		}
		if s.OpenStart > 0 {
			if CanJoin(tx, at.Ref(), doc.SlotRef{Node: first.ID()}) {
				if err := Join(at.Ref(), first.ID())(tx); err != nil {
					return err
				}
			}
		}
		return nil
	}
}
