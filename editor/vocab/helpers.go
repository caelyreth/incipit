package vocab

import (
	"errors"
	"fmt"
	"slices"
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
	from, to = textBound(r, from, false), textBound(r, to, true)
	if doc.Compare(r, doc.CaretAt(to), doc.CaretAt(from)) < 0 {
		return nil
	}
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

// textBound is the text position a range's end at a block gap stands
// for: the first text after the gap for a start, the last before it for an
// end. A text position is its own.
func textBound(r doc.Reader, p doc.Pos, end bool) doc.Pos {
	n, ok := r.Node(p.Node)
	if !ok || p.Slot >= n.NumSlots() || n.Slot(p.Slot).IsInline() {
		return p
	}
	s := n.Slot(p.Slot)
	if !end {
		for _, c := range s.Children()[min(p.Offset, s.Len()):] {
			if ref, ok := FirstTextSlot(c); ok {
				return doc.At(ref, 0)
			}
		}
		if ref, ok := NextTextSlot(r, doc.SlotRef{Node: p.Node, Slot: p.Slot}); ok {
			return doc.At(ref, 0)
		}
		return p
	}
	children := s.Children()[:min(max(p.Offset, 0), s.Len())]
	for i := len(children) - 1; i >= 0; i-- {
		if ref, ok := LastTextSlot(children[i]); ok {
			tn, _ := r.Node(ref.Node)
			return doc.At(ref, tn.Slot(ref.Slot).Len())
		}
	}
	return p
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
			op := doc.MarkText{At: s.Slot, From: s.From, To: s.To, Add: []doc.Mark{m}, Remove: excludedBy(tx.Schema(), m.Kind)}
			if err := tx.Apply(op); err != nil {
				return err
			}
		}
		return nil
	}
}

// InheritedMarks are the marks text typed at off takes where no ghost
// notation or pending marks decide: those of the text before it. Bounded
// marks are not extended past their end, so typing after inline code
// leaves it.
func InheritedMarks(s *doc.Schema, in *doc.Inline, off int) *doc.MarkSet {
	ms := in.MarksAt(off)
	if ms == nil || off == 0 {
		return ms
	}
	after := in.MarksAt(min(off+1, in.Len()))
	for _, m := range ms.Marks() {
		if spec, ok := s.Mark(m.Kind); ok && spec.Bounded && (off == in.Len() || after == nil || !after.Has(m.Kind)) {
			ms = ms.Without(m.Kind)
		}
	}
	return ms
}

// Excludes reports whether marks a and b may not share a set: either one
// excludes the other.
func Excludes(s *doc.Schema, a, b doc.MarkKind) bool {
	if a == b {
		return false
	}
	for _, p := range [][2]doc.MarkKind{{a, b}, {b, a}} {
		if spec, ok := s.Mark(p[0]); ok && spec.Excludes != nil && spec.Excludes(p[1]) {
			return true
		}
	}
	return false
}

// excludedBy are the marks adding k removes.
func excludedBy(s *doc.Schema, k doc.MarkKind) []doc.MarkKind {
	var out []doc.MarkKind
	for _, other := range s.MarkKinds() {
		if Excludes(s, k, other) {
			out = append(out, other)
		}
	}
	return out
}

// WithMark is set with m added and the marks m excludes removed: the
// newest mark wins.
func WithMark(s *doc.Schema, set *doc.MarkSet, m doc.Mark) *doc.MarkSet {
	for _, other := range excludedBy(s, m.Kind) {
		set = set.Without(other)
	}
	return set.With(m)
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
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index + 1, Nodes: []*doc.Node{NewBlock(tx.Schema(), kind, attrs)}}); err != nil {
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
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{NewBlock(tx.Schema(), kind, attrs)}}); err != nil {
			return err
		}
		pn, _ := tx.Node(parent)
		container := pn.Slot(slot).Child(index).ID()
		body := bodySlot(tx.Schema(), kind)
		for i, id := range ids {
			if err := tx.Apply(doc.Move{ID: id, Parent: container, Slot: body, Index: i}); err != nil {
				return err
			}
		}
		return nil
	}
}

