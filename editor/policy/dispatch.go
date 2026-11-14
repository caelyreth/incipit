package policy

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// LevelClass is the class of a scope stack entry.
type LevelClass uint8

const (
	LevelNode LevelClass = iota + 1
	LevelSlot
	LevelFact
	LevelAnnotation
	// LevelBoundary is the boundary between two blocks a caret crosses.
	LevelBoundary
)

// Level is one entry of the scope stack.
type Level struct {
	Class LevelClass
	// Node is the node of a node level, or the owner of a slot level.
	Node *doc.Node
	// Slot is the slot of a slot level, or, for a node level, the node's
	// slot the caret is within; for facts and annotations, their slot.
	Slot       doc.SlotRef
	Fact       *Fact
	Annotation *Annotation
	// Start and End are the level's inner edges, as places: a fact's and an
	// annotation's stand at a stop; a node's and a slot's are content
	// edges, holding every stop at their offsets (doc.StopFirst,
	// doc.StopLast).
	Start, End doc.Caret
	// Crossing is set on the level of what a caret crosses toward its
	// next stop, for a rule standing at that crossing.
	Crossing *Crossing

	keys []Scope
	prog *Program
}

// Role returns the level's implementation of a role.
func (l *Level) Role(r doc.Reader, ar vocab.AnyRole) (any, bool) {
	role := ar.RoleID()
	switch l.Class {
	case LevelNode:
		return l.prog.nodeRole(r, l.Node, role)
	case LevelFact:
		d, ok := l.prog.factDecls[l.Fact.Kind]
		if !ok {
			return nil, false
		}
		bind, ok := d.binders[role]
		if !ok {
			return nil, false
		}
		return bind(l), true
	}
	return nil, false
}

func (l *Level) String() string {
	switch l.Class {
	case LevelNode:
		return fmt.Sprintf("node(%d)", l.Node.Kind())
	case LevelSlot:
		return fmt.Sprintf("slot:%s", l.Slot)
	case LevelFact:
		return fmt.Sprintf("fact:%s[%d,%d]", l.Fact.Kind, l.Fact.Outer.From.At, l.Fact.Outer.To.At)
	case LevelAnnotation:
		return "annotation:" + l.Annotation.Kind.String()
	case LevelBoundary:
		return "boundary"
	}
	return "level?"
}

// Stack is the scope stack at a caret, outermost first: the page and its
// containers, the caret's node and slot, the fact spans containing the
// caret, and the scoped annotations containing it.
type Stack struct {
	Levels []*Level
	Caret  doc.Caret
}

// Innermost is the innermost level.
func (s *Stack) Innermost() *Level { return s.Levels[len(s.Levels)-1] }

func (s *Stack) String() string {
	parts := make([]string, len(s.Levels))
	for i, l := range s.Levels {
		parts[i] = l.String()
	}
	return strings.Join(parts, " > ")
}

type stackKey struct {
	root    *doc.Node
	slot    doc.SlotRef
	segment int
	gap     int
	ann     AnnotationID
	nann    int
	// stop places the caret among glyphs, which are levels while
	// the caret is inside them.
	stop int
}

