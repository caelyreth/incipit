package policy

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Composition is an active IME composition: the preedit text shown at a
// point, which is view state, not document content. A commit arrives as a
// Type intent, as typing does.
type Composition struct {
	At       doc.Pos
	Text     string
	Selected [2]int
}

// Pending is the marks text typed at a caret takes instead of the marks
// around it: a format toggled there, a run just closed, the marks a stop
// among signs decides. It lives while the selection head stays at At.
type Pending struct {
	At    doc.Caret
	Marks *doc.MarkSet
	// Owner is the policy that set it; Origin the transaction.
	Owner  string
	Origin uint64
}

// Map carries the pending marks' caret through edits.
func (p *Pending) Map(carry doc.Carry) *Pending {
	if p == nil {
		return nil
	}
	at, ok := carry(p.At, doc.BiasAfter)
	if !ok {
		return nil
	}
	out := *p
	out.At = at
	return &out
}

// Settle ends the pending marks once the head has left them. Marks set in
// transaction tx stand at its head, wherever it settled.
func (p *Pending) Settle(tx uint64, head doc.Caret) *Pending {
	switch {
	case p == nil:
		return nil
	case p.Origin == tx:
		out := *p
		out.At = head
		return &out
	case p.At != head:
		return nil
	}
	return p
}

// View is one view's state, as the engine keeps it and rules read it.
type View struct {
	ID          uint64
	Selection   doc.Selection
	Composition *Composition
	// Pending is the marks typing at the caret takes, or nil.
	Pending     *Pending
	Annotations Annotations
	// GoalX is the remembered horizontal position for vertical motion.
	GoalX float64
}

// HistoryAction asks the engine to undo or redo.
type HistoryAction uint8

const (
	HistoryUndo HistoryAction = iota + 1
	HistoryRedo
)

// HistoryRequest is a history action for the intent's origin class.
type HistoryRequest struct {
	Action   HistoryAction
	TopLayer bool
}

// EffectKind is a view effect a result asks the renderer for.
type EffectKind uint8

const (
	// EffectReveal scrolls the selection head into view.
	EffectReveal EffectKind = iota + 1
	// EffectBeep signals a refused intent.
	EffectBeep
	// EffectMedia asks the host to load media; the answer is a Media
	// intent with Token.
	EffectMedia
)

// Effect is a view effect.
type Effect struct {
	Kind  EffectKind
	Token string
	Data  any
}

// Annotate creates an annotation; its owner is the policy whose rule
// returned it.
type Annotate struct {
	Kind     AnnotationKind
	Anchor   Anchor
	Data     any
	Lifetime Lifetime
}

// Result is what a handler, rewrite or observer returns. It is data: the
// engine applies it. An empty Result from a handler is an explicit,
// traced swallow.
type Result struct {
	// Edits run in order on the transaction.
	Edits []vocab.Edit
	// Selection, when set, is the selection after the edits, in
	// post-edit positions. Unset, the selection is mapped through them.
	Selection *doc.Selection
	// Then, when set, computes the selection after the edits from the
	// edited document, for carets in nodes the edits create. It is pure.
	Then func(r doc.Reader) doc.Selection
	// Composition sets the composition; EndComposition clears it.
	Composition    *Composition
	EndComposition bool
	// Pending sets the marks text typed at the resulting caret takes;
	// DropPending clears them.
	Pending     *doc.MarkSet
	DropPending bool
	Annotate    []Annotate
	Drop        []AnnotationID
	DropKinds   []AnnotationKind
	Deadlines   []time.Time
	Effects     []Effect
	History     *HistoryRequest
	// GoalX, when set, is the new remembered horizontal position; motion
	// that is not vertical clears it.
	GoalX *float64
	// KeepGoal keeps the remembered horizontal position.
	KeepGoal bool

	pass bool
}

// Empty reports whether the result does nothing.
func (r Result) Empty() bool {
	return len(r.Edits) == 0 && r.Selection == nil && r.Then == nil && r.Composition == nil && !r.EndComposition &&
		r.Pending == nil && !r.DropPending &&
		len(r.Annotate) == 0 && len(r.Drop) == 0 && len(r.DropKinds) == 0 && len(r.Deadlines) == 0 &&
		len(r.Effects) == 0 && r.History == nil && r.GoalX == nil
}

// Do returns a result running edits.
func Do(edits ...vocab.Edit) Result { return Result{Edits: edits} }

// Select returns a result setting the selection.
func Select(s doc.Selection) Result { return Result{Selection: &s} }