// bodySlot is the block slot of a container kind.
func bodySlot(s *doc.Schema, kind doc.Kind) int {
	spec, _ := s.Kind(kind)
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
		var moving []doc.ID
		for _, s := range n.Slots() {
			if !s.IsInline() {
				for _, c := range s.Children() {
					moving = append(moving, c.ID())
				}
			}
		}
		for i, c := range moving {
			if err := tx.Apply(doc.Move{ID: c, Parent: parent, Slot: slot, Index: index + i}); err != nil {
				return err
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
		// Moves edit a slot the transaction owns in place: what moves is
		// read before.
		rest := slices.Clone(pn.Slot(slot).Children()[index+1:])
		if err := tx.Apply(doc.Move{ID: id, Parent: grand, Slot: cslot, Index: cindex + 1}); err != nil {
			return err
		}
		if len(rest) > 0 {
			tail := NewBlock(tx.Schema(), cn.Kind(), cn.Attrs())
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
		if n := body.Len(); n > 0 && body.Child(n-1).Kind() == ln.Kind() {
			target = body.Child(n - 1).ID()
		} else {
			if err := tx.Apply(doc.Insert{Parent: prev.ID(), Slot: 1, Index: n, Nodes: []*doc.Node{NewBlock(tx.Schema(), ln.Kind(), ln.Attrs())}}); err != nil {
				return err
			}
			pn, _ := tx.Node(prev.ID())
			target = pn.Slot(1).Child(n).ID()
		}
		tn, _ := tx.Node(target)
		return tx.Apply(doc.Move{ID: item, Parent: target, Slot: 0, Index: tn.Slot(0).Len()})
	}
}

// Outdent moves an item one level out: from a nested list to just after
// the item holding that list, the items after it in its list moving into
// its body to keep their order; an item of a top-level list leaves it as a
// paragraph (Unlist).
func Outdent(item doc.ID) Edit {
	return func(tx *doc.Tx) error {
		list, slot, index, ok := Index(tx, item)
		if !ok {
			return doc.ErrNoNode
		}
		owner, _, _, ok := Index(tx, list)
		if !ok {
			return doc.ErrNoNode
		}
		if on, _ := tx.Node(owner); !tx.Schema().IsEntry(on.Kind()) {
			return Unlist(item)(tx)
		}
		ln, _ := tx.Node(list)
		rest := slices.Clone(ln.Slot(slot).Children()[index+1:])
		outer, oslot, oindex, _ := Index(tx, owner)
		if err := tx.Apply(doc.Move{ID: item, Parent: outer, Slot: oslot, Index: oindex + 1}); err != nil {
			return err
		}
		if len(rest) > 0 {
			n, _ := tx.Node(item)
			at := n.Slot(1).Len()
			if err := tx.Apply(doc.Insert{Parent: item, Slot: 1, Index: at, Nodes: []*doc.Node{NewBlock(tx.Schema(), ln.Kind(), ln.Attrs())}}); err != nil {
				return err
			}
			n, _ = tx.Node(item)
			tail := n.Slot(1).Child(at).ID()
			for i, r := range rest {
				if err := tx.Apply(doc.Move{ID: r.ID(), Parent: tail, Slot: 0, Index: i}); err != nil {
					return err
				}
			}
		}
		return RemoveIfEmpty(list)(tx)
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
// compatible. A block whose start the range holds loses its marker with
// it: from a gap, the rest of the block the range ends in is left
// unmarked (Unmark), and an item holding that block, whose head the range
// holds, leaves its list. Containers left empty are removed.
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
		// The blocks holding the end whose start the range holds: the
		// end's block, and the items around it that hold none of the
		// start, innermost first. Their markers are in the range.
		var second *doc.Node
		var opened []doc.ID
		if toText {
			second, _ = Block(tx, to)
			if second != nil && (second.ID() == from.Node || doc.Contains(tx, second.ID(), from.Node)) {
				second = nil
			}
		}
		if second != nil {
			for _, a := range doc.Ancestors(tx, second.ID()) {
				if a == from.Node || doc.Contains(tx, a, from.Node) {
					break
				}
				opened = append(opened, a)
			}
		}
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
		// An opened item's own text, its head, comes before the end.
		for _, a := range opened {
			n, _ := tx.Node(a)
			for i, sl := range n.Slots() {
				if sl.IsInline() && sl.Len() > 0 {
					if err := tx.Apply(doc.EditText{At: doc.SlotRef{Node: a, Slot: i}, From: 0, To: sl.Len()}); err != nil {
						return err
					}
				}
			}
		}
		if len(between) > 0 {
			if err := tx.Apply(doc.Remove{IDs: between}); err != nil {
				return err
			}
		}
		switch {
		case second == nil:
		case fromText:
			// The rest of the end's block joins the start's.
			if CanJoin(tx, from.Ref(), to.Ref()) {
				if err := Join(from.Ref(), second.ID())(tx); err != nil {
					return err
				}
			}
		default:
			// From a gap, what is left of the end's block is plain text.
			if err := Unmark(second.ID())(tx); err != nil {
				return err
			}
		}
		// Opened items lose their markers, and with them their emptied
		// heads; containers left empty go.
		for _, a := range opened {
			if n, ok := tx.Node(a); ok && tx.Schema().IsEntry(n.Kind()) {
				para, err := unlist(tx, a)
				if err != nil {
					return err
				}
				if pn, ok := tx.Node(para); ok && pn.Slot(0).Len() == 0 {
					if err := tx.Apply(doc.Remove{IDs: []doc.ID{para}}); err != nil {
						return err
					}
				}
			}
		}
		for _, a := range opened {
			if err := RemoveIfEmpty(a)(tx); err != nil {
				return err
			}
		}
		return nil
	}
}

// IsText reports whether p is a position in text, not a gap between
// blocks.
func IsText(r doc.Reader, p doc.Pos) bool { return isInline(r, p.Ref()) }

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

// Copy returns the slice from from to to. Its nodes are the children of
// the deepest block slot holding both ends, each cut where an end falls
// in it, so every container the range passes through comes with what it
// holds: an item copies with its list. The open depths count the nodes
// cut at each edge.
func Copy(r doc.Reader, from, to doc.Pos) Slice {
	if doc.Compare(r, doc.CaretAt(to), doc.CaretAt(from)) < 0 {
		from, to = to, from
	}
	if from.Ref() == to.Ref() && isInline(r, from.Ref()) {
		n, _ := r.Node(from.Node)
		return Slice{Inline: freshItems(n.Slot(from.Slot).Content().Slice(from.Offset, to.Offset))}
	}
	a, b := pathTo(r, liftOut(r, from, false)), pathTo(r, liftOut(r, to, true))
	d := 0
	for d+1 < len(a) && d+1 < len(b) && a[d] == b[d] {
		d++
	}
	if a[d].node != b[d].node || a[d].slot != b[d].slot || isInline(r, doc.SlotRef{Node: a[d].node, Slot: a[d].slot}) {
		// The ends part inside one child: the slot holding it is the
		// deepest that holds both.
		d--
	}
	if d < 0 {
		return Slice{}
	}
	common, _ := r.Node(a[d].node)
	cut, openStart, openEnd := cutNode(common, a[d:], b[d:])
	return Slice{Nodes: cut.Slot(a[d].slot).Children(), OpenStart: openStart - 1, OpenEnd: openEnd - 1}
}

// step is one level of a position's path from the page: a slot of a node
// and an index in it, a child for each level but the last, which is the
// position itself: an offset in text, or a gap.
type step struct {
	node        doc.ID
	slot, index int
}

func pathTo(r doc.Reader, p doc.Pos) []step {
	out := []step{{p.Node, p.Slot, p.Offset}}
	for id := p.Node; ; {
		parent, slot, index, ok := Index(r, id)
		if !ok {
			break
		}
		out = append(out, step{parent, slot, index})
		id = parent
	}
	slices.Reverse(out)
	return out
}

// cutNode copies n between two paths that start at it; a nil path leaves
// that side whole. It returns the copy and how many nodes each side cuts,
// n among them.
func cutNode(n *doc.Node, from, to []step) (*doc.Node, int, int) {
	openStart, openEnd := 0, 0
	slots := make([]doc.Slot, n.NumSlots())
	for si, s := range n.Slots() {
		empty := from != nil && si < from[0].slot || to != nil && si > to[0].slot
		lo, hi := 0, s.Len()
		var inFrom, inTo []step
		if !empty && from != nil && si == from[0].slot {
			lo = from[0].index
			if len(from) > 1 {
				inFrom = from[1:]
			}
		}
		if !empty && to != nil && si == to[0].slot {
			hi = to[0].index
			if len(to) > 1 {
				hi, inTo = hi+1, to[1:]
			}
		}
		switch {
		case s.IsInline() && empty:
			slots[si] = doc.InlineSlot()
		case s.IsInline():
			slots[si] = doc.InlineSlot(freshItems(s.Content().Slice(lo, hi))...)
		case empty:
			slots[si] = doc.BlockSlot()
		default:
			var kids []*doc.Node
			for ci := lo; ci < hi; ci++ {
				var cf, ct []step
				if ci == lo {
					cf = inFrom
				}
				if ci == hi-1 {
					ct = inTo
				}
				if cf == nil && ct == nil {
					kids = append(kids, Fresh(s.Child(ci)))
					continue
				}
				k, os, oe := cutNode(s.Child(ci), cf, ct)
				if cf != nil {
					openStart = os
				}
				if ct != nil {
					openEnd = oe
				}
				kids = append(kids, k)
			}
			slots[si] = doc.BlockSlot(kids...)
		}
	}
	if from != nil {
		openStart++
	}
	if to != nil {
		openEnd++
	}
	return doc.NewNode(n.Kind(), n.Attrs(), slots...), openStart, openEnd
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

// InsertSlice pastes a slice at a text position. Inline items go in place.
// Blocks land in the nearest slot around the position's block that takes
// them all, loose items gathered into lists where none does: the block is
// split at the position and each container up to that slot after it, and
// the blocks go between the halves. An open edge joins its text to the
// half beside it where the two are text blocks that can join, and a half
// the split left empty goes.
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
		nodes := s.Nodes
		target, ok := landing(tx, block.ID(), nodes)
		if !ok {
			nodes = listed(tx.Schema(), nodes)
			if target, ok = landing(tx, block.ID(), nodes); !ok {
				return fmt.Errorf("%w: no slot takes the pasted blocks", ErrHelper)
			}
		}
		if err := Split(at, block.Kind(), block.Attrs())(tx); err != nil {
			return err
		}
		right := nextSibling(tx, block.ID())
		// Split each container up to the landing slot after the left half.
		left := block.ID()
		for {
			parent, slot, index, _ := Index(tx, left)
			if parent == target.Node && slot == target.Slot {
				break
			}
			if err := splitAfter(tx, parent, slot, index); err != nil {
				return err
			}
			left = parent
		}
		_, _, index, _ := Index(tx, left)
		if err := tx.Apply(doc.Insert{Parent: target.Node, Slot: target.Slot, Index: index + 1, Nodes: nodes}); err != nil {
			return err
		}
		tn, _ := tx.Node(target.Node)
		first := tn.Slot(target.Slot).Child(index + 1).ID()
		last := tn.Slot(target.Slot).Child(index + len(nodes)).ID()
		direct := left == block.ID()
		if s.OpenEnd > 0 && direct && right != 0 {
			if err := joinText(tx, last, right, false); err != nil {
				return err
			}
		}
		if s.OpenStart > 0 && direct {
			if err := joinText(tx, block.ID(), first, true); err != nil {
				return err
			}
		}
		for _, half := range []doc.ID{block.ID(), right} {
			if err := dropEmptyText(tx, half); err != nil {
				return err
			}
		}
		return nil
	}
}

// landing is the nearest block slot around a block that takes every node.
func landing(r doc.Reader, block doc.ID, nodes []*doc.Node) (doc.SlotRef, bool) {
	for id := block; ; {
		parent, slot, _, ok := Index(r, id)
		if !ok {
			return doc.SlotRef{}, false
		}
		ref := doc.SlotRef{Node: parent, Slot: slot}
		if spec, ok := SlotSpec(r, ref); ok && takesAll(spec, nodes) {
			return ref, true
		}
		id = parent
	}
}

func takesAll(spec doc.SlotSpec, nodes []*doc.Node) bool {
	for _, n := range nodes {
		if spec.Accepts == nil || !spec.Accepts(n.Kind()) {
			return false
		}
	}
	return true
}

// listed gathers each run of loose entries into a new container of the
// kind that holds them.
func listed(s *doc.Schema, nodes []*doc.Node) []*doc.Node {
	var out, run []*doc.Node
	flush := func() {
		if len(run) > 0 {
			if k, ok := containerOf(s, run[0].Kind()); ok {
				out = append(out, doc.NewNode(k, doc.Attrs{}, doc.BlockSlot(run...)))
			} else {
				out = append(out, run...)
			}
			run = nil
		}
	}
	for _, n := range nodes {
		if s.IsEntry(n.Kind()) {
			if len(run) > 0 && run[0].Kind() != n.Kind() {
				flush()
			}
			run = append(run, n)
			continue
		}
		flush()
		out = append(out, n)
	}
	flush()
	return out
}

// splitAfter moves the children of a container's slot after index into a
// new container of its kind just after it; with none after, nothing
// moves.
func splitAfter(tx *doc.Tx, container doc.ID, slot, index int) error {
	n, _ := tx.Node(container)
	children := n.Slot(slot).Children()
	if index+1 >= len(children) {
		return nil
	}
	var moving []doc.ID
	for _, c := range children[index+1:] {
		moving = append(moving, c.ID())
	}
	parent, pslot, pindex, ok := Index(tx, container)
	if !ok {
		return fmt.Errorf("%w: split of the page", ErrHelper)
	}
	if err := tx.Apply(doc.Insert{Parent: parent, Slot: pslot, Index: pindex + 1, Nodes: []*doc.Node{NewBlock(tx.Schema(), n.Kind(), n.Attrs())}}); err != nil {
		return err
	}
	pn, _ := tx.Node(parent)
	created := pn.Slot(pslot).Child(pindex + 1).ID()
	for i, id := range moving {
		if err := tx.Apply(doc.Move{ID: id, Parent: created, Slot: slot, Index: i}); err != nil {
			return err
		}
	}
	return nil
}

// nextSibling is the node after id in its slot, or 0.
func nextSibling(r doc.Reader, id doc.ID) doc.ID {
	parent, slot, index, ok := Index(r, id)
	if !ok {
		return 0
	}
	pn, _ := r.Node(parent)
	if index+1 >= pn.Slot(slot).Len() {
		return 0
	}
	return pn.Slot(slot).Child(index + 1).ID()
}

// joinText joins second onto first where both are text blocks that can
// join: first's last text slot (its own, when own) takes second's text.
func joinText(tx *doc.Tx, first, second doc.ID, own bool) error {
	fn, ok1 := tx.Node(first)
	sn, ok2 := tx.Node(second)
	if !ok1 || !ok2 {
		return nil
	}
	ref, ok := LastTextSlot(fn)
	if own {
		ref, ok = FirstTextSlot(fn)
	}
	src, sok := FirstTextSlot(sn)
	if !ok || !sok || src.Node != second || (own && ref.Node != first) || !CanJoin(tx, ref, src) {
		return nil
	}
	// Second's children move into the body of ref's block, so a block
	// without a body takes no children.
	if target, _ := tx.Node(ref.Node); hasBlocks(sn) && !holdsBlocks(tx.Schema(), target.Kind()) {
		return nil
	}
	return Join(ref, second)(tx)
}

// holdsBlocks reports whether a kind has a block slot.
func holdsBlocks(s *doc.Schema, kind doc.Kind) bool {
	spec, _ := s.Kind(kind)
	for _, s := range spec.Slots {
		if s.Blocks {
			return true
		}
	}
	return false
}

// hasBlocks reports whether a node holds any block children.
func hasBlocks(n *doc.Node) bool {
	for _, s := range n.Slots() {
		if !s.IsInline() && s.Len() > 0 {
			return true
		}
	}
	return false
}

// dropEmptyText removes a text block left with no text and no children,
// and then containers left empty.
func dropEmptyText(tx *doc.Tx, id doc.ID) error {
	n, ok := tx.Node(id)
	if !ok {
		return nil
	}
	for _, s := range n.Slots() {
		if s.Len() > 0 {
			return nil
		}
	}
	parent, _, ok := tx.Parent(id)
	if !ok {
		return nil
	}
	if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
		return err
	}
	return RemoveIfEmpty(parent)(tx)
}

