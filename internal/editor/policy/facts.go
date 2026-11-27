package policy

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Edge is a place a fact's extent starts or ends at: a slot offset and a
// stop there (§13.5). An edge at content holds every stop at its offset
// (doc.StopFirst or doc.StopLast); a glyph's edges stand at the stops just
// before and after it.
type Edge struct{ At, Stop int }

// Caret is the edge as a caret in a slot.
func (e Edge) Caret(slot doc.SlotRef) doc.Caret {
	return doc.Caret{Pos: doc.At(slot, e.At), Stop: e.Stop}
}

// order orders a caret against the edge.
func (e Edge) order(c doc.Caret) int {
	if c.Pos.Offset != e.At {
		return cmp.Compare(c.Pos.Offset, e.At)
	}
	return cmp.Compare(c.Stop, e.Stop)
}

// Range is the stretch between two edges.
type Range struct{ From, To Edge }

// Holds reports whether a caret is in the range, its edges included.
func (r Range) Holds(c doc.Caret) bool { return r.From.order(c) >= 0 && r.To.order(c) <= 0 }

// Fact is a recognised extent in one inline slot, at slot offsets. Outer
// is all of it, Inner what it holds: a pair without its delimiters. A
// keyed kind's facts carry their key, so a rule can stand in the facts of
// one key. A glyph fact is projected notation: Glyph is drawn at its
// offset, holding the stops between its edges.
type Fact struct {
	Kind         FactID
	Key          any
	Outer, Inner Range
	Data         any
	Glyph        *Glyph
}

// Holds reports whether a caret is inside the fact's inner extent.
func (f Fact) Holds(c doc.Caret) bool { return f.Inner.Holds(c) }

// Starts reports whether the fact starts right after the caret, nothing
// between them; Ends whether it ends right before it.
func (f Fact) Starts(c doc.Caret) bool {
	return c.Pos.Offset == f.Outer.From.At && c.Stop <= f.Outer.From.Stop
}
func (f Fact) Ends(c doc.Caret) bool {
	return c.Pos.Offset == f.Outer.To.At && c.Stop >= f.Outer.To.Stop
}

// Attach is the side of stop 0, a position's own stop, a glyph stands on.
// Content edited on either side carries its glyphs, and the carets among
// them, along.
type Attach uint8

const (
	// AttachBefore glyphs belong to the content before them, as notation
	// closing it or a block's marker at its start: they stand below stop
	// 0, the last ending there.
	AttachBefore Attach = iota + 1
	// AttachAfter glyphs belong to the content after them, as notation
	// opening it: they stand above stop 0, the first starting there.
	AttachAfter
)

// Glyph is notation a fact projects at a point: a spelling that spans no
// document bytes but holds a stop between each two of its glyphs and one
// after the last. The framework numbers the stops: at one offset glyphs
// stand in emission order on their side of stop 0.
type Glyph struct {
	Spelling vocab.Spelling
	// Groups style the glyph, outermost first.
	Groups []vocab.Group
	Attach Attach
	// Hidden draws nothing where the spelling would be; its stops stand.
	Hidden bool
	// Block marks a block's marker: the stop before it stands before the
	// block's structure, so a range starting there covers the block whole.
	Block bool
}

// Stops is how many stops the glyph holds.
func (g Glyph) Stops() int { return vocab.GraphemeCount(string(g.Spelling)) }

// GlyphRef names a glyph emitted in the unit being analyzed, for a span
// between two glyphs.
type GlyphRef struct{ i int }

// Emit collects what a per-unit analyzer recognises: facts, and glyphs
// whose stops the framework numbers once the unit is read.
type Emit struct {
	facts  []Fact
	glyphs []Fact
	spans  []pendingSpan
}

type pendingSpan struct {
	f           Fact
	open, close int
}

// Fact emits a fact edged by content.
func (e *Emit) Fact(f Fact) { e.facts = append(e.facts, f) }

// done numbers the glyphs' stops and returns every fact.
func (e *Emit) done() []Fact {
	below := map[int]int{}
	for _, g := range e.glyphs {
		if g.Glyph.Attach == AttachBefore {
			below[g.Outer.From.At] -= g.Glyph.Stops()
		}
	}
	above := map[int]int{}
	for i := range e.glyphs {
		g := &e.glyphs[i]
		at, n := g.Outer.From.At, g.Glyph.Stops()
		from := &above
		if g.Glyph.Attach == AttachBefore {
			from = &below
		}
		s := (*from)[at]
		(*from)[at] = s + n
		g.Outer = Range{Edge{at, s}, Edge{at, s + n}}
		g.Inner = Range{Edge{at, s + 1}, Edge{at, s + n - 1}}
	}
	out := append(e.facts, e.glyphs...)
	for _, sp := range e.spans {
		o, c := e.glyphs[sp.open].Outer, e.glyphs[sp.close].Outer
		sp.f.Outer = Range{o.From, c.To}
		sp.f.Inner = Range{o.To, c.From}
		out = append(out, sp.f)
	}
	return out
}

