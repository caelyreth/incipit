// Package lists is the lists policy, over items: Enter in an empty item
// steps it out of its list, and nesting moves an item into the item before
// it or one level out. Both step the item's own level (vocab.Leveled), as
// Backspace at an item's start does through core, so an item has one
// notion of its depth whichever key changes it.
package lists

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"time"
)

// ID is the policy's id.
const ID = "lists"

// Policy is the lists policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	kinds := policy.Spans(policy.SpanCaret | policy.SpanBlocks)
	// ⇧⌘8 and ⇧⌘7 make lists, as word processors have them.
	p.Command("lists.bullet", "commands.bulletList", profile.MakeList{}, policy.Offer(profile.KindOffers, 5, "l", kinds), policy.Key("shift+primary+8"))
	p.Command("lists.nest", "commands.nest", profile.Nest{Dir: vocab.Forward}, policy.Key("tab"))
	p.Command("lists.outdent", "commands.outdent", profile.Nest{Dir: vocab.Backward}, policy.Key("shift+tab"))
	p.Command("lists.toggleTask", "commands.toggleTask", profile.ToggleTask{}, policy.Key("primary+enter"))
	p.Command("lists.numbered", "commands.numberedList", profile.MakeList{
		Attrs: doc.MakeAttrs(doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(profile.Ordered))},
			doc.Field{Num: profile.FieldListStart, Value: doc.Int(1)}),
	}, policy.Offer(profile.KindOffers, 6, "n", kinds), policy.Key("shift+primary+7"))
	policy.State(p, policy.Intent[profile.MakeList](), listed)

	items := policy.Node(profile.Item)
	policy.On(p, "leave-empty", policy.Intent[vocab.Break](), items.Empty().Own(), leave)
	policy.On(p, "nest", policy.Intent[profile.Nest](), items.Own(), nest).Any()
	policy.On(p, "make-list", policy.Intent[profile.MakeList](), policy.Slot.Inside(), makeList).Caret().Range()
	policy.On(p, "toggle-task", policy.Intent[profile.ToggleTask](), policy.Slot.Inside(), toggleTask).Any()

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("- tides\nebb"),
		policy.Waits(500*time.Millisecond),
		policy.Presses(profile.Nest{Dir: vocab.Forward}),
		policy.Waits(300*time.Millisecond),
		policy.Types("\nflow"),
		policy.Waits(500*time.Millisecond),
		policy.Presses(profile.Nest{Dir: vocab.Backward}),
	)
}, policy.Label("policies.lists"), policy.Description("policies.lists.description"))

// level is the item's own level.
func level(c *policy.Ctx, item *doc.Node) (vocab.Leveled, bool) {
	return policy.As(c, item, vocab.RoleLeveled)
}

// leave steps an empty item one level out: a nested item follows the item
// holding it, a top-level one becomes a paragraph after its list.
func leave(c *policy.Ctx, _ vocab.Break, at policy.At[*doc.Node]) policy.Result {
	l, ok := level(c, at.Data)
	if !ok {
		return policy.Swallow
	}
	return policy.Result{Edits: []vocab.Edit{l.Step(vocab.Backward)}, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
}

// nest moves the item in (Forward) or out.
func nest(c *policy.Ctx, in profile.Nest, at policy.At[*doc.Node]) policy.Result {
	l, ok := level(c, at.Data)
	if !ok {
		return policy.Swallow
	}
	dir := in.Dir
	if dir == 0 {
		dir = vocab.Forward
	}
	return policy.Do(l.Step(dir))
}

// listed is whether every text block the selection touches is in a list
// of the style, the nearest list holding it.
func listed(c *policy.Ctx, in profile.MakeList) bool {
	style := profile.Bullet
	if v, ok := in.Attrs.Get(profile.FieldListStyle); ok {
		style = profile.ListStyle(v.Int())
	}
	blocks := c.Span().Blocks
	for _, b := range blocks {
		list, ok := policy.Within(c.Doc(), b, profile.List)
		if !ok || profile.ListStyleOf(list) != style {
			return false
		}
	}
	return len(blocks) > 0
}

// makeList makes each block the selection touches an item; lists of one
// style meet as one list.
func makeList(c *policy.Ctx, in profile.MakeList, _ policy.At[doc.SlotRef]) policy.Result {
	blocks := c.Span().Blocks
	if len(blocks) == 0 {
		return policy.Swallow
	}
	edits := make([]vocab.Edit, len(blocks))
	for i, b := range blocks {
		edits[i] = profile.IntoList(b.ID(), in.Attrs)
	}
	return policy.Do(edits...)
}

// toggleTask completes or reopens the task the intent names, or the one
// holding the caret, its body included. At a caret in an item that is no
// task, or in a paragraph, it makes the line an open task, as typing
// "- [ ] " before it does.
func toggleTask(c *policy.Ctx, in profile.ToggleTask, at policy.At[doc.SlotRef]) policy.Result {
	id := in.ID
	if id == 0 {
		id = itemAt(c.Doc(), c.Head().Pos)
	}
	n, ok := c.Doc().Node(id)
	switch {
	case ok && n.Kind() == profile.Item && profile.TaskOf(n) != profile.NoTask:
		return policy.Do(profile.FlipTask(id))
	case ok && n.Kind() == profile.Item:
		return policy.Do(profile.IntoTask(id, doc.Attrs{}))
	case in.ID == 0:
		if b, ok := c.Doc().Node(at.Data.Node); ok && b.Kind() == profile.Paragraph {
			return policy.Do(profile.IntoTask(b.ID(), doc.Attrs{}))
		}
	}
	return policy.Swallow
}

// itemAt is the item holding p, or zero.
func itemAt(r doc.Reader, p doc.Pos) doc.ID {
	for id := p.Node; ; {
		n, ok := r.Node(id)
		if !ok {
			return 0
		}
		if n.Kind() == profile.Item {
			return n.ID()
		}
		parent, _, ok := r.Parent(id)
		if !ok {
			return 0
		}
		id = parent
	}
}
