// Package lists is the lists policy, over items: Enter in an empty item
// steps it out of its list, and nesting moves an item into the item before
// it or one level out. Both step the item's own level (vocab.Leveled), as
// Backspace at an item's start does through core, so an item has one
// notion of its depth whichever key changes it.
package lists

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"time"
)

// ID is the policy's id.
const ID = "lists"

// Policy is the lists policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	p.Command("lists.bullet", "commands.bulletList", profile.MakeList{})
	p.Command("lists.nest", "commands.nest", profile.Nest{Dir: vocab.Forward}, policy.Key("tab"))
	p.Command("lists.outdent", "commands.outdent", profile.Nest{Dir: vocab.Backward}, policy.Key("shift+tab"))
	p.Command("lists.toggleTask", "commands.toggleTask", profile.ToggleTask{}, policy.Key("primary+enter"))
	p.Command("lists.numbered", "commands.numberedList", profile.MakeList{
		Attrs: doc.MakeAttrs(doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(profile.Ordered))},
			doc.Field{Num: profile.FieldListStart, Value: doc.Int(1)}),
	})

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

// makeList makes each block the selection touches an item; lists of one
// style meet as one list.
func makeList(c *policy.Ctx, in profile.MakeList, _ policy.At[doc.SlotRef]) policy.Result {
	from, to := c.Range()
	blocks := vocab.SelectedBlocks(c.Doc(), from, to)
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
// holding the caret, its body included.
func toggleTask(c *policy.Ctx, in profile.ToggleTask, _ policy.At[doc.SlotRef]) policy.Result {
	id := in.ID
	if id == 0 {
		id = itemAt(c.Doc(), c.Head().Pos)
	}
	n, ok := c.Doc().Node(id)
	if !ok || n.Kind() != profile.Item || profile.TaskOf(n) == profile.NoTask {
		return policy.Swallow
	}
	return policy.Do(profile.FlipTask(id))
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
