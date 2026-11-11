// Package core is the core policy: inserting and deleting text, Enter and
// soft breaks per slot kind, grapheme, line and page motion, Place,
// VisualMove accepting the host's target, and paste through Fit; and the
// product's text features: formats, block kinds and quotes. Its rules are
// the lowest precedence at every scope, and it answers every primitive
// intent.
package core

import (
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Policy is the core policy.
var Policy = policy.Define(policy.CoreID, func(p *policy.P) {
	// Core answers every primitive intent at every selection, inside any
	// slot.
	in := policy.Slot.Inside()
	policy.On(p, "type", policy.Intent[vocab.Type](), in, typeText).Any()
	policy.On(p, "delete-selection", policy.Intent[vocab.Delete](), in, deleteSelection).Range().Node()
	policy.On(p, "delete", policy.Intent[vocab.Delete](), in, deleteText).Any()
	policy.On(p, "break", policy.Intent[vocab.Break](), in, breakBlock).Any()
	policy.On(p, "soft-break", policy.Intent[vocab.SoftBreak](), in, softBreak).Any()
	policy.On(p, "paste", policy.Intent[vocab.Paste](), in, paste).Any()
	policy.On(p, "cut", policy.Intent[vocab.Cut](), in, cut).Any()
	policy.On(p, "compose", policy.Intent[vocab.Compose](), in, compose).Any()
	policy.On(p, "cancel-composition", policy.Intent[vocab.CancelComposition](), in,
		func(*policy.Ctx, vocab.CancelComposition, policy.At[doc.SlotRef]) policy.Result {
			return policy.Result{EndComposition: true}
		}).Any()

	// What a range or a selected object does first is core's, for every
	// unit; unit policies answer the caret.
	policy.On(p, "collapse-range", policy.Moving(0, 0), in, collapseRange).Range()
	policy.On(p, "leave-object", policy.Moving(0, 0), in, leaveObject).Node()
	policy.On(p, "move", policy.Intent[vocab.Move](), in, move).Any()
	policy.On(p, "visual-move", policy.Intent[vocab.VisualMove](), in, visualMove).Any()
	policy.On(p, "place", policy.Intent[vocab.Place](), in, placeCaret).Any()

	// Leave and Return move between scopes, which the navigation policy
	// answers; with no scope to leave or go back to they do nothing, so a
	// key bound to them falls through to its next command.
	policy.On(p, "leave", policy.Intent[vocab.Leave](), in, swallow).Any()
	policy.On(p, "return", policy.Intent[vocab.Return](), in, swallow).Any()
	policy.On(p, "select-scope", policy.Intent[vocab.SelectScope](), in, selectAll).Any()
	policy.On(p, "dismiss", policy.Intent[vocab.Dismiss](), in, dismiss).Any()

	policy.On(p, "move-node", policy.Intent[vocab.MoveNode](), in, moveNode).Any()

	// The product's text features: a format toggles the marks typing takes
	// at a caret and the marks of a range; blocks the selection touches
	// take another kind or a quote.
	policy.On(p, "format-at-caret", policy.Intent[profile.Format](), policy.Slot.Own(), formatAtCaret)
	policy.On(p, "format", policy.Intent[profile.Format](), in, format).Range()
	policy.On(p, "retype", policy.Intent[profile.Retype](), in, retype).Caret().Range()
	policy.On(p, "wrap-quote", policy.Intent[profile.WrapQuote](), in, wrapQuote).Caret().Range()
	// Timers end annotation lifetimes in the engine; nothing else to do.
	policy.On(p, "timer", policy.Intent[vocab.Timer](), in, swallow).Any()
	// The history policy answers undo and redo; without it they do nothing.
	policy.On(p, "undo", policy.Intent[vocab.Undo](), in, swallow).Any()
	policy.On(p, "redo", policy.Intent[vocab.Redo](), in, swallow).Any()
	// Commands resolve to intents in the engine before dispatch.
	policy.On(p, "command", policy.Intent[vocab.Command](), in, swallow).Any()

	for _, c := range commands {
		p.Command(c.id, c.label, c.intent)
	}
})

type command struct {
	id     vocab.CommandID
	label  string
	intent vocab.Intent
}

// The core commands: keymaps store these ids.
var commands = []command{
	{"core.undo", "commands.undo", vocab.Undo{}},
	{"core.redo", "commands.redo", vocab.Redo{}},
	{"core.selectAll", "commands.selectAll", vocab.SelectScope{}},
	{"core.strong", "commands.strong", profile.Format{Mark: profile.Strong}},
	{"core.emphasis", "commands.emphasis", profile.Format{Mark: profile.Emphasis}},
	{"core.code", "commands.code", profile.Format{Mark: profile.CodeMark}},
	{"core.strike", "commands.strike", profile.Format{Mark: profile.Strike}},
	{"core.paragraph", "commands.paragraph", profile.Retype{To: profile.Paragraph}},
	{"core.heading1", "commands.heading1", profile.Retype{To: profile.Heading, Attrs: headingAttrs(1)}},
	{"core.heading2", "commands.heading2", profile.Retype{To: profile.Heading, Attrs: headingAttrs(2)}},
	{"core.heading3", "commands.heading3", profile.Retype{To: profile.Heading, Attrs: headingAttrs(3)}},
	{"core.quote", "commands.quote", profile.WrapQuote{}},
	{"core.codeBlock", "commands.codeBlock", profile.Retype{To: profile.Code}},
	{"core.toggleTask", "commands.toggleTask", profile.ToggleTask{}},

	{"core.left", "commands.left", vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}},
	{"core.right", "commands.right", vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward}},
	{"core.wordLeft", "commands.wordLeft", vocab.Move{Unit: vocab.ByWord, Dir: vocab.Backward}},
	{"core.wordRight", "commands.wordRight", vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward}},
	{"core.lineStart", "commands.lineStart", vocab.VisualMove{Dir: vocab.LineStart}},
	{"core.lineEnd", "commands.lineEnd", vocab.VisualMove{Dir: vocab.LineEnd}},
	{"core.up", "commands.up", vocab.VisualMove{Dir: vocab.Up}},
	{"core.down", "commands.down", vocab.VisualMove{Dir: vocab.Down}},
	{"core.paragraphUp", "commands.paragraphUp", vocab.Move{Unit: vocab.ByParagraph, Dir: vocab.Backward}},
	{"core.paragraphDown", "commands.paragraphDown", vocab.Move{Unit: vocab.ByParagraph, Dir: vocab.Forward}},
	{"core.pageStart", "commands.pageStart", vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}},
	{"core.pageEnd", "commands.pageEnd", vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}},
	{"core.pageUp", "commands.pageUp", vocab.VisualMove{Dir: vocab.PageUp}},
	{"core.pageDown", "commands.pageDown", vocab.VisualMove{Dir: vocab.PageDown}},
	{"core.deleteBackward", "commands.deleteBackward", vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}},
	{"core.deleteForward", "commands.deleteForward", vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward}},
	{"core.deleteWordBackward", "commands.deleteWordBackward", vocab.Delete{Unit: vocab.ByWord, Dir: vocab.Backward}},
	{"core.deleteWordForward", "commands.deleteWordForward", vocab.Delete{Unit: vocab.ByWord, Dir: vocab.Forward}},
	{"core.deleteToStart", "commands.deleteToStart", vocab.Delete{Unit: vocab.ToSlotEdge, Dir: vocab.Backward}},
	{"core.deleteToEnd", "commands.deleteToEnd", vocab.Delete{Unit: vocab.ToSlotEdge, Dir: vocab.Forward}},
	{"core.break", "commands.break", vocab.Break{}},
	{"core.softBreak", "commands.softBreak", vocab.SoftBreak{}},
	{"core.nest", "commands.nest", profile.Nest{Dir: vocab.Forward}},
	{"core.outdent", "commands.outdent", profile.Nest{Dir: vocab.Backward}},
	{"core.leave", "commands.leave", vocab.Leave{}},
	{"core.return", "commands.return", vocab.Return{}},
	{"core.dismiss", "commands.dismiss", vocab.Dismiss{}},
}

