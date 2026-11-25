package store

import (
	"bytes"
	"cmp"
	"fmt"
	"maps"
	"slices"

	"github.com/caelyreth/neverland/internal/editor/doc"
)

// Row is one stored node: a node held in a block slot, or the root.
type Row struct {
	ID     doc.ID
	Parent doc.ID // zero for the root
	Slot   int
	Ord    string
	Kind   doc.Kind
	Attrs  []byte
	// Text is the plain text of the node's inline slots in reading order,
	// nested inline nodes' slots included; Runs says how it divides.
	Text string
	Runs []byte
}

// Encode makes a node's row.
func Encode(n *doc.Node, parent doc.ID, slot int) Row {
	w := &writer{}
	var text []byte
	encodeSlots(w, &text, n)
	// Empty encodings are empty, never nil, so they store as blobs.
	attrs := append([]byte{}, EncodeAttrs(n.Attrs())...)
	return Row{ID: n.ID(), Parent: parent, Slot: slot, Ord: n.Ord(), Kind: n.Kind(),
		Attrs: attrs, Text: string(text), Runs: append([]byte{}, w.b...)}
}

// Rows encodes every stored node of a snapshot, the root first.
func Rows(s *doc.Snapshot) []Row {
	var out []Row
	var walk func(n *doc.Node, parent doc.ID, slot int)
	walk = func(n *doc.Node, parent doc.ID, slot int) {
		out = append(out, Encode(n, parent, slot))
		for i, sl := range n.Slots() {
			if sl.IsInline() {
				continue
			}
			for _, c := range sl.Children() {
				walk(c, n.ID(), i)
			}
		}
	}
	walk(s.Root(), 0, 0)
	return out
}

// Diff is a save: the rows to upsert and the ids to delete, found by a
// pointer diff that skips subtrees the snapshots share.
type Diff struct {
	Upsert []Row
	Delete []doc.ID
}

// Empty reports a diff with nothing to write.
func (d Diff) Empty() bool { return len(d.Upsert) == 0 && len(d.Delete) == 0 }

// Changes diffs two snapshots of one page; prev nil saves everything.
func Changes(prev, next *doc.Snapshot) Diff {
	if prev == nil {
		return Diff{Upsert: Rows(next)}
	}
	var d Diff
	seen := map[doc.ID]bool{}
	var walk func(n *doc.Node, parent doc.ID, slot int)
	walk = func(n *doc.Node, parent doc.ID, slot int) {
		seen[n.ID()] = true
		old, ok := prev.Node(n.ID())
		if ok && old == n {
			if pp, ps, _ := prev.Parent(n.ID()); pp == parent && ps == slot {
				markShared(n, seen)
				return
			}
		}
		pp, ps, _ := prev.Parent(n.ID())
		if !ok || old.Kind() != n.Kind() || !old.Attrs().Equal(n.Attrs()) || old.Ord() != n.Ord() ||
			pp != parent || ps != slot || !sameInline(old, n) {
			d.Upsert = append(d.Upsert, Encode(n, parent, slot))
		}
		for i, sl := range n.Slots() {
			if sl.IsInline() {
				continue
			}
			for _, c := range sl.Children() {
				walk(c, n.ID(), i)
			}
		}
	}
	walk(next.Root(), 0, 0)
	var gone func(n *doc.Node)
	gone = func(n *doc.Node) {
		if !seen[n.ID()] {
			d.Delete = append(d.Delete, n.ID())
		}
		for _, sl := range n.Slots() {
			if !sl.IsInline() {
				for _, c := range sl.Children() {
					gone(c)
				}
			}
		}
	}
	gone(prev.Root())
	slices.Sort(d.Delete)
	return d
}

// markShared marks a shared subtree's stored nodes as present.
func markShared(n *doc.Node, seen map[doc.ID]bool) {
	seen[n.ID()] = true
	for _, sl := range n.Slots() {
		if !sl.IsInline() {
			for _, c := range sl.Children() {
				markShared(c, seen)
			}
		}
	}
}

