// Package core is the core policy: inserting and deleting text, Enter and
// soft breaks per slot kind, grapheme, line and page motion, Place,
// VisualMove accepting the host's target, and paste through Fit. Its rules
// are the lowest precedence at every scope, and it answers every primitive
// intent. It names no kind: blocks step and continue through their roles,
// and schema traits say the rest.
package core

import (
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// Policy is the core policy.
var Policy = policy.Define(policy.CoreID, func(p *policy.P) {
	// Core answers every primitive intent at every selection, inside any
	// slot.
	in := policy.Slot.Inside()
	policy.On(p, "type", policy.Intent[vocab.Type](), in, typeText).Any()
	policy.On(p, "delete-selection", policy.Intent[vocab.Delete](), in, deleteSelection).Range().Node().Cells()
	policy.On(p, "delete", policy.Intent[vocab.Delete](), in, deleteText).Any()
	// What a delete crosses toward the next stop, a nearer crossing first:
	// a block's level, the boundary before it, a container it leaves.
	policy.On(p, "delete-across", policy.Intent[vocab.Delete](),
		policy.Or(policy.Across(vocab.Backward), policy.Across(vocab.Forward)), deleteAcross)
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
	policy.On(p, "select-line", policy.Intent[vocab.SelectLine](), in, selectLine).Any()
	policy.On(p, "place", policy.Intent[vocab.Place](), in, placeCaret).Any()

	// Leave and Return move between scopes, which the navigation policy
	// answers; with no scope to leave or go back to they do nothing, so a
	// key bound to them falls through to its next command.
	policy.On(p, "leave", policy.Intent[vocab.Leave](), in, swallow).Any()
	policy.On(p, "return", policy.Intent[vocab.Return](), in, swallow).Any()
	policy.On(p, "select-scope", policy.Intent[vocab.SelectScope](), in, selectAll).Any()
	policy.On(p, "dismiss", policy.Intent[vocab.Dismiss](), in, dismiss).Any()

	policy.On(p, "move-node", policy.Intent[vocab.MoveNode](), in, moveNode).Any()

	// Timers end annotation lifetimes in the engine; nothing else to do.
	policy.On(p, "timer", policy.Intent[vocab.Timer](), in, swallow).Any()
	// The history policy answers undo and redo; without it they do nothing.
	policy.On(p, "undo", policy.Intent[vocab.Undo](), in, swallow).Any()
	policy.On(p, "redo", policy.Intent[vocab.Redo](), in, swallow).Any()
	// Commands resolve to intents in the engine before dispatch.
	policy.On(p, "command", policy.Intent[vocab.Command](), in, swallow).Any()

	for _, c := range commands {
		p.Command(c.id, c.label, c.intent, c.key)
	}

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("Every keystroke is an edit"),
		policy.Waits(500*time.Millisecond),
		policy.Presses(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true}), policy.Presses(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true}), policy.Presses(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true}), policy.Presses(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true}),
		policy.Waits(400*time.Millisecond),
		policy.Types("intent."),
	)
}, policy.Label("policies.core"), policy.Description("policies.core.description"))

type command struct {
	id     vocab.CommandID
	label  string
	intent vocab.Intent
	key    policy.CommandSetting
}

