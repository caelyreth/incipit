package policy

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// CoreID is the core policy's id. Core is implicitly below every policy
// and cannot be disabled.
const CoreID = "core"

// Order is a manifest precedence entry.
type Order struct {
	over      bool
	a, b      *Policy
	qualifier any
	phase     doc.Phase
}

// Over gives a's rules precedence over b's where both match. The
// qualifier narrows where: a vocab.Role, a Scope, a Pattern,
// a vocab.IntentKind, or nil for everywhere.
func Over(a, b *Policy, qualifier any) Order {
	return Order{over: true, a: a, b: b, qualifier: qualifier}
}

// RunsBefore runs a's rewrites before b's in a phase.
func RunsBefore(a, b *Policy, phase doc.Phase) Order { return Order{a: a, b: b, phase: phase} }

// Manifest is the composition root's declaration: the policies, in order,
// and their precedence. It is the only code that names more than one
// policy.
type Manifest struct {
	Enabled []*Policy
	Resolve []Order
	// Strict makes undeclared outputs errors. Tests set it; a host may.
	Strict bool
	// Profile is the document profile the program edits.
	Profile vocab.Profile
}

// OptionInfo is an option in the catalog.
type OptionInfo struct {
	Key     string
	Kind    OptionKind
	Default any
	Choices []any
	// ChoiceLabels are the choices' label keys, in Choices' order.
	ChoiceLabels []string
	Min, Max     int
	Label        string
	Description  string
	Group        string
	Requires     []Requirement
}

// PolicyInfo is a policy in the catalog.
type PolicyInfo struct {
	ID      string
	Enabled bool
	Options []OptionInfo
}

// Catalog lists policies, their options and the commands, for the
// settings UI and the keymap.
type Catalog struct {
	Policies []PolicyInfo
	Commands []Command
}

// Report is the compile report.
type Report struct {
	Errors []string
	// Dropped rules need facts no enabled analyzer produces.
	Dropped []string
	// Shadowed rules can never win.
	Shadowed []string
	// Dead triggers no rule can produce.
	Dead []string
}

