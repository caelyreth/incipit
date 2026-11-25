package blocks

import (
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// moveBlock moves the caret's block past its sibling before or after it,
// the caret going with it: a task moved up its list, a paragraph down its
// quote. At either end of its slot it stays.
func moveBlock(c *policy.Ctx, in profile.MoveBlock, at policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	id := at.Data.Node
	for {
		n, ok := r.Node(id)
		if !ok {
			return policy.Swallow
		}
		if spec, ok := r.Schema().Kind(n.Kind()); !ok || spec.Placement != doc.PlaceInline {
			break
		}
		if id, _, ok = r.Parent(id); !ok {
			return policy.Swallow
		}
	}
	parent, slot, ok := r.Parent(id)
	if !ok {
		return policy.Swallow
	}
	p, _ := r.Node(parent)
	n, _ := r.Node(id)
	i, ok := p.Slot(slot).IndexOf(n)
	if !ok {
		return policy.Swallow
	}
	to := i - 1
	if in.Dir == vocab.Forward {
		to = i + 2
	}
	if to < 0 || to > p.Slot(slot).Len() {
		return policy.Swallow
	}
	return policy.Do(vocab.MoveTo(id, doc.Pos{Node: parent, Slot: slot, Offset: to}))
}
