package policy

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Composition is an active IME composition: the preedit text shown at a
// point, which is view state, not document content. A commit arrives as a
// Type intent with origin IME.
type Composition struct {
	At       doc.Pos
	Text     string
	Selected [2]int
}

// View is one view's state, as the engine keeps it and rules read it.
type View struct {
	ID          uint64
	Selection   doc.Selection
	Composition *Composition
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
	Kind     vocab.AnnotationKind
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
	// Composition sets the composition; EndComposition clears it.
	Composition    *Composition
	EndComposition bool
	Annotate       []Annotate
	Drop           []AnnotationID
	DropKinds      []vocab.AnnotationKind
	Deadlines      []time.Time
	Effects        []Effect
	History        *HistoryRequest
	// GoalX, when set, is the new remembered horizontal position; motion
	// that is not vertical clears it.
	GoalX *float64
	// KeepGoal keeps the remembered horizontal position.
	KeepGoal bool
}

// Empty reports whether the result does nothing.
func (r Result) Empty() bool {
	return len(r.Edits) == 0 && r.Selection == nil && r.Composition == nil && !r.EndComposition &&
		len(r.Annotate) == 0 && len(r.Drop) == 0 && len(r.DropKinds) == 0 && len(r.Deadlines) == 0 &&
		len(r.Effects) == 0 && r.History == nil && r.GoalX == nil
}

// Do returns a result running edits.
func Do(edits ...vocab.Edit) Result { return Result{Edits: edits} }

// Select returns a result setting the selection.
func Select(s doc.Selection) Result { return Result{Selection: &s} }

// With returns r with the selection set.
func (r Result) With(s doc.Selection) Result { r.Selection = &s; return r }

// Swallow is the explicit empty result.
var Swallow = Result{}

// Match is what a signature bound: the scope level it matched at and the
// typed instance there.
type Match struct {
	Intent vocab.Intent
	Level  *Level
	// Role is the level's implementation of the rule's role, when the rule
	// is placed In a role.
	Role any
}

// Fact is the matched fact, when the level is a fact span.
func (m Match) Fact() (Fact, bool) {
	if m.Level == nil || m.Level.Fact == nil {
		return Fact{}, false
	}
	return *m.Level.Fact, true
}

// Handler consumes an intent.
type Handler func(c *Ctx, m Match) Result

// TransformFunc rewrites an intent into another.
type TransformFunc func(c *Ctx, m Match) vocab.Intent

type ruleType uint8

const (
	ruleHandle ruleType = iota + 1
	ruleTransform
)

type rule struct {
	policy   string
	name     string
	typ      ruleType
	on       []vocab.Pattern
	in       vocab.Scope
	when     []vocab.Relation
	unless   []vocab.Relation
	handle   Handler
	trans    TransformFunc
	produces []doc.RecordKind
	emits    []vocab.IntentKind
	order    int

	// lowered by compilation
	required, forbidden atomSet
}

func (r *rule) id() string { return r.policy + "." + r.name }

// Rule builds a Handle or Transform rule.
type Rule struct{ r *rule }

// Handle declares a Handle rule.
func (p *P) Handle(name string) *Rule {
	r := &rule{policy: p.policy.id, name: name, typ: ruleHandle, in: vocab.Slot, order: len(p.rules)}
	p.rules = append(p.rules, r)
	return &Rule{r: r}
}

// Transform declares a Transform rule.
func (p *P) Transform(name string) *Rule {
	r := &rule{policy: p.policy.id, name: name, typ: ruleTransform, in: vocab.Slot, order: len(p.rules)}
	p.rules = append(p.rules, r)
	return &Rule{r: r}
}

// On sets the intent patterns the rule matches.
func (b *Rule) On(patterns ...vocab.Pattern) *Rule { b.r.on = append(b.r.on, patterns...); return b }

// In places the rule at a scope kind or role. The default is the caret's
// slot.
func (b *Rule) In(s vocab.Scope) *Rule { b.r.in = s; return b }

// When requires relations.
func (b *Rule) When(rels ...vocab.Relation) *Rule { b.r.when = append(b.r.when, rels...); return b }

// Unless forbids relations.
func (b *Rule) Unless(rels ...vocab.Relation) *Rule {
	b.r.unless = append(b.r.unless, rels...)
	return b
}

// Do sets a Handle rule's handler.
func (b *Rule) Do(h Handler) *Rule { b.r.handle = h; return b }

// To sets a Transform rule's function.
func (b *Rule) To(fn TransformFunc) *Rule { b.r.trans = fn; return b }

// Produces declares the record kinds the rule's edits produce.
func (b *Rule) Produces(kinds ...doc.RecordKind) *Rule {
	b.r.produces = append(b.r.produces, kinds...)
	return b
}

// Emits declares the intent kinds a Transform can emit.
func (b *Rule) Emits(kinds ...vocab.IntentKind) *Rule {
	b.r.emits = append(b.r.emits, kinds...)
	return b
}

// --- rewrites and observers ----------------------------------------------

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
	demands  []vocab.FactKind
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
func (b *RewriteRule) Demands(kinds ...vocab.FactKind) *RewriteRule {
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