// Retype changes a text block's kind and attributes and fits its text to
// the new kind's slot: between a rich and a literal slot the text becomes
// plain, marks and inline nodes dropping to their text, and where the new
// slot keeps no lines, line breaks become spaces. Attributes are stored
// canonically.
func Retype(id doc.ID, kind doc.Kind, attrs doc.Attrs) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		s := tx.Schema()
		from, fok := s.Kind(n.Kind())
		to, tok := s.Kind(kind)
		if !fok || !tok || len(from.Slots) == 0 || len(to.Slots) == 0 {
			return fmt.Errorf("%w: %s cannot become %s", ErrHelper, s.KindName(n.Kind()), s.KindName(kind))
		}
		if from.Slots[0].Literal != to.Slots[0].Literal {
			if err := plainText(tx, n); err != nil {
				return err
			}
		}
		if !to.Slots[0].Lines {
			if err := unbreak(tx, doc.SlotRef{Node: id, Slot: 0}); err != nil {
				return err
			}
		}
		return tx.Apply(doc.SetNode{ID: id, Kind: kind, Attrs: attrs})
	}
}

// plainText replaces a block's first slot with its text, unmarked.
func plainText(tx *doc.Tx, n *doc.Node) error {
	content := n.Slot(0).Content()
	if content.Len() == 0 {
		return nil
	}
	var items []doc.Item
	if text := content.Text(); text != "" {
		items = []doc.Item{doc.TextItem(text, nil)}
	}
	return tx.Apply(doc.EditText{At: doc.SlotRef{Node: n.ID(), Slot: 0}, From: 0, To: content.Len(), Items: items})
}

