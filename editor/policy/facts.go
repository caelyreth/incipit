package policy

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Fact is a recognised span or point in one inline slot. Offsets are
// slot-relative bytes. A span has an outer extent and an inner one (a pair
// without its delimiters); a point has From == To.
//
// Each edge is a place: an offset and a stop there (§13.5), so a fact
// whose delimiters are projected, spanning stops but no bytes, has edges
// like any other. An edge at content holds every stop at its offset
// (doc.StopFirst or doc.StopLast); a point's outer edges are the stops
// just before and after its projection, and its inner extent the stops
// between.
type Fact struct {
	Kind                       FactID
	From, To                   int
	InnerFrom, InnerTo         int
	StopFrom, StopTo           int
	InnerStopFrom, InnerStopTo int
	Point                      bool
	Data                       any
}

// StopRange is where a projected delimiter stands among the stops at its
// offset: from the stop just before it to the stop just after it.
type StopRange struct{ From, To int }

// place orders a caret against an edge at offset at and stop stop.
func place(c doc.Caret, at, stop int) int {
	if c.Pos.Offset != at {
		return cmp.Compare(c.Pos.Offset, at)
	}
	return cmp.Compare(c.Stop, stop)
}

// Holds reports whether a caret is inside the fact's inner extent.
func (f Fact) Holds(c doc.Caret) bool {
	return place(c, f.InnerFrom, f.InnerStopFrom) >= 0 && place(c, f.InnerTo, f.InnerStopTo) <= 0
}

// Starts reports whether the fact starts right after the caret, nothing
// between them; Ends whether it ends right before it.
func (f Fact) Starts(c doc.Caret) bool {
	return c.Pos.Offset == f.From && c.Stop <= f.StopFrom
}

func (f Fact) Ends(c doc.Caret) bool {
	return c.Pos.Offset == f.To && c.Stop >= f.StopTo
}

// FactKind is a typed fact kind a policy declared.
type FactKind[T any] struct {
	kind   FactID
	name   string
	policy string
	roles  []vocab.Role
}

// FactOption refines a fact kind's declaration.
type FactOption func(*factDecl)

type factDecl struct {
	kind   FactID
	name   string
	policy string
	roles  []vocab.Role
	data   reflect.Type
}

// Provides declares the roles the fact kind's data provides. The data type
// must implement each role's interface.
func Provides(roles ...vocab.Role) FactOption {
	return func(d *factDecl) { d.roles = append(d.roles, roles...) }
}

var factKinds = struct {
	sync.Mutex
	of map[string]FactID
}{of: map[string]FactID{}}

// stableFactKind returns one handle per policy and name, so a program
// rebuilt after a settings change keeps its fact kinds and their caches.
func stableFactKind(policy, name string) FactID {
	factKinds.Lock()
	defer factKinds.Unlock()
	key := policy + "." + name
	if k, ok := factKinds.of[key]; ok {
		return k
	}
	k := newFactID(key)
	factKinds.of[key] = k
	return k
}

// Facts declares a fact kind whose data is T.
func Facts[T any](p *P, name string, opts ...FactOption) *FactKind[T] {
	d := &factDecl{kind: stableFactKind(p.policy.id, name), name: name, policy: p.policy.id, data: reflect.TypeFor[T]()}
	for _, o := range opts {
		o(d)
	}
	for _, r := range d.roles {
		if iface := roleInterface(r); iface != nil && !d.data.Implements(iface) {
			p.fail("fact %s provides %s, but %s does not implement it", name, r, d.data)
		}
	}
	p.facts = append(p.facts, d)
	return &FactKind[T]{kind: d.kind, name: name, policy: p.policy.id, roles: d.roles}
}

// Kind is the untyped handle.
func (k *FactKind[T]) Kind() FactID { return k.kind }

// Scope is the fact kind as a scope key.
func (k *FactKind[T]) Scope() Scope { return FactScope(k.kind) }

// Span is a fact whose inner extent is its outer one, edged by content.
func (k *FactKind[T]) Span(from, to int, data T) Fact {
	return k.Delimited(from, from, to, to, data)
}

// Delimited is a fact with delimiters: [from, innerFrom) and [innerTo, to).
func (k *FactKind[T]) Delimited(from, innerFrom, innerTo, to int, data T) Fact {
	return Fact{Kind: k.kind, From: from, To: to, InnerFrom: innerFrom, InnerTo: innerTo,
		StopFrom: doc.StopLast, StopTo: doc.StopFirst, InnerStopFrom: doc.StopFirst, InnerStopTo: doc.StopLast, Data: data}
}

// Projected is a span whose delimiters are projected, standing at its
// offsets by stop: open at from, close at to.
func (k *FactKind[T]) Projected(from, to int, open, close StopRange, data T) Fact {
	return Fact{Kind: k.kind, From: from, To: to, InnerFrom: from, InnerTo: to,
		StopFrom: open.From, StopTo: close.To, InnerStopFrom: open.To, InnerStopTo: close.From, Data: data}
}

