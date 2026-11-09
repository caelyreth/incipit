// Package core is the core policy: inserting and deleting text, Enter and
// soft breaks per slot kind, grapheme, line and page motion, Place,
// VisualMove accepting the host's target, and paste through Fit. Its rules
// are the lowest precedence at every scope, and it answers every intent.
package core

import (
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Record kinds text and structure edits produce.
var (
	textEdits = []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks,
		doc.RecordTransferred, doc.RecordNodesInserted, doc.RecordNodesRemoved, doc.RecordNodeMoved, doc.RecordNodeChanged}
	markEdits = []doc.RecordKind{doc.RecordMarks}
)

// Policy is the core policy.
var Policy = policy.Define(policy.CoreID, func(p *policy.P) {
	p.Annotation(vocab.PendingMarks, nil)

	p.Handle("type").On(vocab.On(vocab.IntentType)).Produces(textEdits...).Do(typeText)
	p.Handle("delete").On(vocab.On(vocab.IntentDelete)).Produces(textEdits...).Do(deleteText)
	p.Handle("break").On(vocab.On(vocab.IntentBreak)).Produces(textEdits...).Do(breakBlock)
	p.Handle("soft-break").On(vocab.On(vocab.IntentSoftBreak)).Produces(textEdits...).Do(softBreak)
	p.Handle("paste").On(vocab.On(vocab.IntentPaste)).Produces(textEdits...).Do(paste)
	p.Handle("cut").On(vocab.On(vocab.IntentCut)).Produces(textEdits...).Do(cut)
	p.Handle("compose").On(vocab.On(vocab.IntentCompose)).Produces(textEdits...).Do(compose)
	p.Handle("cancel-composition").On(vocab.On(vocab.IntentCancelComposition)).Do(
		func(*policy.Ctx, policy.Match) policy.Result { return policy.Result{EndComposition: true} })

	p.Handle("move").On(vocab.On(vocab.IntentMove)).Do(move)
	p.Handle("visual-move").On(vocab.On(vocab.IntentVisualMove)).Do(visualMove)
	p.Handle("place").On(vocab.On(vocab.IntentPlace)).Do(placeCaret)

	p.Handle("leave").On(vocab.On(vocab.IntentLeave)).Do(dismiss)
	p.Handle("return").On(vocab.On(vocab.IntentReturn)).Do(swallow)
	p.Handle("select-scope").On(vocab.On(vocab.IntentSelectScope)).Do(selectAll)
	p.Handle("dismiss").On(vocab.On(vocab.IntentDismiss)).Do(dismiss)

	p.Handle("format").On(vocab.On(vocab.IntentFormat)).Produces(markEdits...).Do(format)
	p.Handle("nest").On(vocab.On(vocab.IntentNest)).Do(swallow)
	p.Handle("reshape").On(vocab.On(vocab.IntentReshape)).Produces(vocab.RoleEdits...).Do(reshape)
	p.Handle("move-node").On(vocab.On(vocab.IntentMoveNode)).Produces(vocab.RoleEdits...).Do(moveNode)
	p.Handle("toggle-task").On(vocab.On(vocab.IntentToggleTask)).Produces(doc.RecordNodeChanged).Do(toggleTask)
	// TODO(tables), TODO(figures): the tables and figures policies answer
	// these inside their objects; core has nothing to do elsewhere.
	p.Handle("table").On(vocab.On(vocab.IntentTable)).Do(swallow)
	p.Handle("figure").On(vocab.On(vocab.IntentFigure)).Do(swallow)
	p.Handle("media").On(vocab.On(vocab.IntentMedia)).Do(swallow)
	// Timers end annotation lifetimes in the engine; nothing else to do.
	p.Handle("timer").On(vocab.On(vocab.IntentTimer)).Do(swallow)
	// The history policy answers undo and redo; without it they do nothing.
	p.Handle("undo").On(vocab.On(vocab.IntentUndo)).Do(swallow)
	p.Handle("redo").On(vocab.On(vocab.IntentRedo)).Do(swallow)
	// Commands resolve to intents in the engine before dispatch.
	p.Handle("command").On(vocab.On(vocab.IntentCommand)).Do(swallow)

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
	{"core.strong", "commands.strong", vocab.Format{Mark: vocab.Strong}},
	{"core.emphasis", "commands.emphasis", vocab.Format{Mark: vocab.Emphasis}},
	{"core.code", "commands.code", vocab.Format{Mark: vocab.CodeMark}},
	{"core.strike", "commands.strike", vocab.Format{Mark: vocab.Strike}},
	{"core.paragraph", "commands.paragraph", vocab.Reshape{Shape: vocab.Paragraph}},
	{"core.heading1", "commands.heading1", vocab.Reshape{Shape: vocab.Heading, Attrs: headingAttrs(1)}},
	{"core.heading2", "commands.heading2", vocab.Reshape{Shape: vocab.Heading, Attrs: headingAttrs(2)}},
	{"core.heading3", "commands.heading3", vocab.Reshape{Shape: vocab.Heading, Attrs: headingAttrs(3)}},
	{"core.quote", "commands.quote", vocab.Reshape{Shape: vocab.Quote}},
	{"core.codeBlock", "commands.codeBlock", vocab.Reshape{Shape: vocab.Code}},
}

func headingAttrs(level int) doc.Attrs {
	return doc.MakeAttrs(doc.Field{Num: vocab.FieldLevel, Value: doc.Int(int64(level))})
}

func swallow(*policy.Ctx, policy.Match) policy.Result { return policy.Swallow }

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
		return removeNode(sel.Node), at, true
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
		n, ok := r.Node(gap.Node)
		if ok && gap.Slot < n.NumSlots() && gap.Offset < n.Slot(gap.Slot).Len() {
			child := n.Slot(gap.Slot).Child(gap.Offset)
			if ref, ok := vocab.FirstTextSlot(child); ok {
				cn, _ := r.Node(ref.Node)
				return doc.Collapsed(doc.CaretAt(doc.At(ref, cn.Slot(ref.Slot).Len())))
			}
		}
		return doc.Collapsed(doc.CaretAt(gap))
	}
}