// unbreak turns the line breaks of an inline slot into spaces, keeping
// their marks.
func unbreak(tx *doc.Tx, ref doc.SlotRef) error {
	n, ok := tx.Node(ref.Node)
	if !ok {
		return doc.ErrNoNode
	}
	content := n.Slot(ref.Slot).Content()
	flat := content.Flat()
	for off := strings.IndexByte(flat, '\n'); off >= 0; {
		it, _, _ := content.ItemAt(off)
		if err := tx.Apply(doc.EditText{At: ref, From: off, To: off + 1, Items: []doc.Item{doc.TextItem(" ", it.Marks)}}); err != nil {
			return err
		}
		next := strings.IndexByte(flat[off+1:], '\n')
		if next < 0 {
			break
		}
		off += 1 + next
	}
	return nil
}

// Unmark takes a text block's marker away, leaving its text a plain block
// where it stands: a heading, code or math block is retyped, and an entry
// leaves its container. A plain block has no marker and is left as it is,
// as is a block whose slot takes no plain block, such as a cell.
func Unmark(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		s := tx.Schema()
		switch k := n.Kind(); {
		case s.IsEntry(k):
			return Unlist(id)(tx)
		case k == s.Plain() || !s.IsTextBlock(k) || s.IsObject(k) || !Accepts(tx, id, s.Plain()):
			return nil
		}
		return Retype(id, s.Plain(), doc.Attrs{})(tx)
	}
}