// Point is a fact at an offset whose projection stands between stop
// stopFrom, just before it, and stop stopTo, just after it; the stops
// between are inside it.
func (k *FactKind[T]) Point(at, stopFrom, stopTo int, data T) Fact {
	return Fact{Kind: k.kind, From: at, To: at, InnerFrom: at, InnerTo: at,
		StopFrom: stopFrom, StopTo: stopTo, InnerStopFrom: stopFrom + 1, InnerStopTo: stopTo - 1, Point: true, Data: data}
}

// Data returns a fact's typed data.
func (k *FactKind[T]) Data(f Fact) (T, bool) {
	if f.Kind != k.kind {
		var zero T
		return zero, false
	}
	v, ok := f.Data.(T)
	return v, ok
}

// Placer is fact data whose role implementations depend on where the fact
// stands. Facts are cached by content, at unit-relative offsets and with
// no slot, so the scope stack and handlers place the data before using
// it: Placed returns the data bound to the fact's slot and offsets.
type Placer interface {
	Placed(slot doc.SlotRef, f Fact) any
}

// Placed returns f's data bound to its slot, for data that implements
// Placer, or the data as it is.
func Placed(slot doc.SlotRef, f Fact) any {
	if p, ok := f.Data.(Placer); ok {
		return p.Placed(slot, f)
	}
	return f.Data
}

func roleInterface(r vocab.Role) reflect.Type {
	switch r {
	case vocab.RolePair:
		return reflect.TypeFor[vocab.Pair]()
	case vocab.RoleLeveled:
		return reflect.TypeFor[vocab.Leveled]()
	case vocab.RoleLeavable:
		return reflect.TypeFor[vocab.Leavable]()
	case vocab.RoleStructure:
		return reflect.TypeFor[vocab.Structure]()
	}
	return nil
}

// --- units --------------------------------------------------------------

// Granularity is the unit a per-unit analyzer reads.
type Granularity uint8

const (
	// Lines reads each line of an inline slot.
	Lines Granularity = iota + 1
	// Slots reads a whole inline slot.
	Slots
)

// Unit is what an analyzer reads: a line or a whole inline slot, with the
// slot's kind and enclosing kinds and the annotations the analyzer's own
// policy owns there.
type Unit struct {
	Slot doc.SlotRef
	// Owner is the node holding the slot.
	Owner *doc.Node
	Spec  doc.SlotSpec
	// Enclosing are the kinds of the owner's ancestors, outermost first.
	Enclosing []doc.Kind
	Content   *doc.Inline
	// From and To are the unit's range in the slot; Line is the line's
	// index, or -1 for a whole slot.
	From, To int
	Line     int
	// Annotations are the analyzing policy's annotations anchored here.
	Annotations []Annotation
	// Context is what the slot's place in the structure adds to its
	// content, as the profile reports it (vocab.Profile.Context).
	Context any

	facts func(FactID) []Fact
}

// Text is the unit's text aligned with slot offsets: an inline node is
// doc.NodeByte, one byte as it is one position.
func (u *Unit) Text() string { return u.Content.FlatRange(u.From, u.To) }

// Facts returns facts of another kind in the unit's slot, for
// presentation contributions that read facts.
func (u *Unit) Facts(k FactID) []Fact {
	if u.facts == nil {
		return nil
	}
	return u.facts(k)
}

// TokenKind is the kind of a unit token.
type TokenKind uint8

const (
	TokenText TokenKind = iota + 1
	TokenNode
	TokenMarkOpen
	TokenMarkClose
	// TokenEnd ends the stream, so a scanner can emit what is still open.
	TokenEnd
)

// Token is one item of a unit's token stream: text, an inline node, or a
// mark boundary. Mark tokens have zero width.
type Token struct {
	Kind TokenKind
	At   int
	Text string
	Node *doc.Node
	Mark doc.Mark
}

// End is the offset after the token.
func (t Token) End() int {
	switch t.Kind {
	case TokenText:
		return t.At + len(t.Text)
	case TokenNode:
		return t.At + 1
	}
	return t.At
}

// Tokens streams the unit's tokens in order. Mark closes come before
// opens at one offset.
func (u *Unit) Tokens(fn func(Token) bool) {
	var open []doc.Mark
	stop := false
	u.Content.Each(func(at int, it doc.Item) bool {
		end := at + it.Len()
		if end <= u.From || at >= u.To {
			return at < u.To
		}
		var marks []doc.Mark
		if it.Marks != nil {
			marks = it.Marks.Marks()
		}
		for i := len(open) - 1; i >= 0; i-- {
			if !slices.ContainsFunc(marks, func(m doc.Mark) bool { return m.Kind == open[i].Kind && m.Attrs.Equal(open[i].Attrs) }) {
				if !fn(Token{Kind: TokenMarkClose, At: max(at, u.From), Mark: open[i]}) {
					stop = true
					return false
				}
				open = slices.Delete(open, i, i+1)
			}
		}
		for _, m := range marks {
			if !slices.ContainsFunc(open, func(o doc.Mark) bool { return o.Kind == m.Kind && o.Attrs.Equal(m.Attrs) }) {
				if !fn(Token{Kind: TokenMarkOpen, At: max(at, u.From), Mark: m}) {
					stop = true
					return false
				}
				open = append(open, m)
			}
		}
		if it.Node != nil {
			stop = !fn(Token{Kind: TokenNode, At: at, Node: it.Node})
			return !stop
		}
		from, to := max(at, u.From), min(end, u.To)
		text := it.Text[from-at : to-at]
		// Lines are tokens of their own, so a line break is a boundary.
		for text != "" {
			i := strings.IndexByte(text, '\n')
			piece := text
			if i >= 0 {
				piece = text[:i+1]
			}
			if !fn(Token{Kind: TokenText, At: from, Text: piece}) {
				stop = true
				return false
			}
			from += len(piece)
			text = text[len(piece):]
		}
		return true
	})
	if stop {
		return
	}
	for i := len(open) - 1; i >= 0; i-- {
		if !fn(Token{Kind: TokenMarkClose, At: u.To, Mark: open[i]}) {
			return
		}
	}
}