func (r *Report) errorf(format string, args ...any) {
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

func (r *Report) String() string {
	var b strings.Builder
	for _, s := range r.Errors {
		fmt.Fprintf(&b, "error: %s\n", s)
	}
	for _, s := range r.Dropped {
		fmt.Fprintf(&b, "dropped: %s\n", s)
	}
	for _, s := range r.Shadowed {
		fmt.Fprintf(&b, "shadowed: %s\n", s)
	}
	for _, s := range r.Dead {
		fmt.Fprintf(&b, "dead trigger: %s\n", s)
	}
	return b.String()
}

// CompileError carries the report of a failed compile.
type CompileError struct{ Report *Report }

func (e *CompileError) Error() string {
	return "policy: compile failed:\n" + strings.Join(e.Report.Errors, "\n")
}

type tableKey struct {
	intent vocab.IntentKind
	scope  Scope
}

// Program is a compiled, immutable policy program. Its caches (stacks and
// candidate lists) are internal memos.
type Program struct {
	version  uint64
	strict   bool
	profile  vocab.Profile
	policies []*P
	rank     map[string]int
	orders   []Order

	table      map[tableKey][]*rule
	rules      []*rule
	atoms      []Relation
	atomIndex  map[Relation]int
	phases     map[doc.Phase][]*rewrite
	observers  []*observer
	analyzerOf map[FactID]*analyzer
	// analyzing are the policies with analyzers, in compile order.
	analyzing  []string
	factDecls  map[FactID]*factDecl
	scopeKinds []FactID
	annotation map[annotationKey]AnnotationCheck
	commands   map[vocab.CommandID]Command
	catalog    Catalog
	report     *Report
	grouping   func(prev, next *doc.Entry) bool
	guides     []Guide

	paints      []Paint
	ghosts      []GhostFunc
	decorations []Decoration

	stackMu sync.Mutex
	stacks  map[stackKey]*Stack
	candMu  sync.Mutex
	cands   map[candidateKey][]*rule
}

type annotationKey struct {
	kind  AnnotationKind
	owner string
}

var programVersion atomic.Uint64

// Version identifies the program; presentation keys include it.
func (p *Program) Version() uint64 { return p.version }

// Report is the compile report.
func (p *Program) Report() *Report { return p.report }

// Catalog is the program's catalog.
func (p *Program) Catalog() Catalog { return p.catalog }

// Command resolves a command id to its intent.
func (p *Program) Command(id vocab.CommandID) (Command, bool) {
	c, ok := p.commands[id]
	return c, ok
}

// Rewrites returns the rewrites of a phase, in order.
func (p *Program) Rewrites(ph doc.Phase) []Rewriter {
	out := make([]Rewriter, len(p.phases[ph]))
	for i, r := range p.phases[ph] {
		out[i] = Rewriter{r: r}
	}
	return out
}

// Observers returns the observers, in order.
func (p *Program) Observers() []Observer {
	out := make([]Observer, len(p.observers))
	for i, o := range p.observers {
		out[i] = Observer{o: o}
	}
	return out
}

// Grouping is the history grouping function, or nil.
func (p *Program) Grouping() func(prev, next *doc.Entry) bool { return p.grouping }

// Paints, Ghosts and Decorations are the presentation contributions.
func (p *Program) Paints() []Paint           { return p.paints }
func (p *Program) Ghosts() []GhostFunc       { return p.ghosts }
func (p *Program) Decorations() []Decoration { return p.decorations }
func (p *Program) Guides() []Guide           { return p.guides }
func (p *Program) Strict() bool              { return p.strict }

// HasAnnotationKind reports whether annotations of k survive in this
// program: core kinds always do, others while their owner is enabled.
func (p *Program) HasAnnotationKind(k AnnotationKind) bool {
	switch k {
	case Armed, PendingMarks:
		return true
	}
	for key := range p.annotation {
		if key.kind == k {
			return true
		}
	}
	return false
}

// CheckAnnotation runs the owner's re-check of an annotation.
func (p *Program) CheckAnnotation(r doc.Reader, a Annotation) bool {
	check, ok := p.annotation[annotationKey{kind: a.Kind, owner: a.Owner}]
	if !ok || check == nil {
		return true
	}
	return check(r, a)
}

// Compile builds every enabled policy with its option values, resolves,
// lowers and checks, and emits a program.
func Compile(m Manifest, values Values) (*Program, error) {
	prog := &Program{
		version:    programVersion.Add(1),
		strict:     m.Strict,
		profile:    m.Profile,
		rank:       map[string]int{},
		orders:     m.Resolve,
		table:      map[tableKey][]*rule{},
		atomIndex:  map[Relation]int{},
		phases:     map[doc.Phase][]*rewrite{},
		analyzerOf: map[FactID]*analyzer{},
		factDecls:  map[FactID]*factDecl{},
		annotation: map[annotationKey]AnnotationCheck{},
		commands:   map[vocab.CommandID]Command{},
		stacks:     map[stackKey]*Stack{},
		cands:      map[candidateKey][]*rule{},
	}
	rep := &Report{}
	prog.report = rep

	// Build.
	seen := map[string]bool{}
	hasCore := false
	for i, pol := range m.Enabled {
		if seen[pol.id] {
			rep.errorf("policy %s listed twice", pol.id)
			continue
		}
		seen[pol.id] = true
		enabled := true
		if v, ok := values[EnabledKey(pol.id)].(bool); ok && pol.id != CoreID {
			enabled = v
		}
		p := &P{policy: pol, values: values}
		pol.build(p)
		info := PolicyInfo{ID: pol.id, Enabled: enabled}
		info.Options = append(info.Options, OptionInfo{Key: EnabledKey(pol.id), Kind: OptionBool, Default: true, Label: "settings." + pol.id + ".enabled"})
		for _, o := range p.options {
			info.Options = append(info.Options, OptionInfo{Key: o.Key, Kind: o.Kind, Default: o.Default, Choices: o.Choices, ChoiceLabels: o.ChoiceLabels,
				Min: o.Min, Max: o.Max, Label: o.meta.label, Description: o.meta.description, Group: o.meta.group, Requires: o.meta.requires})
		}
		prog.catalog.Policies = append(prog.catalog.Policies, info)
		if !enabled {
			continue
		}
		for _, err := range p.errs {
			rep.errorf("%v", err)
		}
		prog.rank[pol.id] = i
		prog.policies = append(prog.policies, p)
		if pol.id == CoreID {
			hasCore = true
		}
	}
	if !hasCore {
		rep.errorf("the manifest has no core policy")
	}

	// Resolve fact kinds, analyzers and annotation kinds.
	for _, p := range prog.policies {
		for _, d := range p.facts {
			prog.factDecls[d.kind] = d
		}
		for _, a := range p.analyzers {
			if !slices.Contains(prog.analyzing, a.policy) {
				prog.analyzing = append(prog.analyzing, a.policy)
			}
			for _, k := range a.kinds {
				if prev, ok := prog.analyzerOf[k]; ok && prev != a {
					rep.errorf("fact %s has two analyzers", k)
				}
				prog.analyzerOf[k] = a
			}
		}
		for _, d := range p.annotations {
			key := annotationKey{kind: d.kind, owner: p.policy.id}
			if _, ok := prog.annotation[key]; ok {
				rep.errorf("policy %s declares annotation %s twice", p.policy.id, d.kind)
				continue
			}
			prog.annotation[key] = d.check
		}
		for _, c := range p.commands {
			if _, ok := prog.commands[c.ID]; ok {
				rep.errorf("command %s declared twice", c.ID)
			}
			prog.commands[c.ID] = c
			prog.catalog.Commands = append(prog.catalog.Commands, c)
		}
		if p.grouping != nil {
			if prog.grouping != nil {
				rep.errorf("two policies declare history grouping")
			}
			prog.grouping = p.grouping
		}
		prog.paints = append(prog.paints, p.paints...)
		prog.ghosts = append(prog.ghosts, p.ghosts...)
		prog.guides = append(prog.guides, p.guides...)
		prog.decorations = append(prog.decorations, p.decorations...)
	}
	produced := func(k FactID) bool { _, ok := prog.analyzerOf[k]; return ok }
	roleProvided := func(r vocab.Role) bool {
		if len(prog.kindsProviding(r)) > 0 {
			return true
		}
		for k, d := range prog.factDecls {
			if slices.Contains(d.roles, r) && produced(k) {
				return true
			}
		}
		return false
	}
	needs := func(s Scope) (string, bool) {
		switch s.Class {
		case ScopeFact:
			if !produced(FactID(s.ID)) {
				return s.String(), false
			}
		case ScopeRole:
			if !roleProvided(vocab.Role(s.ID)) {
				return s.String(), false
			}
		}
		return "", true
	}

	// Lower rules.
	scopeKinds := map[FactID]bool{}
	for _, p := range prog.policies {
	rules:
		for _, r := range p.rules {
			if len(r.on) == 0 {
				rep.errorf("rule %s matches no intent", r.id())
				continue
			}
			if r.typ == ruleHandle && r.handle == nil || r.typ == ruleTransform && r.trans == nil {
				rep.errorf("rule %s has no handler", r.id())
				continue
			}
			if r.typ == ruleTransform && len(r.emits) == 0 {
				rep.errorf("transform %s declares no outputs", r.id())
				continue
			}
			for _, s := range append([]Scope{r.in}, relationTargets(r.when)...) {
				if what, ok := needs(s); !ok {
					rep.Dropped = append(rep.Dropped, fmt.Sprintf("%s: nothing produces %s", r.id(), what))
					continue rules
				}
			}
			var err error
			if r.required, err = prog.lower(r.when); err != nil {
				rep.errorf("rule %s: %v", r.id(), err)
				continue
			}
			if r.forbidden, err = prog.lower(r.unless); err != nil {
				rep.errorf("rule %s: %v", r.id(), err)
				continue
			}
			for _, s := range append([]Scope{r.in}, relationTargets(append(slices.Clone(r.when), r.unless...))...) {
				prog.noteScopeKinds(s, scopeKinds)
			}
			prog.rules = append(prog.rules, r)
			for _, k := range patternKinds(r.on) {
				key := tableKey{intent: k, scope: r.in}
				prog.table[key] = append(prog.table[key], r)
			}
		}
	}
	for k := range scopeKinds {
		prog.scopeKinds = append(prog.scopeKinds, k)
	}
	slices.Sort(prog.scopeKinds)
	for key, rs := range prog.table {
		slices.SortStableFunc(rs, prog.precedes)
		prog.table[key] = rs
	}

	// Rewrites and observers.
	for _, p := range prog.policies {
		for _, r := range p.rewrites {
			if r.fn == nil {
				rep.errorf("rewrite %s has no function", r.id())
				continue
			}
			for _, k := range r.demands {
				if !produced(k) {
					rep.Dropped = append(rep.Dropped, fmt.Sprintf("%s: nothing produces %s", r.id(), k))
				}
			}
			prog.phases[r.phase] = append(prog.phases[r.phase], r)
		}
		prog.observers = append(prog.observers, p.observers...)
	}
	for ph, rs := range prog.phases {
		prog.phases[ph] = prog.orderRewrites(ph, rs, rep)
	}

	for _, g := range prog.guides {
		if _, ok := prog.commands[g.Command]; !ok {
			rep.errorf("a guide shows %s, which no enabled policy declares", g.Command)
		}
	}
	prog.check(rep)
	if len(rep.Errors) > 0 {
		return prog, &CompileError{Report: rep}
	}
	return prog, nil
}

// nodeRoles are the roles a node kind provides in the profile.
func (p *Program) nodeRoles(k doc.Kind) []vocab.Role {
	if p.profile == nil {
		return nil
	}
	return p.profile.Roles(k)
}

// nodeRole is n's implementation of role in the profile.
func (p *Program) nodeRole(r doc.Reader, n *doc.Node, role vocab.Role) (any, bool) {
	if p.profile == nil {
		return nil, false
	}
	return p.profile.Role(r, n, role)
}

// rootScopes are where core's fallbacks stand: the slot and the root.
func (p *Program) rootScopes() []Scope {
	return []Scope{Slot, Root}
}

func (p *Program) kindsProviding(r vocab.Role) []doc.Kind {
	var out []doc.Kind
	if p.profile == nil {
		return nil
	}
	for _, k := range p.profile.Schema().Kinds() {
		if slices.Contains(p.nodeRoles(k), r) {
			out = append(out, k)
		}
	}
	return out
}

func relationTargets(rels []Relation) []Scope {
	var out []Scope
	for _, r := range rels {
		if r.Target.Class != 0 {
			out = append(out, r.Target)
		}
		if r.At.Class != 0 && r.At.Class != ScopeParent {
			out = append(out, r.At)
		}
	}
	return out
}

func (p *Program) noteScopeKinds(s Scope, into map[FactID]bool) {
	switch s.Class {
	case ScopeFact:
		into[FactID(s.ID)] = true
	case ScopeRole:
		for k, d := range p.factDecls {
			if slices.Contains(d.roles, vocab.Role(s.ID)) {
				if _, ok := p.analyzerOf[k]; ok {
					into[k] = true
				}
			}
		}
	}
}

func (p *Program) lower(rels []Relation) (atomSet, error) {
	var s atomSet
	for _, r := range rels {
		i, ok := p.atomIndex[r]
		if !ok {
			if len(p.atoms) == maxAtoms {
				return s, fmt.Errorf("more than %d distinct atoms", maxAtoms)
			}
			i = len(p.atoms)
			p.atoms = append(p.atoms, r)
			p.atomIndex[r] = i
		}
		s.set(i)
	}
	return s, nil
}

func patternKinds(ps []Pattern) []vocab.IntentKind {
	var out []vocab.IntentKind
	for _, p := range ps {
		if !slices.Contains(out, p.Kind) {
			out = append(out, p.Kind)
		}
	}
	return out
}

// --- precedence ---------------------------------------------------------------

// ordered reports whether the manifest gives a precedence over b for two
// rules, under its qualifier.
func (p *Program) ordered(a, b *rule) bool {
	for _, o := range p.orders {
		if o.over && o.a.id == a.policy && o.b.id == b.policy && p.qualifies(o.qualifier, a, b) {
			return true
		}
	}
	return false
}

func (p *Program) qualifies(q any, a, b *rule) bool {
	switch q := q.(type) {
	case nil:
		return true
	case vocab.Role:
		s := RoleScope(q)
		return p.coincide(a.in, s) || p.coincide(b.in, s)
	case Scope:
		return p.coincide(a.in, q) && p.coincide(b.in, q)
	case Pattern:
		return patternsOverlap(a.on, []Pattern{q}) && patternsOverlap(b.on, []Pattern{q})
	case vocab.IntentKind:
		return slices.Contains(patternKinds(a.on), q) && slices.Contains(patternKinds(b.on), q)
	}
	return false
}

// precedes orders candidates: the manifest's order first, then more
// specific patterns, then manifest position, then declaration order.
func (p *Program) precedes(a, b *rule) int {
	if ac, bc := a.policy == CoreID, b.policy == CoreID; ac != bc {
		// Core is below everything.
		if ac {
			return 1
		}
		return -1
	}
	switch {
	case p.ordered(a, b):
		return -1
	case p.ordered(b, a):
		return 1
	}
	sa, sb := specificity(a), specificity(b)
	if sa != sb {
		return sb - sa
	}
	if ra, rb := p.rank[a.policy], p.rank[b.policy]; ra != rb {
		return ra - rb
	}
	return a.order - b.order
}

func specificity(r *rule) int {
	n := 0
	for _, pat := range r.on {
		if pat.Specific() {
			n = 1
		}
	}
	c := 0
	r.required.each(func(int) { c++ })
	r.forbidden.each(func(int) { c++ })
	return n*1000 + c
}

func patternsOverlap(a, b []Pattern) bool {
	for _, x := range a {
		for _, y := range b {
			if x.Overlaps(y) {
				return true
			}
		}
	}
	return false
}

// coincide reports whether two scope keys can name the same stack level.
func (p *Program) coincide(a, b Scope) bool {
	if a == b {
		return true
	}
	if a.Class == ScopeRole {
		a, b = b, a
	}
	if b.Class != ScopeRole {
		return false
	}
	role := vocab.Role(b.ID)
	switch a.Class {
	case ScopeNode:
		return slices.Contains(p.nodeRoles(doc.Kind(a.ID)), role)
	case ScopeFact:
		d, ok := p.factDecls[FactID(a.ID)]
		return ok && slices.Contains(d.roles, role)
	case ScopeRole:
		other := vocab.Role(a.ID)
		for _, k := range p.kindsProviding(role) {
			if slices.Contains(p.nodeRoles(k), other) {
				return true
			}
		}
		for _, d := range p.factDecls {
			if slices.Contains(d.roles, role) && slices.Contains(d.roles, other) {
				return true
			}
		}
	}
	return false
}

// compatible reports whether two signatures can hold at once.
func compatible(a, b *rule) bool {
	return a.required.and(b.forbidden).empty() && b.required.and(a.forbidden).empty()
}

func (p *Program) orderRewrites(ph doc.Phase, rs []*rewrite, rep *Report) []*rewrite {
	before := func(a, b *rewrite) bool {
		for _, o := range p.orders {
			if !o.over && o.phase == ph && o.a.id == a.policy && o.b.id == b.policy {
				return true
			}
		}
		return false
	}
	// Interference: overlapping triggers and outputs between policies.
	for i, a := range rs {
		for _, b := range rs[i+1:] {
			if a.policy == b.policy || !interfere(a, b) {
				continue
			}
			if !before(a, b) && !before(b, a) {
				rep.errorf("rewrites %s and %s interfere in %v; the manifest must order them", a.id(), b.id(), ph)
			}
		}
	}
	out := slices.Clone(rs)
	// A stable topological order: by manifest Before, then rank, then
	// declaration.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if before(a, b) {
			return true
		}
		if before(b, a) {
			return false
		}
		if p.rank[a.policy] != p.rank[b.policy] {
			return p.rank[a.policy] < p.rank[b.policy]
		}
		return a.order < b.order
	})
	return out
}

