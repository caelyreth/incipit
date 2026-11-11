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
// qualifier narrows where: a vocab.Role, a Subject, a vocab.IntentKind, or
// nil for everywhere.
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
	// Strict makes a rewrite's undeclared records errors. Tests set it; a
	// host may.
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
	// Shadowed rules can never run: a rule before them answers wherever
	// they would, unless it passes.
	Shadowed []string
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
	projecting  []FactID
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

// Profile is the document profile the program edits; nil when none.
func (p *Program) Profile() vocab.Profile { return p.profile }

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

// Paints and Decorations are presentation contributions; Projecting are
// the fact kinds whose facts may be glyphs.
func (p *Program) Paints() []Paint           { return p.paints }
func (p *Program) Projecting() []FactID      { return p.projecting }
func (p *Program) Decorations() []Decoration { return p.decorations }
func (p *Program) Guides() []Guide           { return p.guides }
func (p *Program) Strict() bool              { return p.strict }

// HasAnnotationKind reports whether annotations of k survive in this
// program: core kinds always do, others while their owner is enabled.
func (p *Program) HasAnnotationKind(k AnnotationKind) bool {
	switch k {
	case Armed:
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
			if d.projects {
				prog.projecting = append(prog.projecting, d.kind)
			}
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

	// Index rules by the scopes their places are found at.
	scopeKinds := map[FactID]bool{}
	for _, p := range prog.policies {
	rules:
		for _, r := range p.rules {
			var scopes []Scope
			for _, a := range r.alts {
				scopes = append(scopes, a.scope)
			}
			for _, c := range r.conds {
				scopes = append(scopes, c.scopes...)
			}
			for _, s := range scopes {
				if what, ok := needs(s); !ok {
					rep.Dropped = append(rep.Dropped, fmt.Sprintf("%s: nothing produces %s", r.id(), what))
					continue rules
				}
			}
			for _, s := range scopes {
				prog.noteScopeKinds(s, scopeKinds)
			}
			prog.rules = append(prog.rules, r)
			var indexed []Scope
			for _, a := range r.alts {
				if s := a.index(); !slices.Contains(indexed, s) {
					indexed = append(indexed, s)
					key := tableKey{intent: r.match.kind, scope: s}
					prog.table[key] = append(prog.table[key], r)
				}
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

// factKindsProviding are the fact kinds that provide a role.
func (p *Program) factKindsProviding(r vocab.Role) []FactID {
	var out []FactID
	for k, d := range p.factDecls {
		if slices.Contains(d.roles, r) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
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
		s := roleScope(q)
		return p.standsAt(a, s) || p.standsAt(b, s)
	case subject:
		return p.standsAt(a, q.key()) && p.standsAt(b, q.key())
	case vocab.IntentKind:
		return a.match.kind == q && b.match.kind == q
	}
	return false
}

// standsAt reports whether a rule's place can be found at a level
// answering to s.
func (p *Program) standsAt(r *rule, s Scope) bool {
	return slices.ContainsFunc(r.alts, func(a alt) bool { return p.coincide(a.index(), s) })
}

// precedes orders candidates: the manifest's order first, then manifest
// position, then declaration order.
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
	if ra, rb := p.rank[a.policy], p.rank[b.policy]; ra != rb {
		return ra - rb
	}
	return a.order - b.order
}

// overlaps reports whether some intent matches both matchers.
func overlaps(a, b matcher) bool {
	if a.kind != b.kind {
		return false
	}
	if a.texts != nil && b.texts != nil && !slices.ContainsFunc(a.texts, func(t string) bool { return slices.Contains(b.texts, t) }) {
		return false
	}
	if !classed(a.texts, b.class) || !classed(b.texts, a.class) || a.class != 0 && b.class != 0 && a.class&b.class == 0 {
		return false
	}
	return (a.unit == 0 || b.unit == 0 || a.unit == b.unit) && (a.dir == 0 || b.dir == 0 || a.dir == b.dir) &&
		(a.extend == 0 || b.extend == 0 || a.extend == b.extend) && (a.clicks == 0 || b.clicks == 0 || a.clicks == b.clicks)
}

// classed reports whether some of texts (any text, for none) starts in
// class c (any class, for none).
func classed(texts []string, c vocab.CharClass) bool {
	return texts == nil || c == 0 || slices.ContainsFunc(texts, func(t string) bool { return vocab.FirstClass(t)&c != 0 })
}

// subsumes reports whether every intent b matches, a matches.
func subsumes(a, b matcher) bool {
	if a.kind != b.kind || a.unit != 0 && a.unit != b.unit || a.dir != 0 && a.dir != b.dir ||
		a.extend != 0 && a.extend != b.extend || a.clicks != 0 && a.clicks != b.clicks {
		return false
	}
	if a.texts != nil && (b.texts == nil || slices.ContainsFunc(b.texts, func(t string) bool { return !slices.Contains(a.texts, t) })) {
		return false
	}
	switch {
	case a.class == 0:
		return true
	case b.texts != nil:
		return !slices.ContainsFunc(b.texts, func(t string) bool { return vocab.FirstClass(t)&a.class == 0 })
	}
	return b.class != 0 && b.class&^a.class == 0
}

// meets reports whether two rules can both answer one intent at one level
// and selection: their patterns overlap, their shapes meet, and two of
// their places can hold at one level at once. Places in a level hold at
// once only where their zones meet.
func (p *Program) meets(a, b *rule) bool {
	if !overlaps(a.match, b.match) || a.shapes&b.shapes == 0 {
		return false
	}
	for _, x := range a.alts {
		for _, y := range b.alts {
			if !p.coincide(x.index(), y.index()) {
				continue
			}
			if x.how != placeIn || y.how != placeIn || x.own != y.own || x.zones&y.zones != 0 {
				return true
			}
		}
	}
	return false
}

// shadows reports whether a, with no conditions, answers wherever b
// would: its pattern, shapes and places cover b's.
func (p *Program) shadows(a, b *rule) bool {
	if len(a.conds) > 0 || a.shapes&b.shapes != b.shapes || !subsumes(a.match, b.match) {
		return false
	}
	for _, y := range b.alts {
		if !slices.ContainsFunc(a.alts, func(x alt) bool { return p.altCovers(x, y) }) {
			return false
		}
	}
	return true
}

// altCovers reports whether place x holds wherever place y does.
func (p *Program) altCovers(x, y alt) bool {
	if !p.scopeCovers(x.index(), y.index()) {
		return false
	}
	if x.how == placeIn && x.zones == zoneAll && !x.own {
		// Anywhere inside: wherever the level is.
		return true
	}
	switch {
	case x.how != y.how:
		return false
	case x.how == placeIn:
		return x.own == y.own && x.zones&y.zones == y.zones
	case x.how == placeAhead, x.how == placeBehind:
		return p.scopeCovers(x.scope, y.scope)
	}
	return true
}

// scopeCovers reports whether every level answering to y answers to x.
func (p *Program) scopeCovers(x, y Scope) bool {
	if x == y || x.Class == y.Class && x.ID == y.ID && x.Key == nil {
		return true
	}
	if x.Class != ScopeRole {
		return false
	}
	role := vocab.Role(x.ID)
	switch y.Class {
	case ScopeNode:
		return slices.Contains(p.nodeRoles(doc.Kind(y.ID)), role)
	case ScopeFact:
		d, ok := p.factDecls[FactID(y.ID)]
		return ok && slices.Contains(d.roles, role)
	}
	return false
}

// coincide reports whether two scope keys can name the same stack level.
func (p *Program) coincide(a, b Scope) bool {
	if a.Class == b.Class && a.ID == b.ID && (a.Key == nil || b.Key == nil || a.Key == b.Key) {
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
	// Rules of two policies that can answer one intent at one level need
	// the manifest's order; within a policy, declaration order decides.
	for i, a := range p.rules {
		for _, b := range p.rules[i+1:] {
			if a.policy == b.policy || (a.policy == CoreID) != (b.policy == CoreID) || !p.meets(a, b) {
				continue
			}
			if !p.ordered(a, b) && !p.ordered(b, a) {
				rep.errorf("rules %s and %s overlap; the manifest must order them", a.id(), b.id())
			}
		}
	}
	// A rule after one that answers wherever it would never runs.
	for _, rs := range p.table {
		for i, a := range rs {
			for _, b := range rs[i+1:] {
				if (a.policy == CoreID) == (b.policy == CoreID) && p.shadows(a, b) {
					if s := fmt.Sprintf("%s by %s", b.id(), a.id()); !slices.Contains(rep.Shadowed, s) {
						rep.Shadowed = append(rep.Shadowed, s)
					}
				}
			}
		}
	}
	slices.Sort(rep.Shadowed)
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
	// Core answers every intent everywhere: a core rule inside the slot,
	// for the bare intent kind, at every selection, with no conditions.
	if !slices.ContainsFunc(p.policies, func(x *P) bool { return x.policy.id == CoreID }) {
		return
	}
	for _, k := range vocab.IntentKinds() {
		ok := slices.ContainsFunc(p.table[tableKey{intent: k, scope: slotScope}], func(r *rule) bool {
			return r.policy == CoreID && r.match.bare() && r.shapes == shapeAny && len(r.conds) == 0 &&
				slices.ContainsFunc(r.alts, func(a alt) bool { return a.how == placeIn && a.zones == zoneAll && !a.own })
		})
		if !ok {
			rep.errorf("core has no fallback for %s", k)
		}
	}
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