// Accepts reports whether the slot holding id takes kind.
func Accepts(r doc.Reader, id doc.ID, kind doc.Kind) bool {
	parent, slot, ok := r.Parent(id)
	if !ok {
		return false
	}
	spec, ok := SlotSpec(r, doc.SlotRef{Node: parent, Slot: slot})
	return ok && spec.Accepts != nil && spec.Accepts(kind)
}

// containerOf is the kind whose block slot takes entries of kind, first in
// declaration order.
func containerOf(s *doc.Schema, entry doc.Kind) (doc.Kind, bool) {
	for _, k := range s.Kinds() {
		spec, _ := s.Kind(k)
		for _, sl := range spec.Slots {
			if sl.Blocks && sl.Accepts != nil && sl.Accepts(entry) {
				return k, true
			}
		}
	}
	return 0, false
}

// Unlist takes an entry out of its container as a plain block holding its
// head, followed by its body's blocks. The container splits around it, and
// is removed when it is left empty.
func Unlist(item doc.ID) Edit {
	return func(tx *doc.Tx) error {
		_, err := unlist(tx, item)
		return err
	}
}

// unlist is Unlist, returning the paragraph the item became.
func unlist(tx *doc.Tx, item doc.ID) (doc.ID, error) {
	if err := Lift(item)(tx); err != nil {
		return 0, err
	}
	parent, slot, index, ok := Index(tx, item)
	if !ok {
		return 0, doc.ErrNoNode
	}
	if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{NewBlock(tx.Schema(), tx.Schema().Plain(), doc.Attrs{})}}); err != nil {
		return 0, err
	}
	pn, _ := tx.Node(parent)
	para := pn.Slot(slot).Child(index).ID()
	n, _ := tx.Node(item)
	if l := n.Slot(0).Len(); l > 0 {
		if err := tx.Apply(doc.Transfer{From: doc.SlotRef{Node: item}, Start: 0, End: l, To: doc.SlotRef{Node: para}, At: 0}); err != nil {
			return 0, err
		}
	}
	n, _ = tx.Node(item)
	if err := moveChildrenAfter(tx, n, para); err != nil {
		return 0, err
	}
	return para, tx.Apply(doc.Remove{IDs: []doc.ID{item}})
}

