// Package pairs is the pairs policy: brackets and quotes typed in text
// close themselves, a closer typed at a pair's end steps over the one
// there, and deleting into a pair, a text pair or a mark between its
// signs, arms both sides before deleting them together. Every balanced
// text pair is a scope Leave exits and Return enters.
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
	// Overtype steps over a pair's closer when it is typed right before
	// it, however the closer came to be there.
	Overtype = policy.Bool("overtype", true, policy.Label("settings.pairs.overtype"), policy.Description("settings.pairs.overtype.description"))
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
	policy.Scan[string](p, scanner{kind: span, kinds: kinds}, []policy.FactID{span.Kind()})
	p.Paint(vocab.GroupPairMatch, span.Kind(), policy.WhenCaretInside)

	inserts := []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted}
	for _, k := range kinds {
		k := k
		// A selection is wrapped in the pair, opener or not auto-closing:
		// the selection stays on what it held.
		p.Handle("wrap-" + k.open).On(policy.Typed(k.open)).In(policy.Slot).When(policy.TextRange).Unless(policy.Collapsed).
			Produces(inserts...).Do(wrap(k))
		if !auto {
			continue
		}
		open := p.Handle("open-"+k.open).On(policy.Typed(k.open)).In(policy.Slot).
			When(policy.Collapsed, policy.OwnText).Unless(policy.NextChar(vocab.ClassWord))
		if k == quote {
			// After a word a straight quote closes; it opens nothing.
			open.Unless(policy.PrevChar(vocab.ClassWord))
		}
		open.Produces(inserts...).Do(openPair(k))
	}
	if over {
		// A closer typed at a pair's inner end is the closer already there:
		// the caret steps over it. What the page holds decides, not who
		// typed the closer, so a hand-typed one steps over as an
		// auto-closed one does, and as a mark's sign does.
		for _, k := range kinds {
			p.Handle("overtype-"+k.close).On(policy.Typed(k.close)).In(span.Scope()).
				When(policy.Collapsed, policy.AtInnerEnd).Do(overtype)
		}
	}
	if mates {
		// Deleting into a pair, its text or its signs, arms both sides
		// first, showing what the next press takes; the next press deletes
		// both and keeps what they held, so a pair never loses one side.
		pairScope := policy.RoleScope(vocab.RolePair)
		back, fwd := policy.Deleting(vocab.ByGrapheme, vocab.Backward), policy.Deleting(vocab.ByGrapheme, vocab.Forward)
		armed := policy.Has(policy.Armed)
		for _, d := range []struct {
			name string
			on   policy.Pattern
			edge policy.Relation
		}{{"inside-start", back, policy.AtInnerStart}, {"inside-end", fwd, policy.AtInnerEnd}} {
			p.Handle("arm-"+d.name).On(d.on).In(pairScope).When(policy.Collapsed, d.edge).Unless(armed).Do(armRole)
			p.Handle("delete-"+d.name).On(d.on).In(pairScope).When(policy.Collapsed, d.edge, armed).
				Produces(sideEdits...).Do(deleteRole)
		}
		// From outside a pair, next to its closing side or its opening one.
		for _, d := range []struct {
			name  string
			on    policy.Pattern
			ahead bool
		}{{"after", back, false}, {"before", fwd, true}} {
			near := policy.After(pairScope)
			if d.ahead {
				near = policy.Before(pairScope)
			}
			p.Handle("arm-"+d.name).On(d.on).In(policy.Slot).When(policy.Collapsed, near).Unless(armed).Do(arm(edged(d.ahead)))
			p.Handle("delete-"+d.name).On(d.on).In(policy.Slot).When(policy.Collapsed, near, armed).
				Produces(sideEdits...).Do(deleteSides(edged(d.ahead)))
		}
		p.Annotation(policy.Armed, nil)
		p.Decorate(policy.Armed, vocab.GroupArmed)
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

func (p pair) Sides() (open, close vocab.Edge) {
	at := func(off int) doc.Caret { return doc.CaretAt(doc.At(p.slot, off)) }
	return vocab.Edge{From: at(p.from), To: at(p.innerFrom)}, vocab.Edge{From: at(p.innerTo), To: at(p.to)}
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
			items = []doc.Item{doc.TextItem(text, vocab.InheritedMarks(tx.Schema(), n.Slot(s.Slot.Slot).Content(), s.From))}
		}
		return tx.Apply(doc.EditText{At: s.Slot, From: s.From, To: s.To, Items: items})
	}
}

func openPair(k kind) policy.Handler {
	return func(c *policy.Ctx, _ policy.Match) policy.Result {
		at := c.Head().Pos
		inner := doc.At(at.Ref(), at.Offset+len(k.open))
		res := policy.Do(replace(vocab.Span{Slot: at.Ref(), From: at.Offset, To: at.Offset}, k.open+k.close))
		res = res.With(doc.Collapsed(doc.CaretAt(inner)))
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

// overtype steps over the pair's closer.
func overtype(c *policy.Ctx, m policy.Match) policy.Result {
	f, _ := m.Fact()
	return policy.Select(doc.Collapsed(doc.CaretAt(doc.At(m.Level.Slot, f.To))))
}

// sideEdits are what deleting a pair's sides records: text for a text
// pair, marks for a mark's signs.
var sideEdits = []doc.RecordKind{doc.RecordTextDeleted, doc.RecordMarks}

// pairOf finds the pair a rule acts on.
type pairOf func(c *policy.Ctx, m policy.Match) (vocab.Pair, bool)

func rolePair(_ *policy.Ctx, m policy.Match) (vocab.Pair, bool) {
	pr, ok := m.Role.(vocab.Pair)
	return pr, ok
}

// edged is the pair right after the caret (ahead) or right before it, as
// Before and After match it: of any policy, its sides text or signs.
func edged(ahead bool) pairOf {
	return func(c *policy.Ctx, _ policy.Match) (vocab.Pair, bool) {
		data, ok := c.Beside(policy.RoleScope(vocab.RolePair), ahead)
		pr, pok := data.(vocab.Pair)
		return pr, ok && pok
	}
}

var (
	armRole    = arm(rolePair)
	deleteRole = deleteSides(rolePair)
)

// arm marks both sides of the pair for the next press, which is the only
// intent that sees them.
func arm(of pairOf) policy.Handler {
	return func(c *policy.Ctx, m policy.Match) policy.Result {
		pr, ok := of(c, m)
		if !ok {
			return policy.Swallow
		}
		res := policy.Result{}
		open, close := pr.Sides()
		for _, side := range []vocab.Edge{open, close} {
			res.Annotate = append(res.Annotate, policy.Annotate{Kind: policy.Armed,
				Anchor: policy.Anchor{From: side.From, To: side.To}, Lifetime: policy.UntilNextIntent})
		}
		return res
	}
}

// deleteSides deletes both sides of the pair, the closer first, so the
// opener's place stands, and keeps what they held.
func deleteSides(of pairOf) policy.Handler {
	return func(c *policy.Ctx, m policy.Match) policy.Result {
		pr, ok := of(c, m)
		if !ok {
			return policy.Swallow
		}
		res := policy.Do(pr.EditSide(vocab.Close, ""), pr.EditSide(vocab.Open, ""))
		res.DropKinds = []policy.AnnotationKind{policy.Armed}
		return res
	}
}