// Reveal sets the selection and scrolls it into view.
func Reveal(s doc.Selection) Result {
	return Result{Selection: &s, Effects: []Effect{{Kind: EffectReveal}}}
}

// With returns r with the selection set.
func (r Result) With(s doc.Selection) Result { r.Selection = &s; return r }

// Swallow is the explicit empty result: the intent is answered and does
// nothing.
var Swallow = Result{}

// Pass declines the intent: dispatch goes on to the next rule, as if this
// one had not matched.
var Pass = Result{pass: true}

// At is what a rule's place bound: the stack level it matched at (for a
// place beside the caret, the level of what stands there) and the typed
// value there.
type At[T any] struct {
	*Level
	Data T
}

// shape is a set of selection shapes a rule answers.
type shape uint8

const (
	shapeCaret shape = 1 << iota
	shapeRange
	shapeNode
	shapeCells
	shapeAny = shapeCaret | shapeRange | shapeNode | shapeCells
)

func shapeOf(s doc.Selection) shape {
	switch {
	case s.Kind == doc.SelectNode:
		return shapeNode
	case s.Kind == doc.SelectCells:
		return shapeCells
	case s.IsCollapsed():
		return shapeCaret
	}
	return shapeRange
}

type rule struct {
	policy string
	name   string
	match  matcher
	alts   []alt
	shapes shape
	// shaped is set once a refinement chose the shapes.
	shaped bool
	conds  []Cond
	run    func(c *Ctx, in vocab.Intent, l *Level) (Result, bool)
	order  int
}

func (r *rule) id() string { return r.policy + "." + r.name }

// Rule refines a declared rule.
type Rule struct{ r *rule }

// On declares a rule: intents matching pat, at place, run fn. A rule
// answers a collapsed caret unless refined; at a Selected place, a range
// or a node selection. Refinements choose the shapes instead, each adding
// its own: Range().Node() answers both.
func On[I vocab.Intent, T any](p *P, name string, pat Pattern[I], place Place[T], fn func(c *Ctx, in I, at At[T]) Result) *Rule {
	r := &rule{policy: p.policy.id, name: name, match: pat.m, alts: place.alts, shapes: shapeCaret, order: len(p.rules)}
	if place.alts[0].how == placeSelected {
		r.shapes = shapeRange | shapeNode
	}
	r.run = func(c *Ctx, in vocab.Intent, l *Level) (Result, bool) {
		v, ok := place.bind(c, l)
		if !ok {
			return Result{}, false
		}
		return fn(c, in.(I), At[T]{Level: l, Data: v}), true
	}
	p.rules = append(p.rules, r)
	return &Rule{r: r}
}

// When adds conditions at the caret.
func (b *Rule) When(conds ...Cond) *Rule { b.r.conds = append(b.r.conds, conds...); return b }

func (b *Rule) shape(s shape) *Rule {
	if !b.r.shaped {
		b.r.shapes, b.r.shaped = 0, true
	}
	b.r.shapes |= s
	return b
}

// Caret answers a collapsed caret, Range a text range, Node a node
// selection; Any answers every selection.
func (b *Rule) Caret() *Rule { return b.shape(shapeCaret) }
func (b *Rule) Range() *Rule { return b.shape(shapeRange) }
func (b *Rule) Node() *Rule  { return b.shape(shapeNode) }
func (b *Rule) Any() *Rule   { return b.shape(shapeAny) }

// --- rewrites and observers ----------------------------------------------

// The rewrite phases, in order (§11.13). Each that edits becomes a history
// layer of its own, after the handler's.
const (
	// PhaseStructure recognises structure from text that arrived without
	// the keystroke rules seeing it.
	PhaseStructure doc.Phase = doc.PhaseHandler + 1 + iota
	// PhaseCharacters replaces symbols and punctuation.
	PhaseCharacters
	// PhaseScript refines by script.
	PhaseScript
	// PhaseCleanup normalises last.
	PhaseCleanup
)

// Phases lists the rewrite phases in order.
var Phases = [...]doc.Phase{PhaseStructure, PhaseCharacters, PhaseScript, PhaseCleanup}

// PhaseName names a phase for traces.
func PhaseName(p doc.Phase) string {
	switch p {
	case doc.PhaseHandler:
		return "Handler"
	case PhaseStructure:
		return "Structure"
	case PhaseCharacters:
		return "Characters"
	case PhaseScript:
		return "Script"
	case PhaseCleanup:
		return "Cleanup"
	}
	return fmt.Sprintf("phase(%d)", p)
}

