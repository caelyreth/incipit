// Package find is the find policy: a sheet's find. A query in the query
// language (internal/text/query) is the view's mode; the policy's analyzer
// reads it, so every match in the page is a fact that paints the text it
// spans, line by line as the text changes. Seek sets the query and
// selects the first match from the selection on; Step goes to the next
// match or the one before, round the page. The match the selection goes
// to flashes, and is shown.
//
// What the content holds is never remembered: the matches are read from
// the text and the query, so an edit that makes or breaks one shows at
// once, and undo brings back what it took.
package find

import (
	"strings"
	"sync"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/caelyreth/incipit/internal/text/query"
)

// ID is the policy's id.
const ID = "find"

// FlashFor is how long the match the selection goes to flashes.
const FlashFor = 700 * time.Millisecond

var (
	intentSeek = vocab.NewIntentKind("Find")
	intentStep = vocab.NewIntentKind("FindStep")
)

// Seek finds Query in the page: its matches show, and the first from the
// selection's start on is selected, round the page. An empty query ends
// the find, and the matches go.
type Seek struct{ Query string }

// Step selects the next match (Forward) or the one before (Backward), round
// the page's ends.
type Step struct{ Dir vocab.Direction }

func (Seek) Kind() vocab.IntentKind { return intentSeek }
func (Step) Kind() vocab.IntentKind { return intentStep }

// Flash marks the match the selection went to.
var Flash = policy.NewAnnotationKind("find.flash", false)

// mode is the find's mode for a view: the query as written.
type mode struct{ query string }

// match is a match fact's data.
type match struct{}

// Match is the kind of the find's matches. It is assigned when the policy
// is built.
var Match *policy.FactKind[match]

// Policy is the find policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	Match = policy.Facts[match](p, "match", policy.Styles(vocab.GroupFindMatch))
	// Matches are read line by line, so a keystroke reads its own line
	// again and every other line's come from the store. A formula's source
	// is not read: the page shows the formula, not its source.
	p.Analyze(policy.Lines, analyze, []policy.FactID{Match.Kind()},
		policy.Where(func(u *policy.Unit) bool { return u.Mode != nil && !formula(u.Owner) }))
	p.Annotation(Flash, func(doc.Reader, policy.Annotation) bool { return true })
	p.Decorate(Flash, vocab.GroupFindFlash)
	policy.On(p, "seek", policy.Intent[Seek](), policy.Slot.Inside(), seek).Any()
	policy.On(p, "step", policy.Intent[Step](), policy.Slot.Inside(), step).Any()
	p.Command("find.next", "commands.findNext", Step{Dir: vocab.Forward}, policy.Key("primary+g"))
	p.Command("find.previous", "commands.findPrevious", Step{Dir: vocab.Backward}, policy.Key("primary+shift+g"))

	// The card's demonstration, played by the engine itself.
	p.Showcase(2600*time.Millisecond,
		policy.Types("Tides rise, tides turn, tides fall."),
		policy.Waits(400*time.Millisecond),
		policy.Presses(Seek{Query: "tides"}),
		policy.Waits(600*time.Millisecond),
		policy.Presses(Step{Dir: vocab.Forward}),
		policy.Waits(600*time.Millisecond),
		policy.Presses(Step{Dir: vocab.Forward}),
	)
}, policy.Label("policies.find"), policy.Description("policies.find.description"))

// formula reports a node whose text is a formula's source.
func formula(n *doc.Node) bool { return n.Kind() == profile.Math || n.Kind() == profile.Formula }

// matchers keeps each query's matcher, read once, for the units that read
// it.
var matchers sync.Map

// matcherOf is the matcher of a query as a sheet's find reads it.
func matcherOf(text string) *query.Matcher {
	if m, ok := matchers.Load(text); ok {
		return m.(*query.Matcher)
	}
	m := query.NewMatcher(query.Parse(text), false)
	matchers.Store(text, m)
	return m
}

func analyze(u *policy.Unit, e *policy.Emit) {
	m, ok := u.Mode.(mode)
	if !ok {
		return
	}
	for _, s := range spans(u.Owner, u.Enclosing, u.Content, u.From, u.To, matcherOf(m.query)) {
		e.Fact(Match.Span(s.From, s.To, match{}))
	}
}

// spans are the matches in content from from to to, at slot offsets: a
// match of a key's words only in the text the key names.
func spans(owner *doc.Node, enclosing []doc.Kind, content *doc.Inline, from, to int, m *query.Matcher) []query.Span {
	var out []query.Span
	for _, h := range m.Hits(content.FlatRange(from, to)) {
		s := query.Span{From: from + h.From, To: from + h.To}
		if !in(h.Key, owner, enclosing, content, s) {
			continue
		}
		if n := len(out); n > 0 && s.From <= out[n-1].To {
			out[n-1].To = max(out[n-1].To, s.To)
			continue
		}
		out = append(out, s)
	}
	return out
}

// in reports whether a match of a key's words stands in the text the key
// names: a heading, code (a code block, or text marked as code), a quote.
func in(key string, owner *doc.Node, enclosing []doc.Kind, content *doc.Inline, s query.Span) bool {
	switch key {
	case "":
		return true
	case "heading":
		return owner.Kind() == profile.Heading
	case "code":
		if owner.Kind() == profile.Code {
			return true
		}
		for at := s.From; at < s.To; at++ {
			if !content.MarksAt(at).Has(profile.CodeMark) {
				return false
			}
		}
		return true
	case "quote":
		for _, k := range enclosing {
			if k == profile.Quote {
				return true
			}
		}
		return owner.Kind() == profile.Quote
	}
	return false
}