func headingAttrs(level int) doc.Attrs {
	return doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(int64(level))})
}

func swallow[I vocab.Intent](*policy.Ctx, I, policy.At[doc.SlotRef]) policy.Result {
	return policy.Swallow
}

// --- text ---------------------------------------------------------------------

// clearSelection returns the edit deleting what the selection covers
// (Ctx.Range), and where content replacing it goes: a position before the
// edit, which edits after it map. A collapsed caret clears nothing and
// inserts at its own position: its stop chooses marks, not a place.
func clearSelection(c *policy.Ctx) (vocab.Edit, doc.Pos, bool) {
	sel := c.Selection()
	switch sel.Kind {
	case doc.SelectNode:
		at, _ := c.Range()
		return vocab.RemoveNode(sel.Node), at, true
	case doc.SelectText:
		if sel.IsCollapsed() {
			return nil, c.Head().Pos, false
		}
		from, to := c.Range()
		if from == to {
			// Only notation is selected; the document holds nothing there.
			return nil, from, false
		}
		at := from
		if !vocab.IsText(c.Doc(), from) && vocab.IsText(c.Doc(), to) {
			// From a gap, the block the range ends in keeps the place.
			at = to
		}
		return vocab.DeleteRange(from, to), at, true
	}
	return nil, c.Head().Pos, false
}

