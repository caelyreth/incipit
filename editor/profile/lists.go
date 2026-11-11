package profile

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// IntoList puts a text block into an item, joining adjacent lists of the
// same style. Moving its text keeps the selection on what was selected.
func IntoList(id doc.ID, attrs doc.Attrs) vocab.Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		attrs = tx.Schema().Canonical(List, attrs)
		if n.Kind() == Item {
			list, _, _, ok := vocab.Index(tx, id)
			if !ok {
				return doc.ErrNoNode
			}
			if err := tx.Apply(doc.SetNode{ID: list, Kind: List, Attrs: attrs}); err != nil {
				return err
			}
			return JoinLists(tx, list)
		}
		if !tx.Schema().IsTextBlock(n.Kind()) {
			return nil
		}
		spec, ok := vocab.SlotSpec(tx, doc.SlotRef{Node: id})
		if !ok || spec.Blocks || spec.Literal {
			return nil
		}
		parent, slot, index, ok := vocab.Index(tx, id)
		if !ok {
			return doc.ErrNoNode
		}
		ps, ok := vocab.SlotSpec(tx, doc.SlotRef{Node: parent, Slot: slot})
		if !ok || !ps.Blocks || ps.Accepts == nil || !ps.Accepts(List) {
			return nil
		}

		item := doc.NewNode(Item, doc.Attrs{}, doc.InlineSlot(), doc.BlockSlot())
		list := doc.NewNode(List, attrs, doc.BlockSlot(item))
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{list}}); err != nil {
			return err
		}
		pn, _ := tx.Node(parent)
		listID := pn.Slot(slot).Child(index).ID()
		ln, _ := tx.Node(listID)
		itemID := ln.Slot(0).Child(0).ID()
		sn, _ := tx.Node(id)
		if length := sn.Slot(0).Len(); length > 0 {
			if err := tx.Apply(doc.Transfer{From: doc.SlotRef{Node: id}, Start: 0, End: length,
				To: doc.SlotRef{Node: itemID}, At: 0}); err != nil {
				return err
			}
		}
		if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
			return err
		}
		return JoinLists(tx, listID)
	}
}

// JoinLists makes adjacent lists of one style one list, keeping their
// items in reading order. A list joins the one before it first.
func JoinLists(tx *doc.Tx, id doc.ID) error {
	for {
		ln, ok := tx.Node(id)
		if !ok {
			return nil
		}
		parent, slot, index, ok := vocab.Index(tx, id)
		if !ok {
			return doc.ErrNoNode
		}
		pn, _ := tx.Node(parent)
		children := pn.Slot(slot).Children()
		if index > 0 {
			prev := children[index-1]
			if prev.Kind() == List && sameListStyle(prev, ln) {
				if err := moveItems(tx, id, prev.ID()); err != nil {
					return err
				}
				if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
					return err
				}
				id = prev.ID()
				continue
			}
		}
		if index+1 < len(children) {
			next := children[index+1]
			if next.Kind() == List && sameListStyle(ln, next) {
				if err := moveItems(tx, next.ID(), id); err != nil {
					return err
				}
				if err := tx.Apply(doc.Remove{IDs: []doc.ID{next.ID()}}); err != nil {
					return err
				}
				continue
			}
		}
		return nil
	}
}

// moveItems appends from's items to to and leaves from empty.
func moveItems(tx *doc.Tx, from, to doc.ID) error {
	fn, _ := tx.Node(from)
	tn, _ := tx.Node(to)
	items := fn.Slot(0).Children()
	at := tn.Slot(0).Len()
	for i, item := range items {
		if err := tx.Apply(doc.Move{ID: item.ID(), Parent: to, Slot: 0, Index: at + i}); err != nil {
			return err
		}
	}
	return nil
}

func sameListStyle(a, b *doc.Node) bool {
	return a.Kind() == List && b.Kind() == List && ListStyleOf(a) == ListStyleOf(b)
}

// ToggleTask completes an open task item or reopens a done one; an item
// that is no task is left alone.
func ToggleTask(id doc.ID) vocab.Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok || n.Kind() != Item || TaskOf(n) == NoTask {
			return nil
		}
		next := TaskDone
		if TaskOf(n) == TaskDone {
			next = TaskOpen
		}
		return tx.Apply(doc.SetNode{ID: id, Kind: Item, Attrs: n.Attrs().With(FieldTask, doc.Int(int64(next)))})
	}
}