// sameInline reports whether two versions of a node have the same inline
// content: the same slot pointers mean the same content.
func sameInline(a, b *doc.Node) bool {
	if a.NumSlots() != b.NumSlots() {
		return false
	}
	for i := range a.NumSlots() {
		sa, sb := a.Slot(i), b.Slot(i)
		if sa.IsInline() != sb.IsInline() {
			return false
		}
		if sa.IsInline() && sa.Content() != sb.Content() {
			return false
		}
	}
	return true
}

// Load builds a page from its rows, bottom-up, and validates every node.
// A page that fails is not opened.
func Load(schema *doc.Schema, marks *doc.Interner, root doc.ID, seed uint64, rows []Row) (*doc.Snapshot, error) {
	byParent := map[doc.ID][]Row{}
	var rootRow *Row
	for i := range rows {
		r := rows[i]
		if r.ID == root {
			rootRow = &rows[i]
			continue
		}
		byParent[r.Parent] = append(byParent[r.Parent], r)
	}
	if rootRow == nil {
		return nil, corrupt(root, "no root row")
	}
	built := 0
	var build func(r Row) (*doc.Node, error)
	build = func(r Row) (*doc.Node, error) {
		attrs, err := DecodeAttrs(r.Attrs, knownFields(schema, r.Kind))
		if err != nil {
			return nil, corrupt(r.ID, "attributes")
		}
		rd := &reader{b: r.Runs}
		text := r.Text
		inline := decodeSlots(rd, &text, schema, marks, r.Kind)
		if rd.err != nil || len(rd.b) != 0 || text != "" {
			return nil, corrupt(r.ID, "runs")
		}
		children := byParent[r.ID]
		slices.SortFunc(children, func(a, b Row) int {
			if a.Slot != b.Slot {
				return a.Slot - b.Slot
			}
			switch {
			case a.Ord < b.Ord:
				return -1
			case a.Ord > b.Ord:
				return 1
			}
			return 0
		})
		blocks := map[int][]*doc.Node{}
		for _, c := range children {
			if c.Slot < 0 || c.Slot >= maxSlots {
				return nil, corrupt(c.ID, "slot")
			}
			cn, err := build(c)
			if err != nil {
				return nil, err
			}
			blocks[c.Slot] = append(blocks[c.Slot], cn)
		}
		built++
		return buildNode(r.ID, r.Kind, attrs, r.Ord, inline, blocks, schema), nil
	}
	n, err := build(*rootRow)
	if err != nil {
		return nil, err
	}
	if built != len(rows) {
		return nil, corrupt(root, "rows outside the tree")
	}
	return doc.NewSnapshot(schema, marks, n, seed)
}

// Verify checks a save before it is written: the rows d leaves over
// prev's, as a store holding prev would hold them, load back as next does,
// valid in every node and the same page row for row. A save that fails is
// not to be written: the store keeps prev.
func Verify(prev, next *doc.Snapshot, d Diff) error {
	held := map[doc.ID]Row{}
	if prev != nil {
		for _, r := range Rows(prev) {
			held[r.ID] = r
		}
	}
	for _, id := range d.Delete {
		delete(held, id)
	}
	for _, r := range d.Upsert {
		held[r.ID] = r
	}
	rows := slices.Collect(maps.Values(held))
	loaded, err := Load(next.Schema(), doc.NewInterner(), next.Root().ID(), next.Seed(), rows)
	if err != nil {
		return err
	}
	got, want := Rows(loaded), Rows(next)
	byID := func(a, b Row) int { return cmp.Compare(a.ID, b.ID) }
	slices.SortFunc(got, byID)
	slices.SortFunc(want, byID)
	if !slices.EqualFunc(got, want, sameRow) {
		return fmt.Errorf("%w: the saved rows load as another page", ErrCorrupt)
	}
	return nil
}

func sameRow(a, b Row) bool {
	return a.ID == b.ID && a.Parent == b.Parent && a.Slot == b.Slot && a.Ord == b.Ord && a.Kind == b.Kind &&
		bytes.Equal(a.Attrs, b.Attrs) && a.Text == b.Text && bytes.Equal(a.Runs, b.Runs)
}
