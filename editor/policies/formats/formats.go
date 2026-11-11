// Package formats is the formats policy: a format toggles the marks text
// typed at a caret takes, as pending marks, and the marks of a range.
package formats

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "formats"

// Policy is the formats policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	format := policy.Intent[profile.Format]()
	policy.On(p, "at-caret", format, policy.Slot.Own(), formatAtCaret).When(policy.Rich)
	policy.On(p, "range", format, policy.Slot.Inside(), formatRange).Range()

	p.Command("formats.strong", "commands.strong", profile.Format{Mark: profile.Strong}, policy.Key("primary+b"))
	p.Command("formats.emphasis", "commands.emphasis", profile.Format{Mark: profile.Emphasis}, policy.Key("primary+i"))
	p.Command("formats.code", "commands.code", profile.Format{Mark: profile.CodeMark}, policy.Key("primary+e"))
	p.Command("formats.strike", "commands.strike", profile.Format{Mark: profile.Strike})
}, policy.Label("policies.formats"), policy.Description("policies.formats.description"))

// formatAtCaret toggles a mark in the marks text typed at the caret
// takes: the pending ones, or the marks around it.
func formatAtCaret(c *policy.Ctx, in profile.Format, at policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	head := c.Head().Pos
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

// formatRange toggles a mark over a range: cleared where it covers all of it,
// applied otherwise.
func formatRange(c *policy.Ctx, in profile.Format, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	from, to := c.Range()
	spans := vocab.TextSpans(r, from, to)
	if vocab.CoverageOf(r, spans, in.Mark) == vocab.CoverFull {
		return policy.Do(vocab.ClearMark(spans, in.Mark))
	}
	return policy.Do(vocab.ApplyMark(spans, doc.Mark{Kind: in.Mark}))
}