func interfere(a, b *rewrite) bool {
	feeds := func(out []doc.RecordKind, t Trigger) bool {
		if len(out) == 0 {
			return false
		}
		if len(t.Kinds) == 0 {
			return true
		}
		for _, k := range out {
			if slices.Contains(t.Kinds, k) {
				return true
			}
		}
		return false
	}
	return feeds(a.produces, b.trigger) || feeds(b.produces, a.trigger)
}

// --- checks -------------------------------------------------------------------

func (p *Program) check(rep *Report) {
	// Overlaps.
	for i, a := range p.rules {
		for _, b := range p.rules[i+1:] {
			if (a.policy == CoreID) != (b.policy == CoreID) {
				continue
			}
			if !p.coincide(a.in, b.in) || !patternsOverlap(a.on, b.on) || !compatible(a, b) {
				continue
			}
			if a.policy == b.policy {
				rep.errorf("rules %s and %s overlap at %s", a.id(), b.id(), a.in)
				continue
			}
			if !p.ordered(a, b) && !p.ordered(b, a) {
				rep.errorf("rules %s and %s overlap at %s; the manifest must order them", a.id(), b.id(), a.in)
			}
		}
	}
	// Shadowing: b can never win against an earlier a at the same scope
	// with a pattern covering b's and a weaker signature.
	for _, rs := range p.table {
		for i, a := range rs {
			for _, b := range rs[i+1:] {
				if (a.policy == CoreID) == (b.policy == CoreID) && a.in == b.in && covers(a.on, b.on) && a.required.subset(b.required) && a.forbidden.subset(b.forbidden) && a.typ == ruleHandle {
					rep.Shadowed = append(rep.Shadowed, fmt.Sprintf("%s by %s", b.id(), a.id()))
				}
			}
		}
	}
	// Transform chains acyclic over intent kinds.
	graph := map[vocab.IntentKind][]vocab.IntentKind{}
	for _, r := range p.rules {
		if r.typ != ruleTransform {
			continue
		}
		for _, from := range patternKinds(r.on) {
			for _, to := range r.emits {
				if from != to {
					graph[from] = append(graph[from], to)
				}
			}
		}
	}
	if cyc := findCycle(graph); cyc != "" {
		rep.errorf("transform chains form a cycle: %s", cyc)
	}
	// Precedence acyclic.
	// TODO(policy): qualified orders could form a cycle only under
	// disjoint qualifiers, which is legal; the check is conservative.
	pg := map[string][]string{}
	for _, o := range p.orders {
		if o.over {
			pg[o.a.id] = append(pg[o.a.id], o.b.id)
		}
	}
	if cyc := findCycle(pg); cyc != "" {
		rep.errorf("manifest precedence forms a cycle: %s", cyc)
	}
	for _, o := range p.orders {
		if o.a.id == CoreID {
			rep.errorf("core cannot be ordered over %s", o.b.id)
		}
	}
	// Dead triggers.
	producible := map[doc.RecordKind]bool{doc.RecordSelection: true, doc.RecordView: true}
	for _, r := range p.rules {
		for _, k := range r.produces {
			producible[k] = true
		}
	}
	for _, rs := range p.phases {
		for _, r := range rs {
			for _, k := range r.produces {
				producible[k] = true
			}
		}
	}
	dead := func(id string, t Trigger) {
		if len(t.Kinds) == 0 {
			return
		}
		for _, k := range t.Kinds {
			if producible[k] {
				return
			}
		}
		rep.Dead = append(rep.Dead, id)
	}
	for _, rs := range p.phases {
		for _, r := range rs {
			dead(r.id(), r.trigger)
		}
	}
	for _, o := range p.observers {
		dead(o.id(), o.trigger)
	}
	// Core answers every intent at every scope: a core Handle at the slot
	// or the page, for the bare intent kind, with no required atoms.
	for _, k := range vocab.IntentKinds() {
		ok := false
		for _, s := range p.rootScopes() {
			for _, r := range p.table[tableKey{intent: k, scope: s}] {
				if r.policy == CoreID && r.typ == ruleHandle && r.required.empty() && r.forbidden.empty() && slices.ContainsFunc(r.on, func(pt Pattern) bool { return pt.Kind == k && !pt.Specific() }) {
					ok = true
				}
			}
		}
		if !ok && p.rank[CoreID] >= 0 && slices.ContainsFunc(p.policies, func(x *P) bool { return x.policy.id == CoreID }) {
			rep.errorf("core has no fallback for %s", k)
		}
	}
}

// covers reports whether patterns a match everything patterns b match.
func covers(a, b []Pattern) bool {
	for _, y := range b {
		ok := false
		for _, x := range a {
			if x.Kind == y.Kind && (!x.Specific() || x == y) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func findCycle[K comparable](g map[K][]K) string {
	const (
		white = iota
		grey
		black
	)
	color := map[K]int{}
	var path []K
	var found []K
	var visit func(K) bool
	visit = func(n K) bool {
		color[n] = grey
		path = append(path, n)
		for _, m := range g[n] {
			switch color[m] {
			case grey:
				i := slices.Index(path, m)
				found = append(slices.Clone(path[i:]), m)
				return true
			case white:
				if visit(m) {
					return true
				}
			}
		}
		path = path[:len(path)-1]
		color[n] = black
		return false
	}
	keys := make([]K, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, k := range keys {
		if color[k] == white && visit(k) {
			parts := make([]string, len(found))
			for i, f := range found {
				parts[i] = fmt.Sprint(f)
			}
			return strings.Join(parts, " -> ")
		}
	}
	return ""
}