// Extent is what a rewrite visits.
type Extent uint8

const (
	// TouchedLines visits each line a record touched.
	TouchedLines Extent = iota + 1
	// TouchedSlots visits each slot a record touched.
	TouchedSlots
	// ChangedRanges visits each changed range.
	ChangedRanges
)

// Trigger selects journal records: kinds and origin classes. Empty sets
// match anything.
type Trigger struct {
	Kinds   []doc.RecordKind
	Classes []doc.OriginClass
}

// Matches reports whether a record triggers.
func (t Trigger) Matches(r doc.Record) bool {
	ok := len(t.Kinds) == 0
	for _, k := range t.Kinds {
		if r.Kind == k {
			ok = true
		}
	}
	if !ok {
		return false
	}
	if len(t.Classes) == 0 {
		return true
	}
	for _, c := range t.Classes {
		if r.Origin.Class == c {
			return true
		}
	}
	return false
}

// Site is one place a rewrite visits: a slot range and the records that
// touched it.
type Site struct {
	Slot     doc.SlotRef
	From, To int
	Records  []doc.Record
}

// RewriteFunc rewrites one site. It may iterate internally until its own
// output converges; it runs at most once per site per transaction.
type RewriteFunc func(c *Ctx, s Site) Result

type rewrite struct {
	policy   string
	name     string
	phase    doc.Phase
	trigger  Trigger
	extent   Extent
	demands  []FactID
	produces []doc.RecordKind
	fn       RewriteFunc
	order    int
}

func (r *rewrite) id() string { return r.policy + "." + r.name }

// RewriteRule builds a rewrite.
type RewriteRule struct{ r *rewrite }

// Rewrite declares a rewrite in a phase.
func (p *P) Rewrite(name string, phase doc.Phase) *RewriteRule {
	r := &rewrite{policy: p.policy.id, name: name, phase: phase, extent: TouchedSlots, order: len(p.rewrites)}
	p.rewrites = append(p.rewrites, r)
	return &RewriteRule{r: r}
}

// Trigger sets the records that trigger the rewrite.
func (b *RewriteRule) Trigger(t Trigger) *RewriteRule { b.r.trigger = t; return b }

// Extent sets what the rewrite visits.
func (b *RewriteRule) Extent(e Extent) *RewriteRule { b.r.extent = e; return b }

// Demands declares the facts the rewrite reads.
func (b *RewriteRule) Demands(kinds ...FactID) *RewriteRule {
	b.r.demands = append(b.r.demands, kinds...)
	return b
}

// Produces declares the record kinds the rewrite produces.
func (b *RewriteRule) Produces(kinds ...doc.RecordKind) *RewriteRule {
	b.r.produces = append(b.r.produces, kinds...)
	return b
}

// Do sets the rewrite function.
func (b *RewriteRule) Do(fn RewriteFunc) *RewriteRule { b.r.fn = fn; return b }

// ObserverFunc reacts to a transaction's records. Its result may hold
// only annotations, deadlines and effects.
type ObserverFunc func(c *Ctx, records []doc.Record) Result

type observer struct {
	policy  string
	name    string
	trigger Trigger
	fn      ObserverFunc
}

func (o *observer) id() string { return o.policy + "." + o.name }

// Observe declares an observer, run after the last rewrite phase.
func (p *P) Observe(name string, t Trigger, fn ObserverFunc) {
	p.observers = append(p.observers, &observer{policy: p.policy.id, name: name, trigger: t, fn: fn})
}

// Rewriter is a compiled rewrite, as the engine runs it.
type Rewriter struct{ r *rewrite }

func (w Rewriter) ID() string                 { return w.r.id() }
func (w Rewriter) Policy() string             { return w.r.policy }
func (w Rewriter) Phase() doc.Phase           { return w.r.phase }
func (w Rewriter) Trigger() Trigger           { return w.r.trigger }
func (w Rewriter) Extent() Extent             { return w.r.extent }
func (w Rewriter) Produces() []doc.RecordKind { return w.r.produces }

// Run rewrites one site.
func (w Rewriter) Run(c *Ctx, s Site) Result { return w.r.fn(c, s) }

// Observer is a compiled observer, as the engine runs it.
type Observer struct{ o *observer }

func (o Observer) ID() string       { return o.o.id() }
func (o Observer) Policy() string   { return o.o.policy }
func (o Observer) Trigger() Trigger { return o.o.trigger }

// Run observes a transaction's records.
func (o Observer) Run(c *Ctx, records []doc.Record) Result { return o.o.fn(c, records) }