// The core commands: keymaps store these ids.
var commands = []command{
	{"core.undo", "commands.undo", vocab.Undo{}, policy.SystemKey("primary+z")},
	{"core.redo", "commands.redo", vocab.Redo{}, policy.SystemKey("shift+primary+z")},
	{"core.selectAll", "commands.selectAll", vocab.SelectScope{}, policy.SystemKey("primary+a")},

	{"core.left", "commands.left", vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}, policy.Key("left")},
	{"core.right", "commands.right", vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward}, policy.Key("right")},
	{"core.wordLeft", "commands.wordLeft", vocab.Move{Unit: vocab.ByWord, Dir: vocab.Backward}, policy.Key("alt+left")},
	{"core.wordRight", "commands.wordRight", vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward}, policy.Key("alt+right")},
	{"core.lineStart", "commands.lineStart", vocab.VisualMove{Dir: vocab.LineStart}, policy.Key("primary+left")},
	{"core.lineEnd", "commands.lineEnd", vocab.VisualMove{Dir: vocab.LineEnd}, policy.Key("primary+right")},
	{"core.selectLine", "commands.selectLine", vocab.SelectLine{}, policy.Key("primary+l")},
	{"core.up", "commands.up", vocab.VisualMove{Dir: vocab.Up}, policy.Key("up")},
	{"core.down", "commands.down", vocab.VisualMove{Dir: vocab.Down}, policy.Key("down")},
	{"core.paragraphUp", "commands.paragraphUp", vocab.Move{Unit: vocab.ByParagraph, Dir: vocab.Backward}, policy.Key("alt+up")},
	{"core.paragraphDown", "commands.paragraphDown", vocab.Move{Unit: vocab.ByParagraph, Dir: vocab.Forward}, policy.Key("alt+down")},
	{"core.pageStart", "commands.pageStart", vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, policy.Key("primary+up")},
	{"core.pageEnd", "commands.pageEnd", vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}, policy.Key("primary+down")},
	{"core.pageUp", "commands.pageUp", vocab.VisualMove{Dir: vocab.PageUp}, policy.Key("pageup")},
	{"core.pageDown", "commands.pageDown", vocab.VisualMove{Dir: vocab.PageDown}, policy.Key("pagedown")},
	{"core.deleteBackward", "commands.deleteBackward", vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}, policy.Key("backspace")},
	{"core.deleteForward", "commands.deleteForward", vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward}, policy.Key("delete")},
	{"core.deleteWordBackward", "commands.deleteWordBackward", vocab.Delete{Unit: vocab.ByWord, Dir: vocab.Backward}, policy.Key("alt+backspace")},
	{"core.deleteWordForward", "commands.deleteWordForward", vocab.Delete{Unit: vocab.ByWord, Dir: vocab.Forward}, policy.Key("alt+delete")},
	{"core.deleteToStart", "commands.deleteToStart", vocab.Delete{Unit: vocab.ToSlotEdge, Dir: vocab.Backward}, policy.Key("primary+backspace")},
	{"core.deleteToEnd", "commands.deleteToEnd", vocab.Delete{Unit: vocab.ToSlotEdge, Dir: vocab.Forward}, policy.Key("primary+delete")},
	{"core.break", "commands.break", vocab.Break{}, policy.Key("enter")},
	{"core.softBreak", "commands.softBreak", vocab.SoftBreak{}, policy.Key("shift+enter")},
	{"core.leave", "commands.leave", vocab.Leave{}, policy.Key("tab")},
	{"core.return", "commands.return", vocab.Return{}, policy.Key("shift+tab")},
	{"core.dismiss", "commands.dismiss", vocab.Dismiss{}, policy.Key("escape")},
}

func swallow[I vocab.Intent](*policy.Ctx, I, policy.At[doc.SlotRef]) policy.Result {
	return policy.Swallow
}

// --- text ---------------------------------------------------------------------

// opening is a block opened at a gap that earlier edits of the same
// transaction may have moved, as deleting a selection moves it.
type opening struct{ gap doc.Pos }

// openAt is the edit opening a block at gap, mapped through the edits
// before it.
func openAt(gap doc.Pos) (*opening, vocab.Edit) {
	o := &opening{gap: gap}
	return o, func(tx *doc.Tx) error {
		gap, _ := doc.Steps(tx.Steps()).Map(o.gap, doc.BiasBefore)
		_, err := o.open(tx, gap)
		return err
	}
}

// open opens a block the slot at gap takes (GapBlock) there, a gap in the
// transaction's document, and returns the start of its text.
func (o *opening) open(tx *doc.Tx, gap doc.Pos) (doc.Pos, error) {
	o.gap = gap
	spec, ok := vocab.SlotSpec(tx, gap.Ref())
	if !ok {
		return doc.Pos{}, doc.ErrSlot
	}
	block, ok := vocab.GapBlock(tx.Schema(), spec)
	if !ok {
		return doc.Pos{}, doc.ErrSlot
	}
	if err := tx.Apply(doc.Insert{Parent: gap.Node, Slot: gap.Slot, Index: gap.Offset, Nodes: []*doc.Node{block}}); err != nil {
		return doc.Pos{}, err
	}
	n, _ := tx.Node(gap.Node)
	return doc.Pos{Node: n.Slot(gap.Slot).Child(gap.Offset).ID()}, nil
}