// stackAt builds the scope stack, memoised per caret segment, so motion
// within a segment rebuilds nothing.
func (p *Program) stackAt(store *Store, r doc.Reader, view *View, caret doc.Caret) *Stack {
	pos := caret.Pos
	ref := pos.Ref()
	owner, ok := r.Node(pos.Node)
	if !ok {
		owner = r.Root()
		pos = doc.Pos{Node: owner.ID()}
		ref = pos.Ref()
	}
	key := stackKey{root: r.Root(), slot: ref, gap: -1, stop: caret.Stop}
	if view != nil {
		key.ann, key.nann = view.Annotations.next, view.Annotations.Len()
	}
	inline := ref.Slot < owner.NumSlots() && owner.Slot(ref.Slot).IsInline()
	var facts []Fact
	if inline {
		content := owner.Slot(ref.Slot).Content()
		facts = p.scopeFacts(store, r, view, ref)
		bounds := store.segmentsOf(p.version, ref, content, func() []Fact { return facts })
		key.segment = segmentOf(bounds, pos.Offset)
	} else {
		key.gap = pos.Offset
	}
	p.stackMu.Lock()
	if s, ok := p.stacks[key]; ok {
		p.stackMu.Unlock()
		return &Stack{Levels: s.Levels, Caret: caret}
	}
	p.stackMu.Unlock()

	s := &Stack{Caret: caret}
	// The caret's node and its ancestors, outermost first.
	ids := doc.Ancestors(r, pos.Node)
	slices.Reverse(ids)
	ids = append(ids, pos.Node)
	for i, id := range ids {
		n, _ := r.Node(id)
		l := &Level{Class: LevelNode, Node: n, keys: p.nodeKeys(n.Kind()), prog: p}
		if i+1 < len(ids) {
			_, slot, _ := r.Parent(ids[i+1])
			l.Slot = doc.SlotRef{Node: id, Slot: slot}
		} else {
			l.Slot = ref
		}
		start, end := nodeEdges(r.Schema(), n)
		l.Start, l.End = doc.Caret{Pos: start, Stop: doc.StopFirst}, doc.Caret{Pos: end, Stop: doc.StopLast}
		s.Levels = append(s.Levels, l)
	}
	slotLevel := &Level{Class: LevelSlot, Node: owner, Slot: ref, keys: []Scope{slotScope}, prog: p}
	slotLevel.Start = doc.Caret{Pos: doc.At(ref, 0), Stop: doc.StopFirst}
	slotLevel.End = doc.Caret{Pos: doc.At(ref, 0), Stop: doc.StopLast}
	if ref.Slot < owner.NumSlots() {
		slotLevel.End.Offset = owner.Slot(ref.Slot).Len()
	}
	s.Levels = append(s.Levels, slotLevel)
	for i := range facts {
		f := facts[i]
		if !f.Holds(caret) {
			continue
		}
		s.Levels = append(s.Levels, p.factLevel(owner, ref, f))
	}
	if view != nil {
		for _, a := range view.Annotations.All() {
			if !a.Kind.Scoped() || !a.Anchor.Touches(caret) {
				continue
			}
			s.Levels = append(s.Levels, &Level{
				Class: LevelAnnotation, Node: owner, Slot: ref, Annotation: &a,
				Start: a.Anchor.From, End: a.Anchor.To,
				keys: []Scope{annotationScope(a.Kind)}, prog: p,
			})
		}
	}
	p.stackMu.Lock()
	if len(p.stacks) > 4096 {
		clear(p.stacks)
	}
	p.stacks[key] = s
	p.stackMu.Unlock()
	return s
}

func (p *Program) nodeKeys(k doc.Kind) []Scope {
	keys := []Scope{nodeScope(k)}
	for _, r := range p.nodeRoles(k) {
		keys = append(keys, roleScope(r))
	}
	return keys
}

// factKeys are what a fact answers to: its kind, its kind and key, and
// the roles its kind provides.
func (p *Program) factKeys(f Fact) []Scope {
	keys := []Scope{factScope(f.Kind)}
	if f.Key != nil {
		keys = append(keys, Scope{Class: ScopeFact, ID: uint32(f.Kind), Key: f.Key})
	}
	if d, ok := p.factDecls[f.Kind]; ok {
		for _, r := range d.roles {
			keys = append(keys, roleScope(r))
		}
	}
	return keys
}

// factLevel is a fact's level in a slot of owner.
func (p *Program) factLevel(owner *doc.Node, ref doc.SlotRef, f Fact) *Level {
	return &Level{Class: LevelFact, Node: owner, Slot: ref, Fact: &f,
		Start: f.Inner.From.Caret(ref), End: f.Inner.To.Caret(ref), keys: p.factKeys(f), prog: p}
}

// nodeLevel is a node's level, edged by its text (nodeEdges).
func (p *Program) nodeLevel(s *doc.Schema, n *doc.Node) *Level {
	start, end := nodeEdges(s, n)
	return &Level{Class: LevelNode, Node: n, Slot: start.Ref(), keys: p.nodeKeys(n.Kind()), prog: p,
		Start: doc.Caret{Pos: start, Stop: doc.StopFirst}, End: doc.Caret{Pos: end, Stop: doc.StopLast}}
}

// atEdge reports whether a caret stands at an edge: at its stop, or at
// any stop of its offset for a content edge, which holds them all.
func atEdge(c, edge doc.Caret) bool {
	if c.Pos != edge.Pos {
		return false
	}
	return edge.Stop == doc.StopFirst || edge.Stop == doc.StopLast || c.Stop == edge.Stop
}

// nodeEdges are a node's inner edges: the start of its first text and the
// end of its last, or of its first and last gaps. They are found through
// its blocks but not inside an object below it, which is whole within it:
// there the object's place is the edge.
func nodeEdges(s *doc.Schema, n *doc.Node) (doc.Pos, doc.Pos) {
	start, ok := edgeOf(s, n, false)
	if !ok {
		start = doc.Pos{Node: n.ID()}
	}
	end, ok := edgeOf(s, n, true)
	if !ok {
		end = start
		if n.NumSlots() > 0 {
			end = doc.Pos{Node: n.ID(), Slot: n.NumSlots() - 1, Offset: n.Slot(n.NumSlots() - 1).Len()}
		}
	}
	return start, end
}

