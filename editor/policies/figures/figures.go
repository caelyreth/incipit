// Package figures is the figures policy: pictures placed as figures, and
// the keys over a figure. A figure is one object in the slot that holds
// it, selected whole, with its caption as its text. Selected, Return
// continues below it, Escape goes to its caption, and typing or deleting
// replaces it as any selection is (core); in its caption, Return continues
// below it as in any block, Escape selects it, and Backspace at the
// caption's start selects it as deleting toward any object does.
package figures

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "figures"

// Policy is the figures policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	p.Command("figures.insert", "commands.insertPicture", profile.PickPictures{})
	p.Command("figures.alignStart", "commands.alignPictureStart", profile.AlignFigure{To: profile.AlignStart})
	p.Command("figures.alignCenter", "commands.alignPictureCenter", profile.AlignFigure{To: profile.AlignCenter})

	in := policy.Slot.Inside()
	policy.On(p, "pick", policy.Intent[profile.PickPictures](), in, pick).Any()
	policy.On(p, "place", policy.Intent[profile.PlaceFigures](), in, place).Any()
	policy.On(p, "resize", policy.Intent[profile.ResizeFigure](), in, resize).Any()

	figure := policy.Node(profile.Figure)
	policy.On(p, "align", policy.Intent[profile.AlignFigure](), policy.Or(figure.Selected(), figure.Own()), align).Any()
	policy.On(p, "continue", policy.Intent[vocab.Break](), figure.Selected(), below).Node()
	policy.On(p, "caption", policy.Intent[vocab.Dismiss](), figure.Selected(), caption).Node()
	policy.On(p, "split", policy.Intent[vocab.Break](), figure.Own(), split).Caret().Range()
	policy.On(p, "soft-split", policy.Intent[vocab.SoftBreak](), figure.Own(), split).Caret().Range()
	policy.On(p, "select", policy.Intent[vocab.Dismiss](), figure.Own(), selectFigure)
	policy.On(p, "select-before", policy.Deleting(0, vocab.Backward), policy.Or(figure.AtStart().Own(), figure.Empty().Own()), selectFigure)

	// The card's demonstration, played by the engine itself: a picture
	// still on its way shows its place.
	p.Showcase(2600*time.Millisecond,
		policy.Types("Low tide"),
		policy.Waits(400*time.Millisecond),
		policy.Presses(profile.PlaceFigures{Pictures: []profile.Picture{{Width: 1600, Height: 900}}}),
		policy.Waits(400*time.Millisecond),
		policy.Types("The bay at dawn"),
	)
}, policy.Label("policies.figures"), policy.Description("policies.figures.description"))

// pick asks the host for pictures; it answers with PlaceFigures.
func pick(*policy.Ctx, profile.PickPictures, policy.At[doc.SlotRef]) policy.Result {
	return policy.Result{Effects: []policy.Effect{{Kind: policy.EffectMedia}}}
}

