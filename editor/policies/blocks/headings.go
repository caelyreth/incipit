package blocks

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// jumpHeading puts the caret at the start of the nearest heading the way
// in goes from the caret, of any level and however deep it stands: going
// back from inside a heading, its own start first. With none that way,
// the caret stays.
func jumpHeading(c *policy.Ctx, in profile.JumpHeading, _ policy.At[doc.SlotRef]) policy.Result {
	r, head := c.Doc(), c.Head()
	var to doc.Caret
	found := false
	for _, start := range headingStarts(c) {
		n := doc.Compare(r, start, head)
		switch {
		case in.Dir == vocab.Forward && n > 0:
			return reveal(start)
		case in.Dir == vocab.Backward && n < 0:
			to, found = start, true
		}
	}
	if !found {
		return policy.Swallow
	}
	return reveal(to)
}

// headingStarts are where each heading's text starts, in reading order.
func headingStarts(c *policy.Ctx) []doc.Caret {
	var out []doc.Caret
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		if n.Kind() == profile.Heading {
			out = append(out, c.Stops().Normalize(doc.CaretAt(doc.At(doc.SlotRef{Node: n.ID()}, 0))))
			return
		}
		for _, slot := range n.Slots() {
			if !slot.IsInline() {
				for _, child := range slot.Children() {
					walk(child)
				}
			}
		}
	}
	walk(c.Doc().Root())
	return out
}

// reveal puts the caret at to, bringing it into view.
func reveal(to doc.Caret) policy.Result { return policy.Reveal(doc.Collapsed(to)) }