// FactKind is a typed fact kind a policy declared. It is a subject rules
// stand in: every fact of the kind.
type FactKind[T any] struct {
	Subject[T]
	decl *factDecl
}

// FactOption refines a fact kind's declaration.
type FactOption func(*factDecl)

type factDecl struct {
	kind     FactID
	name     string
	policy   string
	data     reflect.Type
	roles    []vocab.RoleID
	binders  map[vocab.RoleID]func(*Level) any
	key      func(any) any
	projects bool
	// styles is the group the kind's facts style the text they span in.
	styles vocab.Group
	errs   []string
}

// Provides declares roles the kind's facts provide, as bind makes them
// from a fact where it stands. R must implement each role.
func Provides[T, R any](bind func(At[T]) R, roles ...vocab.AnyRole) FactOption {
	return func(d *factDecl) {
		if d.data != reflect.TypeFor[T]() {
			d.errs = append(d.errs, fmt.Sprintf("provides binds %s, not the kind's data", reflect.TypeFor[T]()))
			return
		}
		for _, ar := range roles {
			r := ar.RoleID()
			if iface := r.Type(); iface != nil && !reflect.TypeFor[R]().Implements(iface) {
				d.errs = append(d.errs, fmt.Sprintf("provides %s, but %s does not implement it", r, reflect.TypeFor[R]()))
				continue
			}
			d.roles = append(d.roles, r)
			d.binders[r] = func(l *Level) any { return bind(At[T]{Level: l, Data: l.Fact.Data.(T)}) }
		}
	}
}

// Projects lets the kind's facts be glyphs.
func Projects() FactOption { return func(d *factDecl) { d.projects = true } }

// Styles sets the text the kind's facts span in group, wherever the caret
// is: what the content spells, presented as it reads, as a written tag
// reads as one.
func Styles(group vocab.Group) FactOption { return func(d *factDecl) { d.styles = group } }

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
	d := &factDecl{kind: stableFactKind(p.policy.id, name), name: name, policy: p.policy.id, data: reflect.TypeFor[T](),
		binders: map[vocab.RoleID]func(*Level) any{}}
	for _, o := range opts {
		o(d)
	}
	for _, e := range d.errs {
		p.fail("fact %s: %s", name, e)
	}
	p.facts = append(p.facts, d)
	k := &FactKind[T]{decl: d}
	k.Subject = factSubject[T](factScope(d.kind))
	return k
}

func factSubject[T any](s Scope) Subject[T] {
	return Subject[T]{scope: s, bind: func(_ *Ctx, l *Level) (T, bool) {
		if l.Fact == nil {
			var zero T
			return zero, false
		}
		v, ok := l.Fact.Data.(T)
		return v, ok
	}}
}

// Kind is the untyped handle.
func (k *FactKind[T]) Kind() FactID { return k.decl.kind }

// KeyedKind is a fact kind whose facts carry a key of type K from their
// data, so a rule can stand in the facts of one key.
type KeyedKind[T any, K comparable] struct{ *FactKind[T] }

// KeyedFacts declares a fact kind whose data is T, keyed by key.
func KeyedFacts[T any, K comparable](p *P, name string, key func(T) K, opts ...FactOption) KeyedKind[T, K] {
	k := Facts[T](p, name, opts...)
	k.decl.key = func(v any) any { return key(v.(T)) }
	return KeyedKind[T, K]{k}
}

// Key is the subject of the kind's facts of one key.
func (k KeyedKind[T, K]) Key(key K) Subject[T] {
	s := factScope(k.decl.kind)
	s.Key = key
	return factSubject[T](s)
}

func (k *FactKind[T]) fact(data T) Fact {
	f := Fact{Kind: k.decl.kind, Data: data}
	if k.decl.key != nil {
		f.Key = k.decl.key(data)
	}
	return f
}

// Span is a fact whose inner extent is its outer one, edged by content.
func (k *FactKind[T]) Span(from, to int, data T) Fact {
	return k.Delimited(from, from, to, to, data)
}

// Delimited is a fact with delimiters: [from, innerFrom) and [innerTo, to).
func (k *FactKind[T]) Delimited(from, innerFrom, innerTo, to int, data T) Fact {
	f := k.fact(data)
	f.Outer = Range{Edge{from, doc.StopLast}, Edge{to, doc.StopFirst}}
	f.Inner = Range{Edge{innerFrom, doc.StopFirst}, Edge{innerTo, doc.StopLast}}
	return f
}

