// Package pairs is the pairs policy: brackets and quotes typed in text
// close themselves, a closer it typed ahead is stepped over, deleting into
// a pair selects it whole first, and every balanced pair is a scope Leave
// exits and Return re-enters.
package pairs

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "pairs"

// Options.
var (
	// AutoClose types a pair's closer with its opener.
	AutoClose = policy.Bool("autoClose", true, policy.Label("settings.pairs.autoClose"), policy.Description("settings.pairs.autoClose.description"))
	// Overtype steps over a closer typed ahead when it is typed.
	Overtype = policy.Bool("overtype", true, policy.Label("settings.pairs.overtype"), policy.Description("settings.pairs.overtype.description"),
		policy.Requires("autoClose", true))
	// Quotes pairs straight double quotes.
	Quotes = policy.Bool("quotes", true, policy.Label("settings.pairs.quotes"), policy.Description("settings.pairs.quotes.description"))
	// Mates deletes an empty pair's two sides together.
	Mates = policy.Bool("mates", true, policy.Label("settings.pairs.mates"), policy.Description("settings.pairs.mates.description"))
)

// kind is a pair's two spellings.
type kind struct{ open, close string }

var (
	brackets = []kind{{"(", ")"}, {"[", "]"}, {"{", "}"}, {"“", "”"}, {"「", "」"}, {"（", "）"}}
	quote    = kind{`"`, `"`}
)

// Policy is the pairs policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	auto, over := policy.Get(p, AutoClose), policy.Get(p, Overtype)
	quotes, mates := policy.Get(p, Quotes), policy.Get(p, Mates)
	kinds := append([]kind(nil), brackets...)
	if quotes {
		kinds = append(kinds, quote)
	}

	span := policy.Facts[pair](p, "pair", policy.Provides(vocab.RolePair, vocab.RoleLeavable))
	policy.Scan[string](p, scanner{kind: span, kinds: kinds}, []vocab.FactKind{span.Kind()})

	p.Annotation(vocab.TypedAhead, func(r doc.Reader, a policy.Annotation) bool {
		closer, ok := a.Data.(string)
		if !ok {
			return false
		}
		n, ok := r.Node(a.Anchor.From.Node)
		if !ok || a.Anchor.From.Slot >= n.NumSlots() || !n.Slot(a.Anchor.From.Slot).IsInline() {
			return false
		}
		return n.Slot(a.Anchor.From.Slot).Content().FlatRange(a.Anchor.From.Offset, a.Anchor.To.Offset) == closer
	})

	inserts := []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted}
	for _, k := range kinds {
		k := k
		// A selection is wrapped in the pair, opener or not auto-closing:
		// the selection stays on what it held.
		p.Handle("wrap-" + k.open).On(vocab.Typed(k.open)).In(vocab.Slot).When(vocab.TextRange).Unless(vocab.Collapsed).
			Produces(inserts...).Do(wrap(k))
		if !auto {
			continue
		}
		open := p.Handle("open-"+k.open).On(vocab.Typed(k.open)).In(vocab.Slot).
			When(vocab.Collapsed, vocab.OwnText).Unless(vocab.NextChar(vocab.ClassWord))
		if k == quote {
			// After a word a straight quote closes; it opens nothing.
			open.Unless(vocab.PrevChar(vocab.ClassWord))
		}
		open.Produces(inserts...).Do(openPair(k))
	}
	if auto && over {
		for _, k := range kinds {
			p.Handle("overtype-"+k.close).On(vocab.Typed(k.close)).In(span.Scope()).
				When(vocab.Collapsed, vocab.AtInnerEnd, vocab.Has(vocab.TypedAhead)).Do(overtype)
		}
	}
	if mates {
		// Deleting into a pair from either side selects it whole, both
		// sides and what they hold; the next press deletes the selection,
		// so a pair never loses one side alone.
		back, fwd := vocab.Deleting(vocab.ByGrapheme, vocab.Backward), vocab.Deleting(vocab.ByGrapheme, vocab.Forward)
		p.Handle("select-from-inside").On(back).In(span.Scope()).When(vocab.Collapsed, vocab.AtInnerStart).Do(selectPair)
		p.Handle("select-from-inside-end").On(fwd).In(span.Scope()).When(vocab.Collapsed, vocab.AtInnerEnd).Do(selectPair)
		p.Handle("select-after").On(back).In(vocab.Slot).When(vocab.Collapsed, vocab.After(span.Scope())).Do(selectAdjacent(span, false))
		p.Handle("select-before").On(fwd).In(vocab.Slot).When(vocab.Collapsed, vocab.Before(span.Scope())).Do(selectAdjacent(span, true))
	}
})