// WrapRun is the run of siblings that wrapping ids in a container of kind
// takes: each id lifts to its nearest ancestor-or-self the container
// holds and whose slot holds the container, as a list for one of its
// items, then to a common slot, and the run spans every sibling between
// them. It is false when no such run exists.
func WrapRun(r doc.Reader, ids []doc.ID, kind doc.Kind) ([]doc.ID, bool) {
	spec, ok := r.Schema().Kind(kind)
	if !ok || len(ids) == 0 {
		return nil, false
	}
	body := spec.Slots[bodySlot(r.Schema(), kind)]
	fits := func(id doc.ID) bool {
		n, ok := r.Node(id)
		if !ok || body.Accepts == nil || !body.Accepts(n.Kind()) {
			return false
		}
		parent, slot, ok := r.Parent(id)
		if !ok {
			return false
		}
		ps, ok := SlotSpec(r, doc.SlotRef{Node: parent, Slot: slot})
		return ok && ps.Blocks && ps.Accepts != nil && ps.Accepts(kind)
	}
	lift := func(id doc.ID) (doc.ID, bool) {
		for !fits(id) {
			parent, _, ok := r.Parent(id)
			if !ok {
				return 0, false
			}
			id = parent
		}
		return id, true
	}
	depth := func(id doc.ID) int {
		d := 0
		for {
			parent, _, ok := r.Parent(id)
			if !ok {
				return d
			}
			id, d = parent, d+1
		}
	}
	targets := make([]doc.ID, len(ids))
	for i, id := range ids {
		t, ok := lift(id)
		if !ok {
			return nil, false
		}
		targets[i] = t
	}
	for {
		p0, s0, _ := r.Parent(targets[0])
		deepest, same := 0, true
		for i, t := range targets {
			p, s, _ := r.Parent(t)
			if p != p0 || s != s0 {
				same = false
			}
			if depth(t) > depth(targets[deepest]) {
				deepest = i
			}
		}
		if same {
			break
		}
		parent, _, ok := r.Parent(targets[deepest])
		if !ok {
			return nil, false
		}
		t, ok := lift(parent)
		if !ok {
			return nil, false
		}
		targets[deepest] = t
	}
	parent, slot, _ := r.Parent(targets[0])
	lo, hi := -1, -1
	for _, t := range targets {
		_, _, i, ok := Index(r, t)
		if !ok {
			return nil, false
		}
		if lo < 0 || i < lo {
			lo = i
		}
		if i > hi {
			hi = i
		}
	}
	pn, _ := r.Node(parent)
	run := make([]doc.ID, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		run = append(run, pn.Slot(slot).Child(i).ID())
	}
	return run, true
}