func removeNode(id doc.ID) vocab.Edit {
	return func(tx *doc.Tx) error { return tx.Apply(doc.Remove{IDs: []doc.ID{id}}) }
}

// insertAt is where typed content lands for a caret: a text position, or,
// at a gap between blocks, the start of a new paragraph made there.
func insertAt(r doc.Reader, at doc.Pos) (doc.Pos, vocab.Edit) {
	spec, ok := vocab.SlotSpec(r, at.Ref())
	if !ok || !spec.Blocks {
		return at, nil
	}
	para := vocab.NewParagraph()
	return at, func(tx *doc.Tx) error {
		return tx.Apply(doc.Insert{Parent: at.Node, Slot: at.Slot, Index: at.Offset, Nodes: []*doc.Node{para}})
	}
}

// typeText inserts text at the caret, replacing a selection. Text takes
// pending marks, or the marks of the text before it.
func typeText(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Type)
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
	var pending *doc.MarkSet
	for _, a := range c.Annotations().At(vocab.PendingMarks, pos) {
		if ms, ok := a.Data.(*doc.MarkSet); ok {
			pending = ms
		}
		res.Drop = append(res.Drop, a.ID)
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
			para := vocab.NewParagraph()
			if err := tx.Apply(doc.Insert{Parent: pos.Node, Slot: pos.Slot, Index: pos.Offset, Nodes: []*doc.Node{para}}); err != nil {
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
			marks = vocab.InheritedMarks(content, pos.Offset)
		}
		if spec.Literal {
			marks = nil
		}
		return tx.Apply(doc.EditText{At: pos.Ref(), From: pos.Offset, To: pos.Offset, Items: []doc.Item{doc.TextItem(text, marks)}})
	}
}