// caret puts the caret at the end of the opened block.
func (o *opening) caret(r doc.Reader) doc.Selection {
	if c, ok := vocab.BlockCaret(r, o.gap, vocab.Forward); ok {
		return doc.Collapsed(c)
	}
	return doc.Collapsed(doc.CaretAt(o.gap))
}

// plain is an empty plain block.
func plain(s *doc.Schema) *doc.Node { return vocab.NewBlock(s, s.Plain(), doc.Attrs{}) }

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
	clear, pos, ok := c.Replacing()
	if ok {
		edits = append(edits, clear)
		res.Replaced = &pos
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
	o := &opening{gap: pos}
	edits = append(edits, insertText(pos, text, pending, pending != nil, o))
	res.Edits = edits
	if spec, ok := vocab.SlotSpec(c.Doc(), pos.Ref()); ok && spec.Blocks {
		res.Then = o.caret
	}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// insertText inserts text at a position that may have moved under earlier
// edits of the same transaction: it is resolved against the transaction
// by mapping the caret, so it runs after a selection was deleted. At a
// gap, the text goes into a block o opens there.
func insertText(at doc.Pos, text string, marks *doc.MarkSet, explicit bool, o *opening) vocab.Edit {
	return func(tx *doc.Tx) error {
		pos, _ := doc.Steps(tx.Steps()).Map(at, doc.BiasBefore)
		if spec, ok := vocab.SlotSpec(tx, pos.Ref()); ok && spec.Blocks {
			if o == nil {
				o = &opening{}
			}
			var err error
			if pos, err = o.open(tx, pos); err != nil {
				return err
			}
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
	if clear, _, ok := c.Clearing(); ok {
		return policy.Do(clear)
	}
	if c.Selection().Kind != doc.SelectText {
		// Whole pieces holding nothing, as empty cells: nothing goes.
		return policy.Swallow
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

// deleteAt deletes one unit of content from head. At a slot's edge there
// is none: what a delete crosses there is answered at the crossings
// (deleteAcross), and the deletion policy's; crossing nothing it can
// take, as between two cells, it steps on to the next stop.
func deleteAt(c *policy.Ctx, in vocab.Delete, head doc.Caret) policy.Result {
	in.Dir = in.Toward()
	r := c.Doc()
	if c.Bound(head) != head.Pos {
		// Before a block's marker: what is crossed there is the block's.
		return policy.Swallow
	}
	pos := head.Pos
	unit := in.Unit
	if unit == vocab.ToPageEdge {
		// Deleting runs to the slot's edge at most.
		unit = vocab.ToSlotEdge
	}
	target, ok := slotTarget(r, pos, unit, in.Dir)
	if !ok || target == pos {
		if next, ok := c.Stops().Step(head, in.Dir); ok && vocab.IsText(r, pos) {
			return policy.Select(doc.Collapsed(next))
		}
		return policy.Swallow
	}
	if unit == vocab.ByGrapheme {
		// A projected glyph between the caret and the content is not
		// content: the grapheme deleted is the glyph, which only its
		// owner edits, so core crosses it.
		if next, ok := c.Stops().Step(head, in.Dir); ok && next.Pos == head.Pos {
			return policy.Select(doc.Collapsed(next))
		}
	}
	from, to := pos, target
	if from.Offset > to.Offset {
		from, to = to, from
	}
	return policy.Do(vocab.Ops(doc.EditText{At: pos.Ref(), From: from.Offset, To: to.Offset}))
}

// deleteAcross answers a delete at what it crosses: a block left from its
// start steps its level down; a container left from its first block lifts
// that block out, unless the block is an entry, whose boundary is the
// container's, or where the slot around the container does not take it;
// a boundary joins the text blocks beside it where they can join, the
// last before it and the first after it however deep, as a list's first
// item is pulled up into the paragraph before the list; a block
// entered forward at its marker steps its level down. Anything else is
// not core's to delete: a nearer crossing passed, a farther one answers.
func deleteAcross(c *policy.Ctx, _ vocab.Delete, at policy.At[policy.Crossing]) policy.Result {
	r := c.Doc()
	x := at.Data
	switch {
	case x.Node == nil && x.Kind != policy.CrossBoundary:
		// A fact's edge is its owner's.
		return policy.Pass
	case x.Kind == policy.CrossBoundary:
		to, ok1 := vocab.LastTextSlot(x.Before)
		from, ok2 := vocab.FirstTextSlot(x.After)
		if ok1 && ok2 && vocab.CanJoin(r, to, from) {
			return policy.Do(vocab.Join(to, from.Node))
		}

	case x.Kind == policy.CrossExit && x.Dir == vocab.Backward && x.From == nil,
		x.Kind == policy.CrossEnter && x.Dir == vocab.Forward:
		if level, ok := policy.As(c, x.Node, vocab.RoleLeveled); ok {
			return policy.Do(level.Step(vocab.Backward))
		}
	case x.Kind == policy.CrossExit && x.Dir == vocab.Backward && !r.Schema().IsEntry(x.From.Kind()) && vocab.CanLift(r, x.From.ID()):
		return policy.Do(vocab.Lift(x.From.ID()))
	}
	return policy.Pass
}

// breakBlock is Enter: a line break in source text that keeps lines, as
// a code or math block's, but on a block's empty last line it leaves the
// block; so does an object's text that keeps lines, as a cell's, which
// never leaves the object by splitting. Other rich text splits where it
// keeps lines too, its line breaks being Shift+Enter's (softBreak). An
// empty block leaves the
// container holding it, lifted out as Backspace lifts a container's first
// block, where the slot around the container takes it (an entry leaves
// through its own level); a split elsewhere. At a
// block's text start, and before its marker, a block opens before it and
// the caret stays; a heading split at its end continues as a paragraph.
// An object breaks beside it: selected, or from an own slot that keeps
// no lines, the break is made right after it, as though the caret stood
// there.
func breakBlock(c *policy.Ctx, _ vocab.Break, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	var edits []vocab.Edit
	var clear vocab.Edit
	var pos doc.Pos
	if obj, ok := selectedObject(c); ok {
		pos, ok = vocab.Beside(r, obj.ID(), vocab.Forward)
		if !ok {
			return policy.Swallow
		}
	} else {
		var cleared bool
		clear, pos, cleared = c.Replacing()
		if cleared {
			edits = append(edits, clear)
		}
		if obj, ok := vocab.ObjectOf(r, pos.Node); ok {
			if spec, _ := vocab.SlotSpec(r, pos.Ref()); !spec.Lines {
				if pos, ok = vocab.Beside(r, obj, vocab.Forward); !ok {
					return policy.Swallow
				}
				clear = nil
			}
		}
	}
	spec, ok := vocab.SlotSpec(r, pos.Ref())
	if !ok {
		return policy.Swallow
	}
	if spec.Blocks {
		o, edit := openAt(pos)
		res := policy.Do(append(edits, edit)...)
		res.Then = o.caret
		return res
	}
	if _, whole := vocab.ObjectOf(r, pos.Node); spec.Lines && (spec.Literal || whole) {
		if clear == nil {
			if res, ok := leaveLines(r, pos); ok {
				return res
			}
		}
		return policy.Do(append(edits, insertText(pos, "\n", nil, true, nil))...)
	}
	if vocab.Inline(r, pos.Node) {
		return policy.Swallow
	}
	block, ok := vocab.Block(r, pos)
	if !ok {
		return policy.Swallow
	}
	if clear == nil && vocab.HoldsNothing(block) && !r.Schema().IsEntry(block.Kind()) && vocab.CanLift(r, block.ID()) {
		res := policy.Do(vocab.Lift(block.ID()))
		res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
		return res
	}
	if clear == nil && pos.Offset == 0 && pos.Slot == 0 && block.Slot(0).Len() > 0 {
		// The block keeps its text and the caret; an empty one opens
		// before it.
		kind, attrs := continuation(c, block)
		parent, slot, index, ok := vocab.Index(r, block.ID())
		if !ok {
			return policy.Swallow
		}
		res := policy.Do(vocab.Ops(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{vocab.NewBlock(r.Schema(), kind, attrs)}}))
		res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
		return res
	}
	// The block split is the one the caret stands in once the selection is
	// cleared: what clearing leaves of it, as an item a range from before
	// its marker unlists, is what the split continues.
	var split doc.ID
	res := policy.Do(append(edits, func(tx *doc.Tx) error {
		at, _ := doc.Steps(tx.Steps()).Map(pos, doc.BiasBefore)
		b, ok := vocab.Block(tx, at)
		if !ok {
			return vocab.ErrHelper
		}
		split = b.ID()
		kind, attrs := b.Kind(), b.Attrs()
		if at.Offset >= b.Slot(at.Slot).Len() {
			kind, attrs = continuation(c.Over(tx, c.View()), b)
		}
		return vocab.Split(at, kind, attrs)(tx)
	})...)
	res.Then = func(r doc.Reader) doc.Selection { return startOfNext(split)(r) }
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// continuation is the kind a block opened next to block takes: what it
// continues as, or a plain block.
func continuation(c *policy.Ctx, block *doc.Node) (doc.Kind, doc.Attrs) {
	if next, ok := policy.As(c, block, vocab.RoleContinued); ok {
		return next.Next()
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

// leaveLines is Enter on the empty last line of a block whose text keeps
// lines, as a code or math block: the block is left, the empty lines going
// and a plain block opening after it, where the slot around it takes one.
// Shift+Enter still adds the line.
func leaveLines(r doc.Reader, pos doc.Pos) (policy.Result, bool) {
	n, ok := r.Node(pos.Node)
	if !ok {
		return policy.Result{}, false
	}
	parent, slot, index, ok := vocab.Index(r, pos.Node)
	if !ok {
		return policy.Result{}, false
	}
	// The slot around the block must take the plain block that opens, as a
	// row takes only cells.
	if ps, ok := vocab.SlotSpec(r, doc.SlotRef{Node: parent, Slot: slot}); !ok || !ps.Blocks || ps.Accepts == nil || !ps.Accepts(r.Schema().Plain()) {
		return policy.Result{}, false
	}
	// The last line holds nothing, not even an inline object. Code takes
	// one empty line as its own, a blank between its parts, so it is left
	// from the second, both going; an object's source, as a math block's,
	// has no blank lines and is left from the first.
	empty := "\n"
	if _, whole := vocab.ObjectOf(r, pos.Node); !whole {
		empty = "\n\n"
	}
	s := n.Slot(pos.Slot)
	if pos.Offset != s.Len() || !strings.HasSuffix(s.Content().Flat(), empty) {
		return policy.Result{}, false
	}
	res := policy.Do(vocab.Ops(
		doc.EditText{At: pos.Ref(), From: pos.Offset - len(empty), To: pos.Offset},
		doc.Insert{Parent: parent, Slot: slot, Index: index + 1, Nodes: []*doc.Node{plain(r.Schema())}},
	))
	res.Then = startOfNext(pos.Node)
	return res, true
}

// selectedObject is the object selected whole, if one is.
func selectedObject(c *policy.Ctx) (*doc.Node, bool) {
	sel := c.Selection()
	if sel.Kind != doc.SelectNode {
		return nil, false
	}
	n, ok := c.Doc().Node(sel.Node)
	return n, ok && c.Doc().Schema().IsObject(n.Kind())
}

// softBreak inserts a line break where the slot keeps lines, and is Enter
// elsewhere.
func softBreak(c *policy.Ctx, _ vocab.SoftBreak, at policy.At[doc.SlotRef]) policy.Result {
	clear, pos, cleared := c.Clearing()
	if spec, ok := vocab.SlotSpec(c.Doc(), pos.Ref()); ok && spec.Lines && !spec.Blocks {
		var edits []vocab.Edit
		if cleared {
			edits = append(edits, clear)
		}
		return policy.Do(append(edits, insertText(pos, "\n", nil, true, nil))...)
	}
	return breakBlock(c, vocab.Break{}, at)
}

// paste inserts the clipboard's slice, or its text as paragraphs, fitted
// to the slot.
func paste(c *policy.Ctx, in vocab.Paste, _ policy.At[doc.SlotRef]) policy.Result {
	slice := in.Slice
	text := slice.Empty()
	if text {
		slice = TextSlice(c.Doc().Schema(), in.Text)
	}
	if slice.Empty() {
		return policy.Swallow
	}
	var edits []vocab.Edit
	clear, pos, ok := c.Replacing()
	if ok {
		edits = append(edits, clear)
	}
	// The caret goes where the pasted content ends.
	var end *doc.Pos
	edits = append(edits, func(tx *doc.Tx) error {
		at, _ := doc.Steps(tx.Steps()).Map(pos, doc.BiasBefore)
		if spec, ok := vocab.SlotSpec(tx, at.Ref()); ok && spec.Blocks {
			if err := tx.Apply(doc.Insert{Parent: at.Node, Slot: at.Slot, Index: at.Offset, Nodes: []*doc.Node{plain(tx.Schema())}}); err != nil {
				return err
			}
			n, _ := tx.Node(at.Node)
			at = doc.Pos{Node: n.Slot(at.Slot).Child(at.Offset).ID()}
		}
		end = &doc.Pos{}
		if spec, ok := vocab.SlotSpec(tx, at.Ref()); ok && spec.Literal && in.Text != "" {
			// Literal slots (code, math, formula) keep the clipboard's raw characters
			// untouched rather than parsed structure.
			slice = TextSlice(tx.Schema(), in.Text)
			text = true
		}
		if block, ok := vocab.Block(tx, at); ok && text {
			// Lines of text pasted into an item are items of its list.
			slice.Nodes = vocab.AsEntries(tx.Schema(), block, slice.Nodes)
		}
		if err := vocab.PasteSlice(at, slice, end)(tx); err != nil {
			return err
		}
		if end.Node == 0 {
			end = nil
		}
		return nil
	})
	res := policy.Result{Edits: edits, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
	res.Then = func(r doc.Reader) doc.Selection {
		if end == nil {
			// Content ending in no text, as a figure: the caret after it.
			at := pos
			if tx, ok := r.(*doc.Tx); ok {
				at, _ = doc.Steps(tx.Steps()).Map(pos, doc.BiasAfter)
			}
			return doc.Collapsed(doc.CaretAt(at))
		}
		return doc.Collapsed(doc.CaretAt(*end))
	}
	if ok {
		res.Replaced = &pos
	}
	return res
}

// TextSlice is plain text as a slice: one line inline, several lines as
// paragraphs with open edges, sanitized for the document model.
func TextSlice(s *doc.Schema, text string) vocab.Slice {
	text = sanitizePlainText(text)
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

func sanitizePlainText(text string) string {
	text = strings.ToValidUTF8(text, "\uFFFD")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.ReplaceAll(text, string(doc.NodeByte), "")
}

// cut deletes the selection; the host copied it before sending Cut.
func cut(c *policy.Ctx, _ vocab.Cut, _ policy.At[doc.SlotRef]) policy.Result {
	if clear, _, ok := c.Clearing(); ok {
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
	} else if clear, from, ok := c.Clearing(); ok {
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
	first, ok1 := stops.Edge(vocab.Backward)
	last, ok2 := stops.Edge(vocab.Forward)
	if !ok1 || !ok2 {
		return policy.Swallow
	}
	return policy.Select(doc.TextRange(first, last))
}

// dismiss collapses the selection to its head and ends a composition.
// Leaving a scope by Escape is the scopes policy's.
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

func moveNode(c *policy.Ctx, in vocab.MoveNode, _ policy.At[doc.SlotRef]) policy.Result {
	return policy.Do(vocab.MoveTo(in.ID, in.Target))
}
