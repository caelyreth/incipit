// Package tags is the tags policy: a "#" where a word starts, a letter
// after it, writes a tag (profile.TagsIn), and the tag reads as one
// wherever the caret is. A tag is its text and nothing else, so it is
// typed, deleted, copied and undone as text; a sheet's tags are the tags
// its text writes. A picker names the tag the caret is in with PickTag.
package tags

import (
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "tags"

// tag is a tag fact's data: the tag's name.
type tag struct{ Name string }

// Policy is the tags policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	tags := policy.Facts[tag](p, "tag", policy.Styles(profile.GroupTag))
	// Tags are read line by line, so a keystroke reads its own line again
	// and every other line's tags come from the store.
	p.Analyze(policy.Lines, analyze(tags), []policy.FactID{tags.Kind()},
		policy.Where(func(u *policy.Unit) bool { return !u.Spec.Literal && u.Spec.Marks != nil }))
	policy.On(p, "pick", policy.Intent[profile.PickTag](), tags.Inside(), pick)

	// The card's demonstration, played by the engine itself.
	p.Showcase(2600*time.Millisecond,
		policy.Types("Notes on tides #ocean"),
		policy.Waits(500*time.Millisecond),
		policy.Types(" and #moon phases."),
	)
}, policy.Label("policies.tags"), policy.Description("policies.tags.description"))

func analyze(tags *policy.FactKind[tag]) func(u *policy.Unit, e *policy.Emit) {
	return func(u *policy.Unit, e *policy.Emit) {
		for _, t := range profile.TagsIn(u.Content, u.From, u.To) {
			e.Fact(tags.Span(t.From, t.To, tag{Name: t.Name}))
		}
	}
}

// pick writes the picked name over the tag's and ends its word: the caret
// goes past the space after it, which it adds where none follows.
func pick(c *policy.Ctx, in profile.PickTag, at policy.At[tag]) policy.Result {
	if !profile.TagNamed(in.Name) {
		return policy.Pass
	}
	ref := at.Slot
	n, ok := c.Doc().Node(ref.Node)
	if !ok {
		return policy.Pass
	}
	content := n.Slot(ref.Slot).Content()
	from, to := at.Fact.Outer.From.At, at.Fact.Outer.To.At
	text := string(profile.TagSpelling) + in.Name
	if content.FlatRange(to, min(to+1, content.Len())) != " " {
		text += " "
	}
	var marks *doc.MarkSet
	if it, _, ok := content.ItemAt(from); ok {
		marks = it.Marks
	}
	end := from + len(text)
	if len(text) == len(profile.TagSpelling)+len(in.Name) {
		end++
	}
	res := policy.Do(vocab.Ops(doc.EditText{At: ref, From: from, To: to, Items: []doc.Item{doc.TextItem(text, marks)}}))
	res.Then = func(doc.Reader) doc.Selection { return doc.Collapsed(doc.CaretAt(doc.At(ref, end))) }
	return res
}