// deleteText deletes what the selection covers, or one unit from a
// collapsed caret. At a slot's edge it steps a block's marker down, joins
// blocks, or selects an object instead of deleting it in one press.
func deleteText(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Delete)
	if in.Dir == 0 {
		in.Dir = vocab.Backward
	}
	if clear, _, ok := clearSelection(c); ok {
		return policy.Do(clear)
	}
	r := c.Doc()
	head := c.Head()
	if sel := c.Selection(); sel.Kind == doc.SelectText && !sel.IsCollapsed() {
		// A selection of notation alone deletes as a caret at its end in
		// the direction of deleting would.
		from, to := sel.Ordered(r)
		head = to
		if in.Dir > 0 {
			head = from
		}
	}
	if bound := c.Bound(head); bound != head.Pos {
		// Before a block's marker: backward deletes the boundary before
		// the block, forward deletes into its marker.
		block, ok := vocab.Block(r, head.Pos)
		if !ok {
			return policy.Swallow
		}
		if in.Dir > 0 {
			if res, ok := stepDown(r, block); ok {
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
		if i >= 0 && i < n.Slot(head.Slot).Len() && vocab.IsObject(n.Slot(head.Slot).Child(i).Kind()) {
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
			if res, ok := stepDown(r, block); ok {
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
	if vocab.IsObject(next.Kind()) {
		return policy.Select(doc.NodeSelection(next.ID()))
	}
	if ref, ok := vocab.FirstTextSlot(next); ok && ref.Node == next.ID() && vocab.CanJoin(r, head.Ref(), ref) {
		return policy.Do(vocab.Join(head.Ref(), next.ID()))
	}
	return policy.Swallow
}

// stepDown takes one level of a block's marker away: a leveled block
// (a heading, an item) steps down one level, and another kind with a
// marker becomes a paragraph. False for a block without a marker.
func stepDown(r doc.Reader, block *doc.Node) (policy.Result, bool) {
	if role, ok := vocab.NodeRole(r, block, vocab.RoleLeveled); ok {
		return policy.Do(role.(vocab.Leveled).Step(vocab.Backward)), true
	}
	switch k := block.Kind(); {
	case k == vocab.Paragraph || k == vocab.Cell || !vocab.IsTextBlock(k) || vocab.IsObject(k):
		return policy.Result{}, false
	}
	return policy.Do(vocab.Unmark(block.ID())), true
}

// deleteBefore deletes the boundary before a block, its marker kept: the
// block joins the text before it, an object before it is selected, and
// the first block of a quote leaves it. The first item of a list deletes
// the boundary before its list.
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
			if vocab.IsObject(prev.Kind()) {
				return policy.Select(doc.NodeSelection(prev.ID()))
			}
			ref, ok := vocab.LastTextSlot(prev)
			first, fok := vocab.FirstTextSlot(block)
			if ok && fok && first.Node == block.ID() && vocab.CanJoin(r, ref, first) {
				return policy.Do(vocab.Join(ref, block.ID()))
			}
			return policy.Swallow
		}
		switch {
		case parent == r.Root().ID() || pn.Kind() == vocab.Item:
			return policy.Swallow
		case pn.Kind() == vocab.List:
			// The boundary before a list's first item is the list's.
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
func breakBlock(c *policy.Ctx, m policy.Match) policy.Result {
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
	kind, attrs := continuation(block)
	if clear == nil && pos.Offset == 0 && pos.Slot == 0 && block.Slot(0).Len() > 0 {
		// The block keeps its text and the caret; an empty one opens
		// before it.
		parent, slot, index, ok := vocab.Index(r, block.ID())
		if !ok {
			return policy.Swallow
		}
		res := policy.Do(vocab.Ops(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{vocab.NewBlock(kind, attrs)}}))
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

// continuation is the kind a block opened next to block takes: an item
// continues its list as an open task or a plain item, every other block
// as a paragraph.
func continuation(block *doc.Node) (doc.Kind, doc.Attrs) {
	if block.Kind() == vocab.Item {
		if vocab.TaskOf(block) != vocab.NoTask {
			return vocab.Item, doc.MakeAttrs(doc.Field{Num: vocab.FieldTask, Value: doc.Int(int64(vocab.TaskOpen))})
		}
		return vocab.Item, doc.Attrs{}
	}
	return vocab.Paragraph, doc.Attrs{}
}

// startOfNext puts the caret at the start of the block after id, as a
// split leaves it.
func startOfNext(id doc.ID) func(doc.Reader) doc.Selection {
	return func(r doc.Reader) doc.Selection {
		parent, slot, index, ok := vocab.Index(r, id)
		if ok {
			pn, _ := r.Node(parent)
			if index+1 < pn.Slot(slot).Len() {
				if ref, ok := vocab.FirstTextSlot(pn.Slot(slot).Child(index + 1)); ok {
					return doc.Collapsed(doc.CaretAt(doc.At(ref, 0)))
				}
			}
		}
		return doc.Collapsed(doc.CaretAt(doc.Pos{Node: r.Root().ID()}))
	}
}

// softBreak inserts a line break where the slot keeps lines, and is Enter
// elsewhere.
func softBreak(c *policy.Ctx, m policy.Match) policy.Result {
	clear, pos, cleared := clearSelection(c)
	if spec, ok := vocab.SlotSpec(c.Doc(), pos.Ref()); ok && spec.Lines && !spec.Blocks {
		var edits []vocab.Edit
		if cleared {
			edits = append(edits, clear)
		}
		return policy.Do(append(edits, insertText(pos, "\n", nil, true))...)
	}
	return breakBlock(c, m)
}

// paste inserts the clipboard's slice, or its text as paragraphs, fitted
// to the slot.
func paste(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Paste)
	slice := in.Slice
	if slice.Empty() {
		slice = TextSlice(in.Text)
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
			para := vocab.NewParagraph()
			if err := tx.Apply(doc.Insert{Parent: at.Node, Slot: at.Slot, Index: at.Offset, Nodes: []*doc.Node{para}}); err != nil {
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
func TextSlice(text string) vocab.Slice {
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
		if l == "" {
			nodes[i] = vocab.NewParagraph()
		} else {
			nodes[i] = vocab.NewParagraph(doc.TextItem(l, nil))
		}
	}
	return vocab.Slice{Nodes: nodes, OpenStart: 1, OpenEnd: 1}
}

// cut deletes the selection; the host copied it before sending Cut.
func cut(c *policy.Ctx, m policy.Match) policy.Result {
	if clear, _, ok := clearSelection(c); ok {
		return policy.Do(clear)
	}
	return policy.Swallow
}

// compose sets the preedit at the caret, replacing a selection first.
func compose(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Compose)
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
func selectAll(c *policy.Ctx, m policy.Match) policy.Result {
	// TODO(scopes): the scopes policy grows the selection from the
	// innermost scope outward; core selects the page.
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
// It is also Leave where no scope answers it: Escape outside every scope
// dismisses.
func dismiss(c *policy.Ctx, m policy.Match) policy.Result {
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

// format toggles a mark over the selection, or the pending marks at a
// collapsed caret.
func format(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Format)
	sel := c.Selection()
	if sel.Kind != doc.SelectText {
		return policy.Swallow
	}
	r := c.Doc()
	if sel.IsCollapsed() {
		head := c.Head().Pos
		in2, ok := c.Inline(head.Ref())
		if spec, sok := vocab.SlotSpec(r, head.Ref()); !ok || !sok || spec.Literal {
			return policy.Swallow
		}
		res := policy.Result{}
		marks := vocab.InheritedMarks(in2, head.Offset)
		for _, a := range c.Annotations().At(vocab.PendingMarks, head) {
			if ms, ok := a.Data.(*doc.MarkSet); ok {
				marks = ms
			}
			res.Drop = append(res.Drop, a.ID)
		}
		if marks == nil {
			marks = r.Marks().Empty()
		}
		if marks.Has(in.Mark) {
			marks = marks.Without(in.Mark)
		} else {
			marks = vocab.WithMark(marks, doc.Mark{Kind: in.Mark})
		}
		anchor := policy.PointAnchor(head)
		res.Annotate = []policy.Annotate{{Kind: vocab.PendingMarks, Anchor: anchor, Data: marks, Lifetime: policy.UntilCaretLeaves(nil)}}
		return res
	}
	from, to := c.Range()
	spans := vocab.TextSpans(r, from, to)
	if vocab.CoverageOf(r, spans, in.Mark) == vocab.CoverFull {
		return policy.Do(vocab.ClearMark(spans, in.Mark))
	}
	return policy.Do(vocab.ApplyMark(spans, doc.Mark{Kind: in.Mark}))
}

// selectedBlocks are the text blocks the selection touches.
func selectedBlocks(c *policy.Ctx) []*doc.Node {
	r := c.Doc()
	from, to := c.Range()
	var out []*doc.Node
	for _, s := range vocab.TextSpans(r, from, to) {
		b, ok := vocab.Block(r, doc.At(s.Slot, s.From))
		if ok && (len(out) == 0 || out[len(out)-1] != b) {
			out = append(out, b)
		}
	}
	return out
}

// reshape changes the selected text blocks' kind; Quote wraps them.
// TODO(lists): reshaping into a list belongs to the lists policy, which
// builds items; core leaves it.
func reshape(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Reshape)
	r := c.Doc()
	blocks := selectedBlocks(c)
	if len(blocks) == 0 {
		return policy.Swallow
	}
	switch in.Shape {
	case vocab.Quote:
		ids := make([]doc.ID, len(blocks))
		for i, b := range blocks {
			ids[i] = b.ID()
		}
		run, ok := vocab.WrapRun(c.Doc(), ids, vocab.Quote)
		if !ok {
			return policy.Swallow
		}
		return policy.Do(vocab.Wrap(run, vocab.Quote, in.Attrs))
	case vocab.Paragraph, vocab.Heading, vocab.Code:
	default:
		return policy.Swallow
	}
	var edits []vocab.Edit
	for _, b := range blocks {
		if b.Kind() == vocab.Item || !vocab.IsTextBlock(b.Kind()) {
			continue
		}
		kind, attrs := in.Shape, r.Schema().Canonical(in.Shape, in.Attrs)
		if b.Kind() == kind && b.Attrs().Equal(attrs) {
			kind, attrs = vocab.Paragraph, doc.Attrs{}
		}
		edits = append(edits, vocab.Retype(b.ID(), kind, attrs))
	}
	return policy.Do(edits...)
}

func moveNode(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.MoveNode)
	return policy.Do(vocab.MoveTo(in.ID, in.Target))
}

func toggleTask(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.ToggleTask)
	n, ok := c.Doc().Node(in.ID)
	if !ok || n.Kind() != vocab.Item {
		return policy.Swallow
	}
	next := vocab.TaskDone
	switch vocab.TaskOf(n) {
	case vocab.NoTask:
		return policy.Swallow
	case vocab.TaskDone:
		next = vocab.TaskOpen
	}
	return policy.Do(vocab.SetAttrs(in.ID, n.Attrs().With(vocab.FieldTask, doc.Int(int64(next)))))
}