// PlainText is a slice's text: blocks on lines of their own.
func PlainText(s Slice) string {
	var b strings.Builder
	inline := func(items []doc.Item) {
		for _, it := range items {
			if it.Node != nil {
				b.WriteString(it.Node.Text())
			} else {
				b.WriteString(it.Text)
			}
		}
	}
	inline(s.Inline)
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		if n.NumSlots() > 0 && n.Slot(0).IsInline() {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
		}
		for _, sl := range n.Slots() {
			if sl.IsInline() {
				var items []doc.Item
				sl.Content().Each(func(_ int, it doc.Item) bool { items = append(items, it); return true })
				inline(items)
				continue
			}
			for _, c := range sl.Children() {
				walk(c)
			}
		}
	}
	for _, n := range s.Nodes {
		walk(n)
	}
	return b.String()
}

// SelectedBlocks are the text blocks a range's text touches, in order; a
// collapsed range touches the block it is in.
func SelectedBlocks(r doc.Reader, from, to doc.Pos) []*doc.Node {
	var out []*doc.Node
	for _, s := range TextSpans(r, from, to) {
		b, ok := Block(r, doc.At(s.Slot, s.From))
		if ok && (len(out) == 0 || out[len(out)-1] != b) {
			out = append(out, b)
		}
	}
	return out
}