// place puts pictures on the page as figures, in order, replacing what
// the selection covers, the caret going on in the last one's caption.
func place(c *policy.Ctx, in profile.PlaceFigures, _ policy.At[doc.SlotRef]) policy.Result {
	if len(in.Pictures) == 0 {
		return policy.Swallow
	}
	var clear vocab.Edit
	at := c.Head().Pos
	if in.At != nil {
		if _, ok := c.Doc().Node(in.At.Pos.Node); !ok {
			return policy.Swallow
		}
		at = in.At.Pos
	} else if sel := c.Selection(); sel.Kind == doc.SelectNode || !sel.IsCollapsed() {
		from, to := c.Range()
		at = from
		if sel.Kind == doc.SelectNode {
			clear = vocab.RemoveNode(sel.Node)
		} else {
			clear = vocab.DeleteRange(from, to)
		}
	}
	// The last figure's id, which the edit learns as it places them.
	var last doc.ID
	res := policy.Do(func(tx *doc.Tx) error {
		if clear != nil {
			if err := clear(tx); err != nil {
				return err
			}
			at, _ = doc.Steps(tx.Steps()).Map(at, doc.BiasBefore)
		}
		gap, err := gapFor(tx, at)
		if err != nil {
			return err
		}
		nodes := make([]*doc.Node, len(in.Pictures))
		for i, pic := range in.Pictures {
			nodes[i] = profile.NewFigure(tx.Schema(), pic)
		}
		if err := tx.Apply(doc.Insert{Parent: gap.Node, Slot: gap.Slot, Index: gap.Offset, Nodes: nodes}); err != nil {
			return err
		}
		pn, _ := tx.Node(gap.Node)
		last = pn.Slot(gap.Slot).Child(gap.Offset + len(nodes) - 1).ID()
		return nil
	})
	res.Then = func(doc.Reader) doc.Selection {
		return doc.Collapsed(doc.CaretAt(doc.Pos{Node: last}))
	}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// gapFor is where figures placed at a position go: a gap as it is; in a
// caption, after its figure; in an empty plain block, in its place; at a
// text's start or end, before or after its block, and inside it, between
// its halves. A block whose slot takes no figure, as an item's head is,
// takes them at the start of its own body; failing that, its container
// does, after it.
func gapFor(tx *doc.Tx, at doc.Pos) (doc.Pos, error) {
	s := tx.Schema()
	if spec, ok := vocab.SlotSpec(tx, at.Ref()); ok && spec.Blocks {
		if spec.Accepts != nil && spec.Accepts(profile.Figure) {
			return at, nil
		}
	}
	block, ok := vocab.Block(tx, at)
	if !ok {
		return doc.Pos{}, vocab.ErrHelper
	}
	parent, slot, index, _ := vocab.Index(tx, block.ID())
	if !vocab.Accepts(tx, block.ID(), profile.Figure) {
		for i, sl := range block.Slots() {
			if spec, ok := vocab.SlotSpec(tx, doc.SlotRef{Node: block.ID(), Slot: i}); ok && !sl.IsInline() && spec.Accepts != nil && spec.Accepts(profile.Figure) {
				return doc.Pos{Node: block.ID(), Slot: i}, nil
			}
		}
		for _, a := range doc.Ancestors(tx, block.ID())[1:] {
			if vocab.Accepts(tx, a, profile.Figure) {
				p, sl, i, _ := vocab.Index(tx, a)
				return doc.Pos{Node: p, Slot: sl, Offset: i + 1}, nil
			}
		}
		return doc.Pos{}, vocab.ErrHelper
	}
	after := doc.Pos{Node: parent, Slot: slot, Offset: index + 1}
	if block.Kind() == profile.Figure || at.Node != block.ID() {
		// A caption, or an inline node's source: after its block.
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
	if err := vocab.Split(at, block.Kind(), block.Attrs())(tx); err != nil {
		return doc.Pos{}, err
	}
	return after, nil
}

// resize sets a figure's span.
func resize(c *policy.Ctx, in profile.ResizeFigure, _ policy.At[doc.SlotRef]) policy.Result {
	n, ok := c.Doc().Node(in.ID)
	if !ok || n.Kind() != profile.Figure {
		return policy.Swallow
	}
	shape := profile.ShapeOf(n)
	if shape.Span == in.Span {
		return policy.Swallow
	}
	shape.Span = in.Span
	return policy.Do(profile.SetShape(in.ID, shape))
}

// align aligns the figure selected or captioned.
func align(c *policy.Ctx, in profile.AlignFigure, at policy.At[*doc.Node]) policy.Result {
	shape := profile.ShapeOf(at.Data)
	if shape.Align == in.To {
		return policy.Swallow
	}
	shape.Align = in.To
	return policy.Do(profile.SetShape(at.Data.ID(), shape))
}

// below opens a paragraph after the selected figure, the caret in it.
func below(c *policy.Ctx, _ vocab.Break, at policy.At[*doc.Node]) policy.Result {
	parent, slot, index, ok := vocab.Index(c.Doc(), at.Data.ID())
	if !ok {
		return policy.Swallow
	}
	gap := doc.Pos{Node: parent, Slot: slot, Offset: index + 1}
	s := c.Doc().Schema()
	res := policy.Do(vocab.Ops(doc.Insert{Parent: parent, Slot: slot, Index: gap.Offset, Nodes: []*doc.Node{vocab.NewBlock(s, s.Plain(), doc.Attrs{})}}))
	res.Then = func(r doc.Reader) doc.Selection {
		c, _ := vocab.BlockCaret(r, gap, vocab.Backward)
		return doc.Collapsed(c)
	}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// caption puts the caret at the end of the selected figure's caption.
func caption(_ *policy.Ctx, _ vocab.Dismiss, at policy.At[*doc.Node]) policy.Result {
	n := at.Data
	return policy.Reveal(doc.Collapsed(doc.CaretAt(doc.Pos{Node: n.ID(), Offset: n.Slot(0).Len()})))
}

// split continues below a figure from its caption: the caption's text
// after the caret goes on in a paragraph after it.
func split[I vocab.Intent](c *policy.Ctx, _ I, at policy.At[*doc.Node]) policy.Result {
	head := c.Head().Pos
	if head.Node != at.Data.ID() {
		// In an inline node's source within the caption: not a caption
		// break.
		return policy.Swallow
	}
	var edits []vocab.Edit
	if sel := c.Selection(); !sel.IsCollapsed() {
		from, to := c.Range()
		if from.Ref() != head.Ref() || to.Ref() != head.Ref() {
			// A range reaching out of the caption breaks as any does.
			return policy.Pass
		}
		edits = append(edits, vocab.DeleteRange(from, to))
		head = from
	}
	s := c.Doc().Schema()
	edits = append(edits, func(tx *doc.Tx) error {
		pos, _ := doc.Steps(tx.Steps()).Map(head, doc.BiasBefore)
		return vocab.Split(pos, s.Plain(), doc.Attrs{})(tx)
	})
	res := policy.Do(edits...)
	id := at.Data.ID()
	res.Then = func(r doc.Reader) doc.Selection {
		parent, slot, index, _ := vocab.Index(r, id)
		c, _ := vocab.BlockCaret(r, doc.Pos{Node: parent, Slot: slot, Offset: index + 1}, vocab.Backward)
		return doc.Collapsed(c)
	}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// selectFigure selects the figure whose caption holds the caret.
func selectFigure[I vocab.Intent](_ *policy.Ctx, _ I, at policy.At[*doc.Node]) policy.Result {
	return policy.Reveal(doc.NodeSelection(at.Data.ID()))
}
