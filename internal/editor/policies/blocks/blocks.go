// Package blocks is the blocks policy: the text blocks a selection
// touches take another kind, or a quote around them; outdented, they leave
// the container holding them. In source text, as a code block's, nesting
// is the lines' indentation.
package blocks

import (
	"strings"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"time"
)

// ID is the policy's id.
const ID = "blocks"

// Policy is the blocks policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	in := policy.Slot.Inside()
	policy.On(p, "retype", policy.Intent[profile.Retype](), in, retype).Caret().Range()
	policy.On(p, "wrap-quote", policy.Intent[profile.WrapQuote](), in, wrapQuote).Caret().Range()
	policy.On(p, "nest", policy.Intent[profile.Nest](), in, nest).Caret().Range()
	policy.On(p, "jump-heading", policy.Intent[profile.JumpHeading](), in, jumpHeading).Any()
	p.Command("blocks.previousHeading", "commands.previousHeading", profile.JumpHeading{Dir: vocab.Backward}, policy.Key("alt+primary+up"))
	p.Command("blocks.nextHeading", "commands.nextHeading", profile.JumpHeading{Dir: vocab.Forward}, policy.Key("alt+primary+down"))

	// The kinds are offered for whole blocks, or at a caret for its block:
	// never for a part of a block's text.
	kinds := policy.Spans(policy.SpanCaret | policy.SpanBlocks)
	p.Command("blocks.paragraph", "commands.paragraph", profile.Retype{To: profile.Paragraph},
		policy.Offer(profile.KindOffers, 0, "p", kinds))
	for level := 1; level <= 3; level++ {
		digit := string(rune('0' + level))
		p.Command(vocab.CommandID("blocks.heading"+digit), "commands.heading"+digit, profile.Retype{To: profile.Heading, Attrs: headingAttrs(level)},
			policy.Offer(profile.KindOffers, level, digit, kinds))
	}
	p.Command("blocks.quote", "commands.quote", profile.WrapQuote{}, policy.Offer(profile.KindOffers, 4, "q", kinds))
	p.Command("blocks.codeBlock", "commands.codeBlock", profile.Retype{To: profile.Code}, policy.Offer(profile.KindOffers, 7, "c", kinds))
	policy.State(p, policy.Intent[profile.Retype](), retyped)
	policy.State(p, policy.Intent[profile.WrapQuote](), quoted)

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("Tides"),
		policy.Waits(500*time.Millisecond),
		policy.Presses(profile.Retype{To: profile.Heading, Attrs: headingAttrs(1)}),
		policy.Waits(600*time.Millisecond),
		policy.Presses(profile.Retype{To: profile.Heading, Attrs: headingAttrs(2)}),
		policy.Waits(600*time.Millisecond),
		policy.Presses(profile.WrapQuote{}),
	)
}, policy.Label("policies.blocks"), policy.Description("policies.blocks.description"))

func headingAttrs(level int) doc.Attrs {
	return doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(int64(level))})
}

// retyped is whether every text block the selection touches is of the
// kind, with its attributes: where retyping makes them plain again.
func retyped(c *policy.Ctx, in profile.Retype) bool {
	r := c.Doc()
	attrs := r.Schema().Canonical(in.To, in.Attrs)
	blocks := c.Span().Blocks
	for _, b := range blocks {
		if b.Kind() != in.To || !b.Attrs().Equal(attrs) {
			return false
		}
	}
	return len(blocks) > 0
}

// quoted is whether every text block the selection touches is in a
// quote.
func quoted(c *policy.Ctx, _ profile.WrapQuote) bool {
	blocks := c.Span().Blocks
	for _, b := range blocks {
		if _, ok := policy.Within(c.Doc(), b, profile.Quote); !ok {
			return false
		}
	}
	return len(blocks) > 0
}