// Spot is a match in a page.
type Spot struct {
	Slot     doc.SlotRef
	From, To int
}

// Matches are the view's matches in the page, in reading order, as its
// facts hold them: none when the view finds nothing.
func Matches(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View) []Spot {
	if view == nil || view.Modes.Of(ID) == nil || Match == nil {
		return nil
	}
	var out []Spot
	eachSlot(r, func(ref doc.SlotRef) {
		for _, f := range prog.FactsIn(store, r, view, ref, Match.Kind()) {
			out = append(out, Spot{Slot: ref, From: f.Outer.From.At, To: f.Outer.To.At})
		}
	})
	return out
}

// eachSlot visits the page's inline slots in reading order, but formulas'.
func eachSlot(r doc.Reader, fn func(ref doc.SlotRef)) {
	r.Root().Walk(func(n *doc.Node) bool {
		if formula(n) {
			return false
		}
		for i, sl := range n.Slots() {
			if sl.IsInline() {
				fn(doc.SlotRef{Node: n.ID(), Slot: i})
			}
		}
		return true
	})
}

// found are the matches of a query in the page, read now: a Seek reads
// them before its query is the view's.
func found(r doc.Reader, text string) []Spot {
	m := matcherOf(text)
	if m.Empty() {
		return nil
	}
	var out []Spot
	eachSlot(r, func(ref doc.SlotRef) {
		n, _ := r.Node(ref.Node)
		content := n.Slot(ref.Slot).Content()
		var enclosing []doc.Kind
		for _, id := range doc.Ancestors(r, ref.Node) {
			if a, ok := r.Node(id); ok {
				enclosing = append(enclosing, a.Kind())
			}
		}
		for _, s := range spans(n, enclosing, content, 0, content.Len(), m) {
			out = append(out, Spot{Slot: ref, From: s.From, To: s.To})
		}
	})
	return out
}

func seek(c *policy.Ctx, in Seek, _ policy.At[doc.SlotRef]) policy.Result {
	if strings.TrimSpace(in.Query) == "" {
		if c.Mode(ID) == nil {
			return policy.Swallow
		}
		return policy.Result{DropMode: true, DropKinds: []policy.AnnotationKind{Flash}}
	}
	res := policy.Result{Mode: mode{query: in.Query}}
	all := found(c.Doc(), in.Query)
	from, _ := ends(c)
	at := next(c.Doc(), all, from)
	if at < 0 {
		return res
	}
	return goTo(c, res, all[at])
}

func step(c *policy.Ctx, in Step, _ policy.At[doc.SlotRef]) policy.Result {
	if c.Mode(ID) == nil {
		return policy.Pass
	}
	all := Matches(c.Program(), c.Store(), c.Doc(), c.View())
	from, to := ends(c)
	at := next(c.Doc(), all, to)
	if in.Dir == vocab.Backward {
		at = previous(c.Doc(), all, from)
	}
	if at < 0 {
		return policy.Result{Effects: []policy.Effect{{Kind: policy.EffectBeep}}}
	}
	return goTo(c, policy.Result{}, all[at])
}

// goTo selects a match, shows it and flashes it.
func goTo(c *policy.Ctx, res policy.Result, s Spot) policy.Result {
	sel := doc.TextRange(doc.CaretAt(doc.At(s.Slot, s.From)), doc.CaretAt(doc.At(s.Slot, s.To)))
	res.Selection = &sel
	res.Effects = append(res.Effects, policy.Effect{Kind: policy.EffectReveal})
	res.DropKinds = append(res.DropKinds, Flash)
	res.Annotate = append(res.Annotate, policy.Annotate{
		Kind: Flash, Anchor: policy.Between(doc.At(s.Slot, s.From), doc.At(s.Slot, s.To)),
		Data: flash{}, Lifetime: policy.Until(c.Now().Add(FlashFor)),
	})
	return res
}

// flash tells the presenter how to play the flash: once, forward.
type flash struct{}

// DecorationMotion is the direction the flash plays in.
func (flash) DecorationMotion() vocab.Direction { return vocab.Forward }

// DecorationDuration is how long the flash plays.
func (flash) DecorationDuration() time.Duration { return FlashFor }

// ends are where the selection starts and ends: a text selection's ends
// in order, or where the head stands.
func ends(c *policy.Ctx) (doc.Caret, doc.Caret) {
	if sel := c.Selection(); sel.Kind == doc.SelectText {
		return sel.Ordered(c.Doc())
	}
	return c.Head(), c.Head()
}

// next is the first match starting at or after c, round to the first; -1
// for none.
func next(r doc.Reader, all []Spot, c doc.Caret) int {
	if len(all) == 0 {
		return -1
	}
	for i, s := range all {
		if doc.Compare(r, doc.CaretAt(doc.At(s.Slot, s.From)), c) >= 0 {
			return i
		}
	}
	return 0
}

// previous is the last match starting before c, round to the last; -1 for
// none.
func previous(r doc.Reader, all []Spot, c doc.Caret) int {
	if len(all) == 0 {
		return -1
	}
	for i := len(all) - 1; i >= 0; i-- {
		if doc.Compare(r, doc.CaretAt(doc.At(all[i].Slot, all[i].From)), c) < 0 {
			return i
		}
	}
	return len(all) - 1
}