// endOfNewBlock puts the caret at the end of the block created at a gap.
func endOfNewBlock(gap doc.Pos) func(doc.Reader) doc.Selection {
	return func(r doc.Reader) doc.Selection {
		if c, ok := vocab.BlockCaret(r, gap, vocab.Forward); ok {
			return doc.Collapsed(c)
		}
		return doc.Collapsed(doc.CaretAt(gap))
	}
}

// plain is an empty plain block.
func plain(s *doc.Schema) *doc.Node { return vocab.NewBlock(s, s.Plain(), doc.Attrs{}) }

// insertAt is where typed content lands for a caret: a text position, or,
// at a gap between blocks, the start of a new paragraph made there.
func insertAt(r doc.Reader, at doc.Pos) (doc.Pos, vocab.Edit) {
	spec, ok := vocab.SlotSpec(r, at.Ref())
	if !ok || !spec.Blocks {
		return at, nil
	}
	return at, func(tx *doc.Tx) error {
		return tx.Apply(doc.Insert{Parent: at.Node, Slot: at.Slot, Index: at.Offset, Nodes: []*doc.Node{plain(tx.Schema())}})
	}
}

// typeText inserts text at the caret, replacing a selection. Text takes
// pending marks, or the marks of the text before it.
func typeText(c *policy.Ctx, in vocab.Type, _ policy.At[doc.SlotRef]) policy.Result {
	res := policy.Result{}
	if c.Composition() != nil {
		res.EndComposition = true
	}
	// Typed text replaces the selection; empty text deletes it.
	text := in.Text
	var edits []vocab.Edit
	clear, pos, ok := clearSelection(c)
	if ok {
		edits = append(edits, clear)
	}
	if text == "" {
		res.Edits = edits
		return res
	}
	// Pending marks belong to the caret typed at; text takes them once.
	var pending *doc.MarkSet
	if p := c.Pending(); p != nil {
		pending, res.DropPending = p.Marks, true
	}
	edits = append(edits, insertText(pos, text, pending, pending != nil))
	res.Edits = edits
	if spec, ok := vocab.SlotSpec(c.Doc(), pos.Ref()); ok && spec.Blocks {
		res.Then = endOfNewBlock(pos)
	}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// insertText inserts text at a position that may have moved under earlier
// edits of the same transaction: it is resolved against the transaction
// by mapping the caret, so it runs after a selection was deleted.
func insertText(at doc.Pos, text string, marks *doc.MarkSet, explicit bool) vocab.Edit {
	return func(tx *doc.Tx) error {
		pos, _ := doc.Steps(tx.Steps()).Map(at, doc.BiasBefore)
		if spec, ok := vocab.SlotSpec(tx, pos.Ref()); ok && spec.Blocks {
			if err := tx.Apply(doc.Insert{Parent: pos.Node, Slot: pos.Slot, Index: pos.Offset, Nodes: []*doc.Node{plain(tx.Schema())}}); err != nil {
				return err
			}
			n, _ := tx.Node(pos.Node)
			pos = doc.Pos{Node: n.Slot(pos.Slot).Child(pos.Offset).ID()}
		}
		spec, ok := vocab.SlotSpec(tx, pos.Ref())
		if !ok {
			return doc.ErrSlot
		}
		if !spec.Lines {
			text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", " "), "\n", " ")
		}
		n, _ := tx.Node(pos.Node)
		content := n.Slot(pos.Slot).Content()
		if !explicit && !spec.Literal {
			marks = vocab.InheritedMarks(tx.Schema(), content, pos.Offset)
		}
		if spec.Literal {
			marks = nil
		}
		return tx.Apply(doc.EditText{At: pos.Ref(), From: pos.Offset, To: pos.Offset, Items: []doc.Item{doc.TextItem(text, marks)}})
	}
}

// deleteSelection deletes what a range or a selected node covers. A range
// of notation alone deletes as a caret at its end in the direction of
// deleting would.
func deleteSelection(c *policy.Ctx, in vocab.Delete, _ policy.At[doc.SlotRef]) policy.Result {
	if clear, _, ok := clearSelection(c); ok {
		return policy.Do(clear)
	}
	from, to := c.Selection().Ordered(c.Doc())
	if in.Dir > 0 {
		return deleteAt(c, in, from)
	}
	return deleteAt(c, in, to)
}

// deleteText deletes one unit from the caret.
func deleteText(c *policy.Ctx, in vocab.Delete, _ policy.At[doc.SlotRef]) policy.Result {
	return deleteAt(c, in, c.Head())
}

// deleteAt deletes one unit from head. At a slot's edge it steps a
// block's marker down, joins blocks, or selects an object instead of
// deleting it in one press.
func deleteAt(c *policy.Ctx, in vocab.Delete, head doc.Caret) policy.Result {
	if in.Dir == 0 {
		in.Dir = vocab.Backward
	}
	r := c.Doc()
	if bound := c.Bound(head); bound != head.Pos {
		// Before a block's marker: backward deletes the boundary before
		// the block, forward deletes into its marker.
		block, ok := vocab.Block(r, head.Pos)
		if !ok {
			return policy.Swallow
		}
		if in.Dir > 0 {
			if res, ok := stepDown(c, block); ok {
				return res
			}
			return policy.Swallow
		}
		return deleteBefore(c, block)
	}
	pos := head.Pos
	unit := in.Unit
	if unit == vocab.ToPageEdge {
		// Deleting runs to the slot's edge at most.
		unit = vocab.ToSlotEdge
	}
	target, ok := slotTarget(r, pos, unit, in.Dir)
	if ok && target != pos && unit == vocab.ByGrapheme {
		// A projected glyph between the caret and the content is not
		// content: the grapheme deleted is the glyph, which only its
		// owner edits, so core crosses it.
		if stops := c.Stops(); stops != nil {
			if next, ok := stops.Step(head, in.Dir); ok && next.Pos == head.Pos {
				return policy.Select(doc.Collapsed(next))
			}
		}
	}
	if ok && target != pos {
		from, to := pos, target
		if from.Offset > to.Offset {
			from, to = to, from
		}
		return policy.Do(vocab.Ops(doc.EditText{At: pos.Ref(), From: from.Offset, To: to.Offset}))
	}
	return deleteAcross(c, pos, in.Dir)
}

// deleteAcross handles a delete at a slot's edge.
func deleteAcross(c *policy.Ctx, head doc.Pos, dir vocab.Direction) policy.Result {
	r := c.Doc()
	spec, ok := vocab.SlotSpec(r, head.Ref())
	if ok && spec.Blocks {
		// At a gap: the object on that side is selected.
		n, _ := r.Node(head.Node)
		i := head.Offset
		if dir == vocab.Backward {
			i--
		}
		if i >= 0 && i < n.Slot(head.Slot).Len() && r.Schema().IsObject(n.Slot(head.Slot).Child(i).Kind()) {
			return policy.Select(doc.NodeSelection(n.Slot(head.Slot).Child(i).ID()))
		}
		return policy.Swallow
	}
	if _, _, inlineOwner := inlineParent(r, head.Node); inlineOwner {
		// An inline node's source: its edges are not joined.
		return policy.Swallow
	}
	block, ok := vocab.Block(r, head)
	if !ok {
		return policy.Swallow
	}
	if dir == vocab.Backward {
		if head.Slot == 0 {
			// At its text's start, a block with a marker steps it down.
			if res, ok := stepDown(c, block); ok {
				return res
			}
		}
		return deleteBefore(c, block)
	}
	parent, ps, index, ok := vocab.Index(r, block.ID())
	if !ok {
		return policy.Swallow
	}
	pn, _ := r.Node(parent)
	siblings := pn.Slot(ps)
	if index+1 >= siblings.Len() {
		return policy.Swallow
	}
	next := siblings.Child(index + 1)
	if r.Schema().IsObject(next.Kind()) {
		return policy.Select(doc.NodeSelection(next.ID()))
	}
	if ref, ok := vocab.FirstTextSlot(next); ok && ref.Node == next.ID() && vocab.CanJoin(r, head.Ref(), ref) {
		return policy.Do(vocab.Join(head.Ref(), next.ID()))
	}
	return policy.Swallow
}

// stepDown takes one level of a block's marker away through the block's
// own level: a heading or an item steps down, a code block becomes a
// paragraph. False for a block without a level.
func stepDown(c *policy.Ctx, block *doc.Node) (policy.Result, bool) {
	if role, ok := c.Role(block, vocab.RoleLeveled); ok {
		return policy.Do(role.(vocab.Leveled).Step(vocab.Backward)), true
	}
	return policy.Result{}, false
}

// deleteBefore deletes the boundary before a block, its marker kept: the
// block joins the text before it, an object before it is selected, and
// the first block of a container leaves it. A container's first entry
// deletes the boundary before its container.
func deleteBefore(c *policy.Ctx, block *doc.Node) policy.Result {
	r := c.Doc()
	id := block.ID()
	for {
		parent, ps, index, ok := vocab.Index(r, id)
		if !ok {
			return policy.Swallow
		}
		pn, _ := r.Node(parent)
		if index > 0 {
			prev := pn.Slot(ps).Child(index - 1)
			if r.Schema().IsObject(prev.Kind()) {
				return policy.Select(doc.NodeSelection(prev.ID()))
			}
			ref, ok := vocab.LastTextSlot(prev)
			first, fok := vocab.FirstTextSlot(block)
			if ok && fok && first.Node == block.ID() && vocab.CanJoin(r, ref, first) {
				return policy.Do(vocab.Join(ref, block.ID()))
			}
			return policy.Swallow
		}
		n, _ := r.Node(id)
		switch s := r.Schema(); {
		case parent == r.Root().ID() || s.IsEntry(pn.Kind()):
			// The page's first block, or the first of an entry's body:
			// nothing before it to join.
			return policy.Swallow
		case s.IsEntry(n.Kind()):
			// The boundary before a container's first entry is the
			// container's.
			id = parent
		default:
			return policy.Do(vocab.Lift(id))
		}
	}
}

// inlineParent reports the slot holding an inline node.
func inlineParent(r doc.Reader, id doc.ID) (doc.ID, int, bool) {
	parent, slot, ok := r.Parent(id)
	if !ok {
		return 0, 0, false
	}
	pn, _ := r.Node(parent)
	return parent, slot, pn.Slot(slot).IsInline()
}

// breakBlock is Enter: a line break in slots that keep lines, a split
// elsewhere. At a block's text start, and before its marker, a block opens
// before it and the caret stays; a heading split at its end continues as
// a paragraph.
func breakBlock(c *policy.Ctx, _ vocab.Break, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	var edits []vocab.Edit
	clear, pos, ok := clearSelection(c)
	if ok {
		edits = append(edits, clear)
	}
	spec, ok := vocab.SlotSpec(r, pos.Ref())
	if !ok {
		return policy.Swallow
	}
	if spec.Blocks {
		_, edit := insertAt(r, pos)
		res := policy.Do(append(edits, edit)...)
		res.Then = endOfNewBlock(pos)
		return res
	}
	if spec.Lines {
		return policy.Do(append(edits, insertText(pos, "\n", nil, true))...)
	}
	if _, _, inl := inlineParent(r, pos.Node); inl {
		return policy.Swallow
	}
	block, ok := vocab.Block(r, pos)
	if !ok {
		return policy.Swallow
	}
	kind, attrs := continuation(c, block)
	if clear == nil && pos.Offset == 0 && pos.Slot == 0 && block.Slot(0).Len() > 0 {
		// The block keeps its text and the caret; an empty one opens
		// before it.
		parent, slot, index, ok := vocab.Index(r, block.ID())
		if !ok {
			return policy.Swallow
		}
		res := policy.Do(vocab.Ops(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{vocab.NewBlock(r.Schema(), kind, attrs)}}))
		res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
		return res
	}
	if pos.Offset < block.Slot(pos.Slot).Len() {
		kind, attrs = block.Kind(), block.Attrs()
	}
	res := policy.Do(append(edits, func(tx *doc.Tx) error {
		at, _ := doc.Steps(tx.Steps()).Map(pos, doc.BiasBefore)
		return vocab.Split(at, kind, attrs)(tx)
	})...)
	res.Then = startOfNext(block.ID())
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// continuation is the kind a block opened next to block takes: what it
// continues as, or a plain block.
func continuation(c *policy.Ctx, block *doc.Node) (doc.Kind, doc.Attrs) {
	if role, ok := c.Role(block, vocab.RoleContinued); ok {
		return role.(vocab.Continued).Next()
	}
	return c.Doc().Schema().Plain(), doc.Attrs{}
}

// startOfNext puts the caret at the start of the block after id, as a
// split leaves it.
func startOfNext(id doc.ID) func(doc.Reader) doc.Selection {
	return func(r doc.Reader) doc.Selection {
		if parent, slot, index, ok := vocab.Index(r, id); ok {
			if c, ok := vocab.BlockCaret(r, doc.Pos{Node: parent, Slot: slot, Offset: index + 1}, vocab.Backward); ok {
				return doc.Collapsed(c)
			}
		}
		return doc.Collapsed(doc.CaretAt(doc.Pos{Node: r.Root().ID()}))
	}
}

// softBreak inserts a line break where the slot keeps lines, and is Enter
// elsewhere.
func softBreak(c *policy.Ctx, _ vocab.SoftBreak, at policy.At[doc.SlotRef]) policy.Result {
	clear, pos, cleared := clearSelection(c)
	if spec, ok := vocab.SlotSpec(c.Doc(), pos.Ref()); ok && spec.Lines && !spec.Blocks {
		var edits []vocab.Edit
		if cleared {
			edits = append(edits, clear)
		}
		return policy.Do(append(edits, insertText(pos, "\n", nil, true))...)
	}
	return breakBlock(c, vocab.Break{}, at)
}

// paste inserts the clipboard's slice, or its text as paragraphs, fitted
// to the slot.
func paste(c *policy.Ctx, in vocab.Paste, _ policy.At[doc.SlotRef]) policy.Result {
	slice := in.Slice
	if slice.Empty() {
		slice = TextSlice(c.Doc().Schema(), in.Text)
	}
	if slice.Empty() {
		return policy.Swallow
	}
	var edits []vocab.Edit
	clear, pos, ok := clearSelection(c)
	if ok {
		edits = append(edits, clear)
	}
	edits = append(edits, func(tx *doc.Tx) error {
		at, _ := doc.Steps(tx.Steps()).Map(pos, doc.BiasBefore)
		if spec, ok := vocab.SlotSpec(tx, at.Ref()); ok && spec.Blocks {
			if err := tx.Apply(doc.Insert{Parent: at.Node, Slot: at.Slot, Index: at.Offset, Nodes: []*doc.Node{plain(tx.Schema())}}); err != nil {
				return err
			}
			n, _ := tx.Node(at.Node)
			at = doc.Pos{Node: n.Slot(at.Slot).Child(at.Offset).ID()}
		}
		return vocab.InsertSlice(at, slice)(tx)
	})
	return policy.Result{Edits: edits, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
}

// TextSlice is plain text as a slice: one line inline, several lines as
// paragraphs with open edges.
func TextSlice(s *doc.Schema, text string) vocab.Slice {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text == "" {
		return vocab.Slice{}
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		return vocab.Slice{Inline: []doc.Item{doc.TextItem(text, nil)}}
	}
	nodes := make([]*doc.Node, len(lines))
	for i, l := range lines {
		nodes[i] = plain(s)
		if l != "" {
			nodes[i] = vocab.NewBlock(s, s.Plain(), doc.Attrs{}, doc.TextItem(l, nil))
		}
	}
	return vocab.Slice{Nodes: nodes, OpenStart: 1, OpenEnd: 1}
}

// cut deletes the selection; the host copied it before sending Cut.
func cut(c *policy.Ctx, _ vocab.Cut, _ policy.At[doc.SlotRef]) policy.Result {
	if clear, _, ok := clearSelection(c); ok {
		return policy.Do(clear)
	}
	return policy.Swallow
}

// compose sets the preedit at the caret, replacing a selection first.
func compose(c *policy.Ctx, in vocab.Compose, _ policy.At[doc.SlotRef]) policy.Result {
	if in.Preedit == "" {
		return policy.Result{EndComposition: true}
	}
	res := policy.Result{}
	at := c.Head().Pos
	if cur := c.Composition(); cur != nil {
		at = cur.At
	} else if clear, from, ok := clearSelection(c); ok {
		res.Edits = []vocab.Edit{clear}
		at = from
		// The composition point is in pre-edit positions; deleting the
		// selection collapses onto its start, which maps to itself.
	}
	res.Composition = &policy.Composition{At: at, Text: in.Preedit, Selected: in.Selected}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// --- scope and structure --------------------------------------------------------------

// selectAll selects the page's text, from its first stop to its last.
func selectAll(c *policy.Ctx, _ vocab.SelectScope, _ policy.At[doc.SlotRef]) policy.Result {
	// The scopes policy grows the selection from the innermost scope
	// outward; core's fallback is the page when that policy is not enabled.
	stops := c.Stops()
	if stops == nil {
		return policy.Swallow
	}
	first, ok1 := stops.Edge(vocab.Backward)
	last, ok2 := stops.Edge(vocab.Forward)
	if !ok1 || !ok2 {
		return policy.Swallow
	}
	return policy.Select(doc.TextRange(first, last))
}

// dismiss collapses the selection to its head and ends a composition.
// Escape outside every scope dismisses; Leave answers scopes separately.
func dismiss(c *policy.Ctx, _ vocab.Dismiss, _ policy.At[doc.SlotRef]) policy.Result {
	res := policy.Result{}
	if c.Composition() != nil {
		res.EndComposition = true
	}
	sel := c.Selection()
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		s := doc.Collapsed(c.Head())
		res.Selection = &s
	}
	return res
}

// formatAtCaret toggles a mark in the marks text typed at the caret
// takes: the pending ones, or the marks around it.
func formatAtCaret(c *policy.Ctx, in profile.Format, at policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	head := c.Head().Pos
	if spec, ok := vocab.SlotSpec(r, at.Data); !ok || spec.Literal {
		return policy.Swallow
	}
	in2, _ := c.Inline(at.Data)
	marks := vocab.InheritedMarks(r.Schema(), in2, head.Offset)
	if p := c.Pending(); p != nil {
		marks = p.Marks
	}
	if marks == nil {
		marks = r.Marks().Empty()
	}
	if marks.Has(in.Mark) {
		marks = marks.Without(in.Mark)
	} else {
		marks = vocab.WithMark(r.Schema(), marks, doc.Mark{Kind: in.Mark})
	}
	return policy.Result{Pending: marks}
}

// format toggles a mark over a range: cleared where it covers all of it,
// applied otherwise.
func format(c *policy.Ctx, in profile.Format, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	from, to := c.Range()
	spans := vocab.TextSpans(r, from, to)
	if vocab.CoverageOf(r, spans, in.Mark) == vocab.CoverFull {
		return policy.Do(vocab.ClearMark(spans, in.Mark))
	}
	return policy.Do(vocab.ApplyMark(spans, doc.Mark{Kind: in.Mark}))
}

// retype gives the text blocks the selection touches a kind where their
// slot takes it; blocks that already have it, with its attributes, become
// plain again.
func retype(c *policy.Ctx, in profile.Retype, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	from, to := c.Range()
	attrs := r.Schema().Canonical(in.To, in.Attrs)
	var edits []vocab.Edit
	for _, b := range vocab.SelectedBlocks(r, from, to) {
		if !r.Schema().IsTextBlock(b.Kind()) || !vocab.Accepts(r, b.ID(), in.To) {
			continue
		}
		kind, as := in.To, attrs
		if b.Kind() == kind && b.Attrs().Equal(as) {
			kind, as = r.Schema().Plain(), doc.Attrs{}
		}
		edits = append(edits, vocab.Retype(b.ID(), kind, as))
	}
	if len(edits) == 0 {
		return policy.Swallow
	}
	return policy.Do(edits...)
}

// wrapQuote wraps the blocks the selection touches in a quote, lifted to
// what the container holds.
func wrapQuote(c *policy.Ctx, _ profile.WrapQuote, _ policy.At[doc.SlotRef]) policy.Result {
	from, to := c.Range()
	blocks := vocab.SelectedBlocks(c.Doc(), from, to)
	ids := make([]doc.ID, len(blocks))
	for i, b := range blocks {
		ids[i] = b.ID()
	}
	run, ok := vocab.WrapRun(c.Doc(), ids, profile.Quote)
	if len(ids) == 0 || !ok {
		return policy.Swallow
	}
	return policy.Do(vocab.Wrap(run, profile.Quote, doc.Attrs{}))
}

func moveNode(c *policy.Ctx, in vocab.MoveNode, _ policy.At[doc.SlotRef]) policy.Result {
	return policy.Do(vocab.MoveTo(in.ID, in.Target))
}