// --- analyzers ------------------------------------------------------------

type analyzerForm uint8

const (
	formUnit analyzerForm = iota + 1
	formScanner
	formSummary
)

type analyzer struct {
	policy string
	form   analyzerForm
	kinds  []FactID
	// config identifies the analyzer's configuration: caches are keyed by
	// it, so only analyzers whose configuration changed lose them.
	config string
	where  func(*Unit) bool

	granularity Granularity
	unit        func(u *Unit, emit func(Fact))

	start func(u *Unit) any
	step  func(s any, t Token, emit func(Fact)) any

	summary *summaryDecl
}

// AnalyzerOption refines an analyzer.
type AnalyzerOption func(*analyzer)

// Where restricts an analyzer to units it accepts, such as slots of a
// kind. An analyzer configured per slot kind branches on Unit.Owner.
func Where(fn func(u *Unit) bool) AnalyzerOption { return func(a *analyzer) { a.where = fn } }

// Config names the analyzer's configuration for its cache key, for
// analyzers that are closures over option values.
func Config(v any) AnalyzerOption {
	return func(a *analyzer) { a.config = fmt.Sprintf("%T:%#v", v, v) }
}

func (p *P) addAnalyzer(a *analyzer, kinds []FactID, opts []AnalyzerOption) {
	a.policy = p.policy.id
	a.kinds = kinds
	for _, o := range opts {
		o(a)
	}
	for _, k := range kinds {
		if !slices.ContainsFunc(p.facts, func(d *factDecl) bool { return d.kind == k }) {
			p.fail("analyzer emits %s, which the policy does not declare", k)
		}
	}
	p.analyzers = append(p.analyzers, a)
}

// Analyze declares a per-unit analyzer, memoised per unit and
// configuration. It emits facts of kinds, at slot offsets.
func (p *P) Analyze(g Granularity, fn func(u *Unit, emit func(Fact)), kinds []FactID, opts ...AnalyzerOption) {
	a := &analyzer{form: formUnit, granularity: g, unit: fn}
	a.config = fmt.Sprintf("unit:%s:%v", p.policy.id, kinds)
	p.addAnalyzer(a, kinds, opts)
}

// Scanner is a scanning analyzer. Its state is checkpointed at chunk
// boundaries: after an edit, scanning restarts at the nearest checkpoint
// and stops once the state equals the cached one.
type Scanner[S comparable] interface {
	Start(u *Unit) S
	Step(s S, t Token, emit func(Fact)) S
}

// Scan declares a scanner over whole slots. The scanner value is its
// configuration.
func Scan[S comparable](p *P, sc Scanner[S], kinds []FactID, opts ...AnalyzerOption) {
	a := &analyzer{
		form:  formScanner,
		start: func(u *Unit) any { return sc.Start(u) },
		step:  func(s any, t Token, emit func(Fact)) any { return sc.Step(s.(S), t, emit) },
	}
	a.config = fmt.Sprintf("scan:%s:%T:%#v", p.policy.id, sc, sc)
	p.addAnalyzer(a, kinds, opts)
}

// Summary is an analyzer over nodes: Leaf summarises one node's own
// content, Combine joins summaries. Results are memoised per node pointer,
// so recomputing costs the changed path.
type Summary[M any] struct {
	Leaf     func(n *doc.Node) M
	Combine  func(a, b M) M
	Identity M
}

type summaryDecl struct {
	name     string
	leaf     func(n *doc.Node) any
	combine  func(a, b any) any
	identity any
}

// SummaryKind is a declared summary.
type SummaryKind[M any] struct{ decl *summaryDecl }

// Summarize declares a summary.
func Summarize[M any](p *P, name string, s Summary[M]) *SummaryKind[M] {
	d := &summaryDecl{
		name:     p.policy.id + "." + name,
		leaf:     func(n *doc.Node) any { return s.Leaf(n) },
		combine:  func(a, b any) any { return s.Combine(a.(M), b.(M)) },
		identity: s.Identity,
	}
	p.analyzers = append(p.analyzers, &analyzer{policy: p.policy.id, form: formSummary, summary: d, config: "summary:" + d.name})
	return &SummaryKind[M]{decl: d}
}
