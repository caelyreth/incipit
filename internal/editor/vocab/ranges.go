package vocab

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
)

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
		// start, innermost first. Their markers are in the range. An end
		// at a gap opens the container holding the gap and those around
		// it alike.
		var second *doc.Node
		var opened []doc.ID
		if toText {
			second, _ = Block(tx, to)
			if second != nil && (second.ID() == from.Node || doc.Contains(tx, second.ID(), from.Node)) {
				second = nil
			}
		}
		var around []doc.ID
		switch {
		case second != nil:
			around = doc.Ancestors(tx, second.ID())
		case !toText:
			around = append([]doc.ID{to.Node}, doc.Ancestors(tx, to.Node)...)
		}
		for _, a := range around {
			if a == from.Node || doc.Contains(tx, a, from.Node) {
				break
			}
			opened = append(opened, a)
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
		if !fromText {
			// From a gap, the container holding it may be left empty too.
			return RemoveIfEmpty(from.Node)(tx)
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