// retype gives the text blocks the selection touches a kind where their
// slot takes it; blocks that already have it, with its attributes, become
// plain again.
func retype(c *policy.Ctx, in profile.Retype, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	attrs := r.Schema().Canonical(in.To, in.Attrs)
	var edits []vocab.Edit
	for _, b := range c.Span().Blocks {
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
	blocks := c.Span().Blocks
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

// nest steps the selection one level: in source text its lines'
// indentation, nearest; else, outdented, the blocks leave their container.
func nest(c *policy.Ctx, in profile.Nest, at policy.At[doc.SlotRef]) policy.Result {
	if res, ok := indent(c, in); ok {
		return res
	}
	return outdent(c, in, at)
}

// outdent lifts the text blocks the selection touches out of the
// container holding them, one level, as Backspace lifts a container's
// first block and Enter an empty one: outdent is one container less. An
// entry leaves through its own level (lists), and a block the slot around
// its container does not take stays.
func outdent(c *policy.Ctx, in profile.Nest, _ policy.At[doc.SlotRef]) policy.Result {
	if in.Dir != vocab.Backward {
		return policy.Pass
	}
	r := c.Doc()
	var ids []doc.ID
	for _, b := range c.Span().Blocks {
		if !r.Schema().IsEntry(b.Kind()) && vocab.CanLift(r, b.ID()) {
			ids = append(ids, b.ID())
		}
	}
	if len(ids) == 0 {
		return policy.Pass
	}
	return policy.Do(func(tx *doc.Tx) error {
		for _, id := range ids {
			if err := vocab.Lift(id)(tx); err != nil {
				return err
			}
		}
		return nil
	})
}

// indent nests the lines a selection touches in source text one level, as
// Nest nests an item: one SourceIndent at each line's start, or one taken
// from it; not ok outside source text, or with no indentation to take,
// which is the container's to outdent.
func indent(c *policy.Ctx, in profile.Nest) (policy.Result, bool) {
	from, to := c.Range()
	ref := from.Ref()
	spec, ok := vocab.SlotSpec(c.Doc(), ref)
	content, ok2 := c.Inline(ref)
	if !ok || !ok2 || to.Ref() != ref || !spec.Literal || !spec.Lines {
		return policy.Result{}, false
	}
	flat := content.Flat()
	start, end := vocab.LineAround(flat, from.Offset, to.Offset)
	type change struct{ at, n int }
	var changes []change
	for at := start; at <= end; {
		n := len(profile.SourceIndent)
		if in.Dir == vocab.Backward {
			n = 0
			for n < len(profile.SourceIndent) && at+n < len(flat) && flat[at+n] == ' ' {
				n++
			}
			if n == 0 && at < len(flat) && flat[at] == '\t' {
				n = 1
			}
		}
		if n > 0 {
			changes = append(changes, change{at, n})
		}
		next := strings.IndexByte(flat[at:end], '\n')
		if next < 0 {
			break
		}
		at += next + 1
	}
	if len(changes) == 0 {
		return policy.Result{}, false
	}
	var edits []vocab.Edit
	for i := len(changes) - 1; i >= 0; i-- {
		ch := changes[i]
		if in.Dir == vocab.Backward {
			edits = append(edits, vocab.Ops(doc.EditText{At: ref, From: ch.at, To: ch.at + ch.n}))
		} else {
			edits = append(edits, vocab.Ops(doc.EditText{At: ref, From: ch.at, To: ch.at, Items: []doc.Item{doc.TextItem(profile.SourceIndent, nil)}}))
		}
	}
	// Each end keeps its place in its line's text, the indentation moving
	// before it.
	moved := func(off int) int {
		out := off
		for _, ch := range changes {
			switch {
			case in.Dir != vocab.Backward && ch.at <= off:
				out += ch.n
			case in.Dir == vocab.Backward && ch.at < off:
				out -= min(ch.n, off-ch.at)
			}
		}
		return out
	}
	sel := c.Selection()
	sel.Anchor = doc.CaretAt(doc.At(ref, moved(sel.Anchor.Pos.Offset)))
	sel.Head = doc.CaretAt(doc.At(ref, moved(sel.Head.Pos.Offset)))
	res := policy.Do(edits...)
	res.Selection = &sel
	return res, true
}