// edgeOf is n's first edge, or its last: a text slot's start or end, or
// the place before or after an object below n.
func edgeOf(s *doc.Schema, n *doc.Node, last bool) (doc.Pos, bool) {
	slots := n.Slots()
	for k := range slots {
		i := k
		if last {
			i = len(slots) - 1 - k
		}
		sl := slots[i]
		if sl.IsInline() {
			p := doc.Pos{Node: n.ID(), Slot: i}
			if last {
				p.Offset = sl.Len()
			}
			return p, true
		}
		children := sl.Children()
		for j := range children {
			c := j
			if last {
				c = len(children) - 1 - j
			}
			if child := children[c]; s.IsObject(child.Kind()) {
				p := doc.Pos{Node: n.ID(), Slot: i, Offset: c}
				if last {
					p.Offset++
				}
				return p, true
			} else if p, ok := edgeOf(s, child, last); ok {
				return p, true
			}
		}
	}
	return doc.Pos{}, false
}

// scopeFacts are the facts of every kind that can form a scope or that a
// rule's place or conditions read, in the caret's slot.
func (p *Program) scopeFacts(store *Store, r doc.Reader, view *View, ref doc.SlotRef) []Fact {
	var all []Fact
	for _, k := range p.scopeKinds {
		all = append(all, p.facts(store, r, view, ref, k)...)
	}
	sortFacts(all)
	return all
}

// facts runs (or reads the cache of) the analyzer of kind k on a slot.
func (p *Program) facts(store *Store, r doc.Reader, view *View, ref doc.SlotRef, k FactID) []Fact {
	a, ok := p.analyzerOf[k]
	if !ok {
		return nil
	}
	all, ok := store.answered(p, a, r, view, ref)
	if !ok {
		u, ok := buildUnit(p, r, view, ref, a.policy)
		if !ok {
			return nil
		}
		all = store.analyze(a, u)
		store.answer(p, a, r, view, ref, u.Owner, all)
	}
	if len(a.kinds) == 1 {
		return all
	}
	return store.ofKind(all, k)
}

// --- places -----------------------------------------------------------------

// placed finds whether a place stands at level li: in it, or with it
// selected. Crossing places stand at crossings instead (crossings.go).
func (p *Program) placed(c *Ctx, stack *Stack, li int, alts []alt) (*Level, bool) {
	l := stack.Levels[li]
	caret := stack.Caret
	for _, a := range alts {
		if !slices.Contains(l.keys, a.index()) || a.class != 0 && a.class != l.Class {
			continue
		}
		switch a.how {
		case placeSelected:
			sel := c.view.Selection
			switch sel.Kind {
			case doc.SelectNode:
				if l.Class == LevelNode && l.Node.ID() == sel.Node {
					return l, true
				}
			case doc.SelectText:
				from, to := sel.Ordered(c.r)
				if atEdge(from, l.Start) && atEdge(to, l.End) {
					return l, true
				}
			}
		case placeIn:
			start, end := l.Start, l.End
			if a.own {
				own, ok := ownSlot(c.r, l, caret.Pos)
				if !ok {
					continue
				}
				start, end = slotEdges(c.r, own)
			}
			s, e := atEdge(caret, start), atEdge(caret, end)
			z := zoneWithin
			switch {
			case s && e:
				z = zoneEmpty
			case s:
				z = zoneStart
			case e:
				z = zoneEnd
			}
			if a.zones&z != 0 {
				return l, true
			}
		}
	}
	return nil, false
}

// ownSlot is the level's own text slot, when the caret is in it: a node's
// own inline slot, or any an object presents, however deep, as a table's
// cells (vocab.ObjectOf); or the slot of a slot level when it is inline.
func ownSlot(r doc.Reader, l *Level, pos doc.Pos) (doc.SlotRef, bool) {
	switch l.Class {
	case LevelNode:
		if l.Node == nil {
			return doc.SlotRef{}, false
		}
		if obj, ok := vocab.ObjectOf(r, pos.Node); pos.Node != l.Node.ID() && (!ok || obj != l.Node.ID()) {
			return doc.SlotRef{}, false
		}
	case LevelSlot:
	default:
		return pos.Ref(), true
	}
	return pos.Ref(), isInlineSlot(r, pos.Ref())
}

func slotEdges(r doc.Reader, ref doc.SlotRef) (doc.Caret, doc.Caret) {
	end := 0
	if n, ok := r.Node(ref.Node); ok && ref.Slot < n.NumSlots() {
		end = n.Slot(ref.Slot).Len()
	}
	return doc.Caret{Pos: doc.At(ref, 0), Stop: doc.StopFirst}, doc.Caret{Pos: doc.At(ref, end), Stop: doc.StopLast}
}

func isInlineSlot(r doc.Reader, ref doc.SlotRef) bool {
	n, ok := r.Node(ref.Node)
	return ok && ref.Slot < n.NumSlots() && n.Slot(ref.Slot).IsInline()
}

// --- dispatch ----------------------------------------------------------------