// --- recognising ---------------------------------------------------------------

// scanner finds balanced pairs in a slot. Its state is the stack of open
// sides, each "kind@offset;", so it stays comparable.
type scanner struct {
	kind  *policy.FactKind[pair]
	kinds []kind
}

func (s scanner) Start(*policy.Unit) string { return "" }

func (s scanner) Step(st string, t policy.Token, emit func(policy.Fact)) string {
	if t.Kind != policy.TokenText {
		return st
	}
	for i := 0; i < len(t.Text); {
		k, side, n := s.at(t.Text[i:], st)
		switch side {
		case vocab.Open:
			st += strconv.Itoa(k) + "@" + strconv.Itoa(t.At+i) + ";"
		case vocab.Close:
			st = s.close(st, k, t.At+i, emit)
		}
		i += max(n, 1)
	}
	return st
}

// at reads a side at the start of text: which kind, which side and its
// length. A symmetric quote closes when its kind is open innermost.
func (s scanner) at(text, st string) (int, vocab.Side, int) {
	for i, k := range s.kinds {
		switch {
		case k.open == k.close && strings.HasPrefix(text, k.open):
			if top, ok := innermost(st); ok && top.kind == i {
				return i, vocab.Close, len(k.close)
			}
			return i, vocab.Open, len(k.open)
		case strings.HasPrefix(text, k.open):
			return i, vocab.Open, len(k.open)
		case strings.HasPrefix(text, k.close):
			return i, vocab.Close, len(k.close)
		}
	}
	return 0, 0, 0
}

type opened struct{ kind, at int }

func entries(st string) []opened {
	var out []opened
	for _, e := range strings.Split(strings.TrimSuffix(st, ";"), ";") {
		if e == "" {
			continue
		}
		k, at, _ := strings.Cut(e, "@")
		ki, _ := strconv.Atoi(k)
		ai, _ := strconv.Atoi(at)
		out = append(out, opened{kind: ki, at: ai})
	}
	return out
}

func innermost(st string) (opened, bool) {
	es := entries(st)
	if len(es) == 0 {
		return opened{}, false
	}
	return es[len(es)-1], true
}

// close pairs a closer with the innermost open side of its kind; open
// sides inside it are left unclosed. A closer nothing opened is text.
func (s scanner) close(st string, k, at int, emit func(policy.Fact)) string {
	es := entries(st)
	for j := len(es) - 1; j >= 0; j-- {
		if es[j].kind != k {
			continue
		}
		kd := s.kinds[k]
		from := es[j].at
		emit(s.kind.Delimited(from, from+len(kd.open), at, at+len(kd.close), pair{Open: kd.open, Close: kd.close}))
		var b strings.Builder
		for _, e := range es[:j] {
			b.WriteString(strconv.Itoa(e.kind) + "@" + strconv.Itoa(e.at) + ";")
		}
		return b.String()
	}
	return st
}

// --- the pair ------------------------------------------------------------------

// pair is a balanced pair's spellings; Placed binds it to where it stands.
type pair struct {
	Open, Close string

	slot                         doc.SlotRef
	from, innerFrom, innerTo, to int
}

func (p pair) Placed(slot doc.SlotRef, f policy.Fact) any {
	p.slot, p.from, p.innerFrom, p.innerTo, p.to = slot, f.From, f.InnerFrom, f.InnerTo, f.To
	return p
}

func (p pair) Sides() (open, close vocab.Span) {
	return vocab.Span{Slot: p.slot, From: p.from, To: p.innerFrom}, vocab.Span{Slot: p.slot, From: p.innerTo, To: p.to}
}

// EditSide respells one side as text.
func (p pair) EditSide(side vocab.Side, spelling vocab.Spelling) vocab.Edit {
	s := vocab.Span{Slot: p.slot, From: p.from, To: p.innerFrom}
	if side == vocab.Close {
		s = vocab.Span{Slot: p.slot, From: p.innerTo, To: p.to}
	}
	return replace(s, string(spelling))
}

