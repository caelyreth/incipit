// Package formats is the formats policy: a format toggles the marks text
// typed at a caret takes, as pending marks, and the marks of a range.
package formats

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"time"
)

// ID is the policy's id.
const ID = "formats"

// Policy is the formats policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	format := policy.Intent[profile.Format]()
	policy.On(p, "at-caret", format, policy.Slot.Own(), formatAtCaret).When(policy.Rich)
	policy.On(p, "range", format, policy.Slot.Inside(), formatRange).Range().Cells()

	// Marks are offered wherever text is written or cells are selected.
	where := policy.Spans(policy.SpanWriting | policy.SpanCells)
	p.Command("formats.strong", "commands.strong", profile.Format{Mark: profile.Strong}, policy.Key("primary+b"),
		policy.Offer(profile.MarkOffers, 0, "b", where))
	p.Command("formats.emphasis", "commands.emphasis", profile.Format{Mark: profile.Emphasis}, policy.Key("primary+i"),
		policy.Offer(profile.MarkOffers, 1, "i", where))
	p.Command("formats.code", "commands.code", profile.Format{Mark: profile.CodeMark}, policy.Key("primary+e"),
		policy.Offer(profile.MarkOffers, 2, "e", where))
	p.Command("formats.strike", "commands.strike", profile.Format{Mark: profile.Strike},
		policy.Offer(profile.MarkOffers, 3, "s", where))
	policy.State(p, format, marked)

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("Say it "),
		policy.Presses(profile.Format{Mark: profile.Strong}),
		policy.Types("firmly"),
		policy.Presses(profile.Format{Mark: profile.Strong}),
		policy.Types(", then softly."),
	)
}, policy.Label("policies.formats"), policy.Description("policies.formats.description"))

// formatAtCaret toggles a mark in the marks text typed at the caret
// takes: the pending ones, or the marks around it.
func formatAtCaret(c *policy.Ctx, in profile.Format, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	marks := c.TypedMarks()
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

// marked is whether a format is on: in the marks typed at a caret, or
// over all the selection covers, where toggling it clears it.
func marked(c *policy.Ctx, in profile.Format) bool {
	if c.Span().Shape == policy.SpanCaret {
		marks := c.TypedMarks()
		return marks != nil && marks.Has(in.Mark)
	}
	spans := pieceSpans(c)
	return len(spans) > 0 && vocab.CoverageOf(c.Doc(), spans, in.Mark) == vocab.CoverFull
}

// pieceSpans are the text spans of what the selection covers.
func pieceSpans(c *policy.Ctx) []vocab.Span {
	var spans []vocab.Span
	for _, p := range c.Pieces() {
		spans = append(spans, vocab.TextSpans(c.Doc(), p.From, p.To)...)
	}
	return spans
}

// formatRange toggles a mark over what the selection covers: cleared where
// it covers all of it, applied otherwise.
func formatRange(c *policy.Ctx, in profile.Format, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	spans := pieceSpans(c)
	if vocab.CoverageOf(r, spans, in.Mark) == vocab.CoverFull {
		return policy.Do(vocab.ClearMark(spans, in.Mark))
	}
	return policy.Do(vocab.ApplyMark(spans, doc.Mark{Kind: in.Mark}))
}