// Glyph emits a glyph fact at an offset; the caret is inside it between
// its glyphs.
func (k *FactKind[T]) Glyph(e *Emit, at int, g Glyph, data T) GlyphRef {
	if !k.decl.projects {
		panic(fmt.Sprintf("policy: fact %s does not project", k.decl.name))
	}
	f := k.fact(data)
	f.Outer.From.At = at
	f.Glyph = &g
	e.glyphs = append(e.glyphs, f)
	return GlyphRef{i: len(e.glyphs) - 1}
}

// Between emits a fact spanning from one glyph to another: its inner
// extent is between them.
func (k *FactKind[T]) Between(e *Emit, open, close GlyphRef, data T) {
	e.spans = append(e.spans, pendingSpan{f: k.fact(data), open: open.i, close: close.i})
}

// Data returns a fact's typed data.
func (k *FactKind[T]) Data(f Fact) (T, bool) {
	if f.Kind != k.decl.kind {
		var zero T
		return zero, false
	}
	v, ok := f.Data.(T)
	return v, ok
}

// --- units --------------------------------------------------------------

// Granularity is the unit a per-unit analyzer reads.
type Granularity uint8

const (
	// Lines reads each line of an inline slot.
	Lines Granularity = iota + 1
	// Slots reads a whole inline slot.
	Slots
	// Sentences reads each sentence of an inline slot's lines, so an edit
	// reads its own sentence again, however long the line.
	Sentences
)

// Unit is what an analyzer reads: a line, a sentence or a whole inline
// slot, with the
// slot's kind and enclosing kinds, and the annotations the analyzer's own
// policy owns there and its mode for the view.
type Unit struct {
	Slot doc.SlotRef
	// Owner is the node holding the slot.
	Owner *doc.Node
	Spec  doc.SlotSpec
	// Enclosing are the kinds of the owner's ancestors, outermost first.
	Enclosing []doc.Kind
	Content   *doc.Inline
	// From and To are the unit's range in the slot; Line is the line's,
	// or the sentence's, index, or -1 for a whole slot.
	From, To int
	Line     int
	// Annotations are the analyzing policy's annotations anchored here.
	Annotations []Annotation
	// Mode is the analyzing policy's mode for the view, or nil.
	Mode any
	// Context is what the slot's place in the structure adds to its
	// content, as the profile reports it (vocab.Profile.Context). It
	// stands before the slot's text, so a unit after the first has none.
	Context any
}

// Text is the unit's text aligned with slot offsets: an inline node is
// doc.NodeByte, one byte as it is one position.
func (u *Unit) Text() string { return u.Content.FlatRange(u.From, u.To) }

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

// textToken is the most bytes of text a token holds: a scanner's checkpoint
// spacing (checkpointEvery).
const textToken = checkpointEvery

// textCut is where a token of text from from, before to, ends: at a line's
// end, at the first of cuts past from, or before textToken bytes, between
// runes.
func textCut(text string, from, to, base int, cuts func(after int) int) int {
	end := to
	if i := strings.IndexByte(text[from:to], '\n'); i >= 0 {
		end = from + i + 1
	}
	limit := min(end, from+textToken)
	if cuts != nil {
		if c := cuts(base+from) - base; c > from && c < limit {
			return c
		}
	}
	if limit < end {
		for limit > from+1 && !utf8.RuneStart(text[limit]) {
			limit--
		}
	}
	return limit
}

// Tokens streams the unit's tokens in order. Mark closes come before
// opens at one offset. A text token is at most a line, and at most
// textToken bytes: a longer run of text comes in pieces cut between runes,
// so a scanner keeps what spans pieces in its state, and a scan can resume
// within a long paragraph (Store.scan).
func (u *Unit) Tokens(fn func(Token) bool) { u.tokens(nil, fn) }

// tokens streams the unit's tokens as Tokens does, a text token ending
// also at each offset cuts gives, when given: the first after an offset.
func (u *Unit) tokens(cuts func(after int) int, fn func(Token) bool) {
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
			cut := textCut(it.Text, from-at, to-at, at, cuts)
			piece := it.Text[from-at : cut]
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
	unit        func(u *Unit, e *Emit)

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
func (p *P) Analyze(g Granularity, fn func(u *Unit, e *Emit), kinds []FactID, opts ...AnalyzerOption) {
	a := &analyzer{form: formUnit, granularity: g, unit: fn}
	a.config = fmt.Sprintf("unit:%s:%v", p.policy.id, kinds)
	p.addAnalyzer(a, kinds, opts)
}

// Scanner is a scanning analyzer. Its state is checkpointed between tokens,
// at line starts and every checkpointEvery bytes: after an edit, scanning
// restarts at the nearest checkpoint and stops once the state equals the
// cached one. A text token may be a piece of a run (Tokens), so what spans
// tokens is the state's.
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