// Exit is after the closer; Entry is before it, where the caret was.
func (p pair) Exit() doc.Caret  { return doc.CaretAt(doc.At(p.slot, p.to)) }
func (p pair) Entry() doc.Caret { return doc.CaretAt(doc.At(p.slot, p.innerTo)) }

// --- rules -------------------------------------------------------------------

// replace puts text over a span, in the marks the text before it has.
func replace(s vocab.Span, text string) vocab.Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(s.Slot.Node)
		if !ok {
			return doc.ErrNoNode
		}
		var items []doc.Item
		if text != "" {
			items = []doc.Item{doc.TextItem(text, vocab.InheritedMarks(n.Slot(s.Slot.Slot).Content(), s.From))}
		}
		return tx.Apply(doc.EditText{At: s.Slot, From: s.From, To: s.To, Items: items})
	}
}

func openPair(k kind) policy.Handler {
	return func(c *policy.Ctx, _ policy.Match) policy.Result {
		at := c.Head().Pos
		inner := doc.At(at.Ref(), at.Offset+len(k.open))
		whole := policy.Anchor{From: at, To: doc.At(at.Ref(), inner.Offset+len(k.close))}
		res := policy.Do(replace(vocab.Span{Slot: at.Ref(), From: at.Offset, To: at.Offset}, k.open+k.close))
		res = res.With(doc.Collapsed(doc.CaretAt(inner)))
		res.Annotate = []policy.Annotate{{Kind: vocab.TypedAhead, Data: k.close,
			Anchor:   policy.Anchor{From: inner, To: whole.To},
			Lifetime: policy.UntilCaretLeaves(&whole)}}
		res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
		return res
	}
}

func wrap(k kind) policy.Handler {
	return func(c *policy.Ctx, _ policy.Match) policy.Result {
		from, to := c.Range()
		if from.Ref() != to.Ref() || !vocab.IsText(c.Doc(), from) {
			// Across blocks there is no one text to wrap: the opener
			// replaces the selection, as typing does.
			return policy.Do(vocab.DeleteRange(from, to), replace(vocab.Span{Slot: from.Ref(), From: from.Offset, To: from.Offset}, k.open)).
				With(doc.Collapsed(doc.CaretAt(doc.At(from.Ref(), from.Offset+len(k.open)))))
		}
		ref := from.Ref()
		res := policy.Do(
			replace(vocab.Span{Slot: ref, From: to.Offset, To: to.Offset}, k.close),
			replace(vocab.Span{Slot: ref, From: from.Offset, To: from.Offset}, k.open),
		)
		start := doc.CaretAt(doc.At(ref, from.Offset+len(k.open)))
		end := doc.CaretAt(doc.At(ref, to.Offset+len(k.open)))
		return res.With(doc.Selection{Kind: doc.SelectText, Anchor: start, Head: end})
	}
}

// overtype steps over the closer typed ahead.
func overtype(c *policy.Ctx, m policy.Match) policy.Result {
	f, _ := m.Fact()
	res := policy.Select(doc.Collapsed(doc.CaretAt(doc.At(m.Level.Slot, f.To))))
	for _, a := range c.Annotations().At(vocab.TypedAhead, c.Head().Pos) {
		res.Drop = append(res.Drop, a.ID)
	}
	return res
}

// selectPair selects the pair the caret is in, whole.
func selectPair(_ *policy.Ctx, m policy.Match) policy.Result {
	f, _ := m.Fact()
	return selectSpan(m.Level.Slot, f)
}

// selectAdjacent selects the pair right after the caret (ahead) or right
// before it.
func selectAdjacent(span *policy.FactKind[pair], ahead bool) policy.Handler {
	return func(c *policy.Ctx, _ policy.Match) policy.Result {
		head := c.Head().Pos
		var found *policy.Fact
		for _, f := range c.Facts(head.Ref(), span.Kind()) {
			if ahead && f.From == head.Offset || !ahead && f.To == head.Offset {
				// The outermost pair edged there: the last one found.
				f := f
				found = &f
			}
		}
		if found == nil {
			return policy.Swallow
		}
		return selectSpan(head.Ref(), *found)
	}
}

func selectSpan(ref doc.SlotRef, f policy.Fact) policy.Result {
	return policy.Select(doc.TextRange(doc.CaretAt(doc.At(ref, f.From)), doc.CaretAt(doc.At(ref, f.To))))
}
