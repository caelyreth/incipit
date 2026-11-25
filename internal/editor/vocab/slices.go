package vocab

import (
	"fmt"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/editor/doc"
)

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

// Fit adapts a slice to the slot at ref it lands in. Rich text takes it as
// it is: inline items in place, blocks split around. Into a literal slot,
// or an object's text, around which no blocks land as its text never
// leaves it (a cell's), it is plain text, with line breaks only where the
// slot keeps them.
func Fit(r doc.Reader, ref doc.SlotRef, s Slice) Slice {
	spec, _ := SlotSpec(r, ref)
	if !spec.Literal && (!inObject(r, ref.Node) || len(s.Nodes) == 0) {
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
func InsertSlice(at doc.Pos, s Slice) Edit { return PasteSlice(at, s, nil) }

// PasteSlice is InsertSlice, setting end, where one is given, to where the
// pasted content ends: where a caret stands after a paste, before what
// followed the position. It stays unset where the content ends in no text.
func PasteSlice(at doc.Pos, s Slice, end *doc.Pos) Edit {
	return func(tx *doc.Tx) error {
		spec, ok := SlotSpec(tx, at.Ref())
		if !ok || spec.Blocks {
			return fmt.Errorf("%w: paste outside a text slot", ErrHelper)
		}
		s = Fit(tx, at.Ref(), s)
		if len(s.Nodes) == 0 {
			items := s.Inline
			if spec.Literal {
				for i := range items {
					items[i].Marks = nil
				}
			}
			if err := tx.Apply(doc.EditText{At: at.Ref(), From: at.Offset, To: at.Offset, Items: items}); err != nil {
				return err
			}
			if end != nil {
				*end = doc.Pos{Node: at.Node, Slot: at.Slot, Offset: at.Offset + itemsLen(items)}
			}
			return nil
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
		// tail is the text of the right half that joins the last block, which
		// the caret stands before.
		tail := 0
		if s.OpenEnd > 0 && direct && right != 0 {
			before := textLen(tx, right)
			if err := joinText(tx, last, right, false); err != nil {
				return err
			}
			if _, ok := tx.Node(right); !ok {
				tail = before
			}
		}
		if s.OpenStart > 0 && direct {
			if err := joinText(tx, block.ID(), first, true); err != nil {
				return err
			}
			if _, ok := tx.Node(first); !ok && first == last {
				last = block.ID()
			}
		}
		if end != nil {
			if n, ok := tx.Node(last); ok {
				if ref, ok := LastTextSlot(n); ok {
					tn, _ := tx.Node(ref.Node)
					*end = doc.Pos{Node: ref.Node, Slot: ref.Slot, Offset: tn.Slot(ref.Slot).Len() - tail}
				}
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

// textLen is how far the first text slot of block id reaches, or 0.
func textLen(r doc.Reader, id doc.ID) int {
	n, ok := r.Node(id)
	if !ok {
		return 0
	}
	ref, ok := FirstTextSlot(n)
	if !ok || ref.Node != id {
		return 0
	}
	return n.Slot(ref.Slot).Len()
}

// AsEntries are plain text blocks, as lines of text pasted into an entry,
// made entries of the entry's kind, so they stand in its container beside
// it rather than after the whole container. Anything else is as given.
func AsEntries(s *doc.Schema, block *doc.Node, nodes []*doc.Node) []*doc.Node {
	if block == nil || !s.IsEntry(block.Kind()) {
		return nodes
	}
	for _, n := range nodes {
		if n.Kind() != s.Plain() {
			return nodes
		}
	}
	out := make([]*doc.Node, len(nodes))
	for i, n := range nodes {
		text := n.Slot(0).Content()
		out[i] = NewBlock(s, block.Kind(), doc.Attrs{}, text.Slice(0, text.Len())...)
	}
	return out
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
	// The item's head is the paragraph's text, empty or not.
	if err := tx.Apply(doc.Transfer{From: doc.SlotRef{Node: item}, Start: 0, End: n.Slot(0).Len(), To: doc.SlotRef{Node: para}, At: 0, Whole: true}); err != nil {
		return 0, err
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

// PlainText is a slice's text: blocks on lines of their own, a grid's
// rows on lines and its cells between tabs.
func PlainText(schema *doc.Schema, s Slice) string {
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
		if g, ok := GridNode(schema, n); ok {
			// A grid reads as a spreadsheet copies one: its rows on lines,
			// its cells between tabs, each cell on one line.
			for _, row := range g.Rows {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				for j, cell := range row.Slot(0).Children() {
					if j > 0 {
						b.WriteByte('\t')
					}
					b.WriteString(strings.Join(strings.Fields(PlainText(schema, Slice{Nodes: []*doc.Node{cell}})), " "))
				}
			}
			return
		}
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

// PlaceGap is where blocks of kind placed at a position go: a gap as it
// is; in an object's text, after the object; in an empty plain block, in
// its place; at a text's start or end, before or after its block, and
// inside it, between its halves. A block whose slot takes no such block,
// as an item's head or a cell is, takes them at the start of its own
// body; failing that, its nearest container that does, after it.
func PlaceGap(tx *doc.Tx, at doc.Pos, kind doc.Kind) (doc.Pos, error) {
	s := tx.Schema()
	if spec, ok := SlotSpec(tx, at.Ref()); ok && spec.Blocks {
		if spec.Accepts != nil && spec.Accepts(kind) {
			return at, nil
		}
	}
	block, ok := Block(tx, at)
	if !ok {
		return doc.Pos{}, ErrHelper
	}
	parent, slot, index, _ := Index(tx, block.ID())
	if !Accepts(tx, block.ID(), kind) {
		for i, sl := range block.Slots() {
			if spec, ok := SlotSpec(tx, doc.SlotRef{Node: block.ID(), Slot: i}); ok && !sl.IsInline() && spec.Accepts != nil && spec.Accepts(kind) {
				return doc.Pos{Node: block.ID(), Slot: i}, nil
			}
		}
		for _, a := range doc.Ancestors(tx, block.ID())[1:] {
			if Accepts(tx, a, kind) {
				p, sl, i, _ := Index(tx, a)
				return doc.Pos{Node: p, Slot: sl, Offset: i + 1}, nil
			}
		}
		return doc.Pos{}, ErrHelper
	}
	after := doc.Pos{Node: parent, Slot: slot, Offset: index + 1}
	if s.IsObject(block.Kind()) || at.Node != block.ID() {
		// An object's text, or an inline node's source: after its block.
		return after, nil
	}
	text := block.Slot(at.Slot).Len()
	switch {
	case block.Kind() == s.Plain() && text == 0 && block.NumSlots() == 1:
		if err := tx.Apply(doc.Remove{IDs: []doc.ID{block.ID()}}); err != nil {
			return doc.Pos{}, err
		}
		return doc.Pos{Node: parent, Slot: slot, Offset: index}, nil
	case at.Offset == 0 && at.Slot == 0 && text > 0:
		return doc.Pos{Node: parent, Slot: slot, Offset: index}, nil
	case at.Offset >= text:
		return after, nil
	}
	if err := Split(at, block.Kind(), block.Attrs())(tx); err != nil {
		return doc.Pos{}, err
	}
	return after, nil
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