// RemoveNode takes a block out of its slot, or an inline structure out of
// its slot's items.
func RemoveNode(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		parent, slot, ok := tx.Parent(id)
		if !ok {
			return doc.ErrNoNode
		}
		pn, _ := tx.Node(parent)
		if !pn.Slot(slot).IsInline() {
			if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
				return err
			}
			// The page keeps a block to type in when its last structure
			// goes. Other slots keep their own empty form.
			pn, _ = tx.Node(parent)
			if pn.Kind() == tx.Schema().Root() && pn.Slot(slot).Len() == 0 {
				return tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: 0, Nodes: []*doc.Node{NewBlock(tx.Schema(), tx.Schema().Plain(), doc.Attrs{})}})
			}
			return nil
		}
		off, ok := pn.Slot(slot).Content().NodeOffset(id)
		if !ok {
			return doc.ErrNoNode
		}
		return tx.Apply(doc.EditText{
			At: doc.SlotRef{Node: parent, Slot: slot}, From: off, To: off + 1,
		})
	}
}

// BlockCaret is the caret at the start (Backward) or end (Forward) of the
// first text of the block at gap; false when no block or text is there.
func BlockCaret(r doc.Reader, gap doc.Pos, edge Direction) (doc.Caret, bool) {
	n, ok := r.Node(gap.Node)
	if !ok || gap.Slot >= n.NumSlots() || gap.Offset < 0 || gap.Offset >= n.Slot(gap.Slot).Len() {
		return doc.Caret{}, false
	}
	ref, ok := FirstTextSlot(n.Slot(gap.Slot).Child(gap.Offset))
	if !ok {
		return doc.Caret{}, false
	}
	off := 0
	if edge > 0 {
		owner, _ := r.Node(ref.Node)
		off = owner.Slot(ref.Slot).Len()
	}
	return doc.CaretAt(doc.At(ref, off)), true
}