// Outcome is what dispatch decided for an intent.
type Outcome struct {
	Rule   string
	Policy string
	Result Result
	Trace  *Trace
}

// Available reports whether the intent would do anything.
func (o Outcome) Available() bool { return !o.Result.Empty() }

// Trace records why an intent did what it did.
type Trace struct {
	Intent    string
	Stack     string
	Passed    []string
	Rule      string
	Swallowed bool
	Rewrites  []RewriteTrace
	Observers []string
	Notes     []string
}

// RewriteTrace is one rewrite that fired.
type RewriteTrace struct {
	Rule    string
	Phase   doc.Phase
	Site    Site
	Records int
}

func (t *Trace) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s at %s", t.Intent, t.Stack)
	for _, p := range t.Passed {
		fmt.Fprintf(&b, "\n  passed by %s", p)
	}
	fmt.Fprintf(&b, "\n  handled by %s", t.Rule)
	if t.Swallowed {
		b.WriteString(" (swallowed)")
	}
	for _, r := range t.Rewrites {
		fmt.Fprintf(&b, "\n  rewrite %s in %s at %s [%d,%d]: %d records", r.Rule, PhaseName(r.Phase), r.Site.Slot, r.Site.From, r.Site.To, r.Records)
	}
	for _, o := range t.Observers {
		fmt.Fprintf(&b, "\n  observer %s", o)
	}
	for _, n := range t.Notes {
		fmt.Fprintf(&b, "\n  note: %s", n)
	}
	return b.String()
}

// ErrUnhandled means no rule, not even core's, answered: a compile check
// rules it out, so it signals a broken program.
var ErrUnhandled = errors.New("policy: no rule handled the intent")

// Dispatch finds the rule answering the context's intent and runs it. It
// walks the scope stack from the innermost scope outward; at each scope
// the first candidate whose intent, selection shape, place and conditions
// match runs, unless it passes. At the caret's slot it first walks the
// crossings toward the next stop (crossPhase). Core's rules come after every scope found
// nothing, innermost first. A feature's intent no rule answers does
// nothing: its policy is disabled, or it has nothing to do here.
func (p *Program) Dispatch(c *Ctx) (Outcome, error) {
	stack := c.Stack()
	intent := c.input.Intent
	sel := shapeOf(c.view.Selection)
	trace := &Trace{Stack: stack.String(), Intent: fmt.Sprintf("%T%+v", intent, intent)}
	for _, core := range []bool{false, true} {
		for li := len(stack.Levels) - 1; li >= 0; li-- {
			if !core && stack.Levels[li].Class == LevelSlot {
				if out, ok := p.crossPhase(c, sel, trace); ok {
					return out, nil
				}
			}
		rules:
			for _, r := range p.candidates(stack.Levels[li], intent.Kind(), core) {
				if r.shapes&sel == 0 || !r.match.match(intent) {
					continue
				}
				at, ok := p.placed(c, stack, li, r.alts)
				if !ok {
					continue
				}
				for _, cond := range r.conds {
					if !cond.test(c) {
						continue rules
					}
				}
				res, ok := r.run(c, intent, at)
				if !ok {
					continue
				}
				if res.pass {
					trace.Passed = append(trace.Passed, r.id())
					continue
				}
				trace.Rule = r.id()
				trace.Swallowed = res.Empty()
				return Outcome{Rule: r.id(), Policy: r.policy, Result: res, Trace: trace}, nil
			}
		}
	}
	if !intent.Kind().Primitive() {
		trace.Notes = append(trace.Notes, "no enabled policy answers "+intent.Kind().String())
		trace.Swallowed = true
		return Outcome{Trace: trace}, nil
	}
	return Outcome{Trace: trace}, ErrUnhandled
}

type candidateKey struct {
	keys   string
	intent vocab.IntentKind
	core   bool
}

// candidates are the rules for a level's keys and an intent kind, in
// precedence order, memoised per key set.
func (p *Program) candidates(l *Level, k vocab.IntentKind, core bool) []*rule {
	var kb strings.Builder
	for _, s := range l.keys {
		fmt.Fprintf(&kb, "%d.%d.%v;", s.Class, s.ID, s.Key)
	}
	key := candidateKey{keys: kb.String(), intent: k, core: core}
	p.candMu.Lock()
	if rs, ok := p.cands[key]; ok {
		p.candMu.Unlock()
		return rs
	}
	p.candMu.Unlock()
	var rs []*rule
	for _, s := range l.keys {
		for _, r := range p.table[tableKey{intent: k, scope: s}] {
			if (r.policy == CoreID) == core && !slices.Contains(rs, r) {
				rs = append(rs, r)
			}
		}
	}
	slices.SortStableFunc(rs, p.precedes)
	p.candMu.Lock()
	p.cands[key] = rs
	p.candMu.Unlock()
	return rs
}
