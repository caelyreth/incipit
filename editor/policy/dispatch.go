package policy

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

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
	// Start and End are the level's inner edges.
	Start, End doc.Pos

	keys []vocab.Scope
}

// Keys are the scope keys the level answers to: its kind and its roles.
func (l *Level) Keys() []vocab.Scope { return l.keys }

// Provides reports whether the level provides a role.
func (l *Level) Provides(r vocab.Role) bool { return slices.Contains(l.keys, vocab.RoleScope(r)) }

// Role returns the level's implementation of a role.
func (l *Level) Role(r doc.Reader, role vocab.Role) (any, bool) {
	switch l.Class {
	case LevelNode:
		return vocab.NodeRole(r, l.Node, role)
	case LevelFact:
		if !l.Provides(role) {
			return nil, false
		}
		return Placed(l.Slot, *l.Fact), l.Fact.Data != nil
	}
	return nil, false
}

func (l *Level) String() string {
	switch l.Class {
	case LevelNode:
		return "node:" + vocab.Schema().KindName(l.Node.Kind())
	case LevelSlot:
		return fmt.Sprintf("slot:%s", l.Slot)
	case LevelFact:
		return fmt.Sprintf("fact:%s[%d,%d]", l.Fact.Kind, l.Fact.From, l.Fact.To)
	case LevelAnnotation:
		return "annotation:" + l.Annotation.Kind.String()
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

// Find returns the innermost level answering to a scope key.
func (s *Stack) Find(key vocab.Scope) (*Level, int, bool) {
	for i := len(s.Levels) - 1; i >= 0; i-- {
		if slices.Contains(s.Levels[i].keys, key) {
			return s.Levels[i], i, true
		}
	}
	return nil, -1, false
}

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
	// stop places the caret among ghost points, which are levels while
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
		l := &Level{Class: LevelNode, Node: n, keys: nodeKeys(n.Kind())}
		if i+1 < len(ids) {
			_, slot, _ := r.Parent(ids[i+1])
			l.Slot = doc.SlotRef{Node: id, Slot: slot}
		} else {
			l.Slot = ref
		}
		l.Start, l.End = nodeEdges(n)
		s.Levels = append(s.Levels, l)
	}
	slotLevel := &Level{Class: LevelSlot, Node: owner, Slot: ref, keys: []vocab.Scope{vocab.Slot}}
	slotLevel.Start = doc.At(ref, 0)
	if ref.Slot < owner.NumSlots() {
		slotLevel.End = doc.At(ref, owner.Slot(ref.Slot).Len())
	}
	s.Levels = append(s.Levels, slotLevel)
	for i := range facts {
		f := facts[i]
		if !f.Holds(caret) {
			continue
		}
		s.Levels = append(s.Levels, &Level{
			Class: LevelFact, Node: owner, Slot: ref, Fact: &f,
			Start: doc.At(ref, f.InnerFrom), End: doc.At(ref, f.InnerTo),
			keys: p.factKeys(f.Kind),
		})
	}
	if view != nil {
		for _, a := range view.Annotations.All() {
			if !a.Kind.Scoped() || !a.Anchor.Touches(caret) {
				continue
			}
			s.Levels = append(s.Levels, &Level{
				Class: LevelAnnotation, Node: owner, Slot: ref, Annotation: &a,
				Start: a.Anchor.From.Pos, End: a.Anchor.To.Pos,
				keys: []vocab.Scope{vocab.AnnotationScope(a.Kind)},
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

func nodeKeys(k doc.Kind) []vocab.Scope {
	keys := []vocab.Scope{vocab.NodeScope(k)}
	for _, r := range vocab.NodeRoles(k) {
		keys = append(keys, vocab.RoleScope(r))
	}
	return keys
}

func (p *Program) factKeys(k vocab.FactKind) []vocab.Scope {
	keys := []vocab.Scope{vocab.FactScope(k)}
	if d, ok := p.factDecls[k]; ok {
		for _, r := range d.roles {
			keys = append(keys, vocab.RoleScope(r))
		}
	}
	return keys
}

// nodeEdges are a node's inner edges: the start of its first text slot and
// the end of its last, or its first and last gaps.
func nodeEdges(n *doc.Node) (doc.Pos, doc.Pos) {
	start := doc.Pos{Node: n.ID()}
	end := start
	if f, ok := vocab.FirstTextSlot(n); ok {
		start = doc.At(f, 0)
	}
	if l, ok := vocab.LastTextSlot(n); ok {
		end = doc.At(l, 0)
		if owner := findNode(n, l.Node); owner != nil {
			end.Offset = owner.Slot(l.Slot).Len()
		}
	} else if n.NumSlots() > 0 {
		end = doc.Pos{Node: n.ID(), Slot: n.NumSlots() - 1, Offset: n.Slot(n.NumSlots() - 1).Len()}
	}
	return start, end
}

func findNode(n *doc.Node, id doc.ID) *doc.Node {
	var out *doc.Node
	n.Walk(func(d *doc.Node) bool {
		if d.ID() == id {
			out = d
		}
		return out == nil
	})
	return out
}

// scopeFacts are the facts of every kind that can form a scope or that a
// rule's atoms read, in the caret's slot.
func (p *Program) scopeFacts(store *Store, r doc.Reader, view *View, ref doc.SlotRef) []Fact {
	var all []Fact
	for _, k := range p.scopeKinds {
		all = append(all, p.facts(store, r, view, ref, k)...)
	}
	sortFacts(all)
	return all
}

// facts runs (or reads the cache of) the analyzer of kind k on a slot.
func (p *Program) facts(store *Store, r doc.Reader, view *View, ref doc.SlotRef, k vocab.FactKind) []Fact {
	a, ok := p.analyzerOf[k]
	if !ok {
		return nil
	}
	u, ok := buildUnit(p, store, r, view, ref, a.policy)
	if !ok {
		return nil
	}
	all := store.analyze(a, u)
	if len(a.kinds) == 1 {
		return all
	}
	var out []Fact
	for _, f := range all {
		if f.Kind == k {
			out = append(out, f)
		}
	}
	return out
}

// --- atoms -----------------------------------------------------------------

type atomSet [4]uint64

const maxAtoms = 256

func (s *atomSet) set(i int)     { s[i/64] |= 1 << (i % 64) }
func (s atomSet) has(i int) bool { return s[i/64]&(1<<(i%64)) != 0 }
func (s atomSet) and(o atomSet) atomSet {
	return atomSet{s[0] & o[0], s[1] & o[1], s[2] & o[2], s[3] & o[3]}
}
func (s atomSet) empty() bool { return s == atomSet{} }
func (s atomSet) subset(o atomSet) bool {
	return s.and(o) == s
}
func (s atomSet) each(fn func(int)) {
	for i := 0; i < maxAtoms; i++ {
		if s.has(i) {
			fn(i)
		}
	}
}

type atomKey struct {
	level int
	atom  int
}

// evaluator evaluates atoms at stack levels, memoised per level.
type evaluator struct {
	p     *Program
	c     *Ctx
	stack *Stack
	memo  map[atomKey]bool
}

func (e *evaluator) holds(level, atom int) bool {
	k := atomKey{level, atom}
	if v, ok := e.memo[k]; ok {
		return v
	}
	v := e.eval(level, e.p.atoms[atom])
	e.memo[k] = v
	return v
}

func (e *evaluator) matches(level int, r *rule) bool {
	ok := true
	r.required.each(func(i int) {
		if ok && !e.holds(level, i) {
			ok = false
		}
	})
	r.forbidden.each(func(i int) {
		if ok && e.holds(level, i) {
			ok = false
		}
	})
	return ok
}

// levelOf resolves the level a relation is evaluated at: its own, or the
// enclosing one Of names.
func (e *evaluator) levelOf(level int, at vocab.Scope) (int, bool) {
	if at.Class == 0 {
		return level, true
	}
	if at.Class == vocab.ScopeParent {
		return level - 1, level > 0
	}
	for i := level - 1; i >= 0; i-- {
		if slices.Contains(e.stack.Levels[i].keys, at) {
			return i, true
		}
	}
	return 0, false
}

func (e *evaluator) eval(level int, rel vocab.Relation) bool {
	li, ok := e.levelOf(level, rel.At)
	if !ok {
		return false
	}
	l := e.stack.Levels[li]
	c := e.c
	caret := e.stack.Caret
	pos := caret.Pos
	sel := c.view.Selection
	switch rel.Kind {
	case vocab.RelAtInnerStart:
		return pos == l.Start
	case vocab.RelAtInnerEnd:
		return pos == l.End
	case vocab.RelCollapsed:
		return sel.IsCollapsed()
	case vocab.RelTextRange:
		return sel.Kind == doc.SelectText && !sel.IsCollapsed()
	case vocab.RelNodeSelected:
		if sel.Kind != doc.SelectNode {
			return false
		}
		n, ok := c.r.Node(sel.Node)
		return ok && (rel.Node == 0 || n.Kind() == rel.Node)
	case vocab.RelCellRange:
		return sel.Kind == doc.SelectCells
	case vocab.RelComposing:
		return c.view.Composition != nil
	case vocab.RelHas:
		return len(c.view.Annotations.At(rel.Annotation, caret)) > 0
	case vocab.RelPrevChar, vocab.RelNextChar:
		return e.charClass(pos, rel.Kind == vocab.RelPrevChar)&rel.Class != 0
	case vocab.RelSlotStart, vocab.RelSlotEnd, vocab.RelSlotEmpty:
		return e.slotAtom(l, pos, rel.Kind)
	case vocab.RelOwnText:
		return pos.Ref() == l.Slot && l.Node != nil && l.Node.ID() == l.Slot.Node && isInlineSlot(c.r, l.Slot)
	case vocab.RelBefore, vocab.RelAfter:
		return e.adjacent(caret, rel.Target, rel.Kind == vocab.RelBefore)
	case vocab.RelEquals:
		from, to := sel.Ordered(c.r)
		if sel.Kind == doc.SelectNode {
			return l.Class == LevelNode && l.Node.ID() == sel.Node
		}
		return from.Pos == l.Start && to.Pos == l.End
	case vocab.RelCovers, vocab.RelIntersects, vocab.RelCuts:
		return e.selectionRelation(rel.Kind, rel.Target)
	}
	return false
}

// charClass classifies the grapheme before or after pos.
// TODO(words): use grapheme clusters, not runes, once the words policy
// provides boundaries through the vocabulary.
func (e *evaluator) charClass(pos doc.Pos, before bool) vocab.CharClass {
	in, ok := e.c.Inline(pos.Ref())
	if !ok {
		if before {
			return vocab.ClassLineStart
		}
		return vocab.ClassLineEnd
	}
	if before {
		if pos.Offset == 0 {
			return vocab.ClassLineStart
		}
		it, start, _ := in.ItemAt(pos.Offset - 1)
		if it.Node != nil {
			return 0
		}
		s := it.Text[:pos.Offset-start]
		r, _ := utf8.DecodeLastRuneInString(s)
		if r == '\n' {
			return vocab.ClassLineStart
		}
		return vocab.ClassOf(r)
	}
	if pos.Offset >= in.Len() {
		return vocab.ClassLineEnd
	}
	it, start, _ := in.ItemAt(pos.Offset)
	if it.Node != nil {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(it.Text[pos.Offset-start:])
	if r == '\n' {
		return vocab.ClassLineEnd
	}
	return vocab.ClassOf(r)
}

func isInlineSlot(r doc.Reader, ref doc.SlotRef) bool {
	n, ok := r.Node(ref.Node)
	return ok && ref.Slot < n.NumSlots() && n.Slot(ref.Slot).IsInline()
}

// slotAtom evaluates SlotStart, SlotEnd and SlotEmpty for a level's slot.
func (e *evaluator) slotAtom(l *Level, pos doc.Pos, kind vocab.RelationKind) bool {
	r := e.c.r
	n, ok := r.Node(l.Slot.Node)
	if !ok || l.Slot.Slot >= n.NumSlots() {
		return false
	}
	slot := n.Slot(l.Slot.Slot)
	if kind == vocab.RelSlotEmpty {
		return slotEmpty(slot)
	}
	if slot.IsInline() {
		if pos.Ref() != l.Slot {
			return false
		}
		if kind == vocab.RelSlotStart {
			return pos.Offset == 0
		}
		return pos.Offset == slot.Len()
	}
	if pos.Ref() == l.Slot {
		if kind == vocab.RelSlotStart {
			return pos.Offset == 0
		}
		return pos.Offset == slot.Len()
	}
	if slot.Len() == 0 {
		return false
	}
	edge := slot.Child(0)
	if kind == vocab.RelSlotEnd {
		edge = slot.Child(slot.Len() - 1)
	}
	start, end := nodeEdges(edge)
	if kind == vocab.RelSlotStart {
		return pos == start
	}
	return pos == end
}

// slotEmpty: an inline slot with no content, or a block slot with no
// children or only one empty text block.
func slotEmpty(s doc.Slot) bool {
	if s.IsInline() {
		return s.Len() == 0
	}
	if s.Len() == 0 {
		return true
	}
	if s.Len() > 1 {
		return false
	}
	child := s.Child(0)
	if !vocab.IsTextBlock(child.Kind()) {
		return false
	}
	for _, cs := range child.Slots() {
		if cs.Len() > 0 {
			return false
		}
	}
	return true
}

// adjacent evaluates Before(target) (something of target right after the
// caret) and After(target) (right before it).
func (e *evaluator) adjacent(caret doc.Caret, target vocab.Scope, before bool) bool {
	r := e.c.r
	pos := caret.Pos
	owner, ok := r.Node(pos.Node)
	if !ok || pos.Slot >= owner.NumSlots() {
		return false
	}
	slot := owner.Slot(pos.Slot)
	if !slot.IsInline() {
		i := pos.Offset
		if !before {
			i--
		}
		return i >= 0 && i < slot.Len() && e.nodeIs(slot.Child(i), target)
	}
	in := slot.Content()
	// Inline nodes next to the caret.
	if before && pos.Offset < in.Len() {
		if it, _, ok := in.ItemAt(pos.Offset); ok && it.Node != nil && e.nodeIs(it.Node, target) {
			return true
		}
	}
	if !before && pos.Offset > 0 {
		if it, _, ok := in.ItemAt(pos.Offset - 1); ok && it.Node != nil && e.nodeIs(it.Node, target) {
			return true
		}
	}
	// Facts starting or ending at the caret, outside it.
	if target.Class == vocab.ScopeFact || target.Class == vocab.ScopeRole {
		if _, ok := e.p.edged(e.c.store, r, e.c.view, caret, target, before); ok {
			return true
		}
	}
	// Neighbouring blocks, at the edges of a text block's slot.
	if target.Class == vocab.ScopeNode || target.Class == vocab.ScopeRole {
		block, ok := vocab.Block(r, pos)
		if !ok || block.ID() != owner.ID() {
			return false
		}
		parent, ps, index, ok := vocab.Index(r, block.ID())
		if !ok {
			return false
		}
		pn, _ := r.Node(parent)
		siblings := pn.Slot(ps)
		if before && pos.Offset == in.Len() && pos.Slot == owner.NumSlots()-1 && index+1 < siblings.Len() {
			return e.nodeIs(siblings.Child(index+1), target)
		}
		if !before && pos.Offset == 0 && pos.Slot == 0 && index > 0 {
			return e.nodeIs(siblings.Child(index-1), target)
		}
	}
	return false
}

func (e *evaluator) nodeIs(n *doc.Node, target vocab.Scope) bool {
	return slices.Contains(nodeKeys(n.Kind()), target)
}

// selectionRelation evaluates Covers, Intersects and Cuts against spans of
// the target in the slots of the selection's ends.
func (e *evaluator) selectionRelation(kind vocab.RelationKind, target vocab.Scope) bool {
	c := e.c
	sel := c.view.Selection
	if sel.Kind != doc.SelectText || sel.IsCollapsed() {
		return false
	}
	from, to := sel.Ordered(c.r)
	in := func(p doc.Pos) bool {
		cp := doc.CaretAt(p)
		return doc.Compare(c.r, from, cp) <= 0 && doc.Compare(c.r, cp, to) <= 0
	}
	slots := []doc.SlotRef{from.Pos.Ref()}
	if to.Pos.Ref() != from.Pos.Ref() {
		slots = append(slots, to.Pos.Ref())
	}
	for _, ref := range slots {
		for _, f := range c.prog.scopeFacts(c.store, c.r, c.view, ref) {
			if f.Point || !slices.Contains(c.prog.factKeys(f.Kind), target) {
				continue
			}
			a, b := in(doc.At(ref, f.From)), in(doc.At(ref, f.To))
			switch kind {
			case vocab.RelCovers:
				if a && b {
					return true
				}
			case vocab.RelCuts:
				if a != b {
					return true
				}
			case vocab.RelIntersects:
				fs, fe := doc.CaretAt(doc.At(ref, f.From)), doc.CaretAt(doc.At(ref, f.To))
				if doc.Compare(c.r, fs, to) < 0 && doc.Compare(c.r, from, fe) < 0 {
					return true
				}
			}
		}
	}
	return false
}

// --- dispatch ----------------------------------------------------------------

// Outcome is what dispatch decided for an intent.
type Outcome struct {
	// Intent is the intent the handler consumed, after transforms.
	Intent vocab.Intent
	Rule   string
	Policy string
	Match  Match
	Result Result
	// Produces are the record kinds the rule declared.
	Produces []doc.RecordKind
	Trace    *Trace
}

// Available reports whether the intent would do anything.
func (o Outcome) Available() bool { return !o.Result.Empty() }

// Trace records why an intent did what it did.
type Trace struct {
	Intent    string
	Stack     string
	Chain     []string
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
	for _, c := range t.Chain {
		fmt.Fprintf(&b, "\n  transform %s", c)
	}
	fmt.Fprintf(&b, "\n  handled by %s", t.Rule)
	if t.Swallowed {
		b.WriteString(" (swallowed)")
	}
	for _, r := range t.Rewrites {
		fmt.Fprintf(&b, "\n  rewrite %s in %v at %s [%d,%d]: %d records", r.Rule, r.Phase, r.Site.Slot, r.Site.From, r.Site.To, r.Records)
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

// ErrTransformLoop means a transform chain did not end.
var ErrTransformLoop = errors.New("policy: transform chain too long")

const maxChain = 16

// Dispatch finds the rule answering the context's intent and runs it. It
// walks the scope stack from the innermost scope outward; at each scope
// the first matching candidate wins: a Handle runs and dispatch ends, a
// Transform replaces the intent and dispatch continues at the same scope.
// Core's rules come after every scope found nothing, innermost first.
func (p *Program) Dispatch(c *Ctx) (Outcome, error) {
	stack := c.Stack()
	ev := &evaluator{p: p, c: c, stack: stack, memo: map[atomKey]bool{}}
	intent := c.input.Intent
	trace := &Trace{Stack: stack.String(), Intent: fmt.Sprintf("%T%+v", intent, intent)}
	for _, core := range []bool{false, true} {
		for li := len(stack.Levels) - 1; li >= 0; li-- {
			level := stack.Levels[li]
		again:
			for _, r := range p.candidates(level, intent.Kind(), core) {
				if !r.matchesIntent(intent) || !ev.matches(li, r) {
					continue
				}
				m := Match{Intent: intent, Level: level}
				if r.in.Class == vocab.ScopeRole {
					m.Role, _ = level.Role(c.r, vocab.Role(r.in.ID))
				}
				if r.typ == ruleTransform {
					next := r.trans(c, m)
					trace.Chain = append(trace.Chain, fmt.Sprintf("%s: %T%+v", r.id(), next, next))
					if len(trace.Chain) > maxChain {
						return Outcome{Trace: trace}, ErrTransformLoop
					}
					if !slices.Contains(r.emits, next.Kind()) {
						trace.Notes = append(trace.Notes, fmt.Sprintf("%s emitted an undeclared %s", r.id(), next.Kind()))
						if p.strict {
							return Outcome{Trace: trace}, fmt.Errorf("policy: %s emitted undeclared %s", r.id(), next.Kind())
						}
					}
					intent = next
					nc := *c
					nc.input = vocab.Input{Intent: next, At: c.input.At, Class: c.input.Class}
					c = &nc
					ev.c = c
					goto again
				}
				res := r.handle(c, m)
				trace.Rule = r.id()
				trace.Swallowed = res.Empty()
				return Outcome{Intent: intent, Rule: r.id(), Policy: r.policy, Match: m, Result: res, Produces: r.produces, Trace: trace}, nil
			}
		}
	}
	return Outcome{Trace: trace}, ErrUnhandled
}

func (r *rule) matchesIntent(in vocab.Intent) bool {
	for _, pat := range r.on {
		if pat.Match(in) {
			return true
		}
	}
	return false
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
		fmt.Fprintf(&kb, "%d.%d;", s.Class, s.ID)
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

// edged is the fact of target that starts right after the caret (ahead)
// or ends right before it, nothing between them: of several, the one
// whose edge is nearest the caret.
func (p *Program) edged(store *Store, r doc.Reader, view *View, caret doc.Caret, target vocab.Scope, ahead bool) (Fact, bool) {
	var found Fact
	ok := false
	for _, f := range p.scopeFacts(store, r, view, caret.Pos.Ref()) {
		if !slices.Contains(p.factKeys(f.Kind), target) {
			continue
		}
		switch {
		case ahead && f.Starts(caret) && (!ok || f.StopFrom <= found.StopFrom):
			found, ok = f, true
		case !ahead && f.Ends(caret) && (!ok || f.StopTo >= found.StopTo):
			found, ok = f, true
		}
	}
	return found, ok
}
