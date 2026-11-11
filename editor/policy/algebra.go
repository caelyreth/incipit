package policy

import (
	"fmt"
	"slices"
	"sync"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// The rule surface's vocabulary: fact and annotation handles, scope keys,
// intent patterns, subjects and the places in them, and conditions.
// Handles' names exist only for traces and compile messages.

type handleClass uint8

const (
	handleFact handleClass = iota + 1
	handleAnnotation
)

var handles = struct {
	sync.Mutex
	names map[handleClass][]string
}{names: map[handleClass][]string{}}

func newHandle(class handleClass, name string) uint32 {
	handles.Lock()
	defer handles.Unlock()
	handles.names[class] = append(handles.names[class], name)
	return uint32(len(handles.names[class]))
}

func handleName(class handleClass, id uint32) string {
	handles.Lock()
	defer handles.Unlock()
	names := handles.names[class]
	if id == 0 || int(id) > len(names) {
		return fmt.Sprintf("handle(%d,%d)", class, id)
	}
	return names[id-1]
}

// FactID names a kind of fact an analyzer emits; FactKind is its typed
// declaration.
type FactID uint32

func newFactID(name string) FactID { return FactID(newHandle(handleFact, name)) }

func (k FactID) String() string { return handleName(handleFact, uint32(k)) }

// AnnotationKind names a kind of annotation. A scoped kind joins the
// scope stack where its span contains the caret.
type AnnotationKind struct {
	id     uint32
	scoped bool
}

// NewAnnotationKind declares an annotation kind.
func NewAnnotationKind(name string, scoped bool) AnnotationKind {
	return AnnotationKind{id: newHandle(handleAnnotation, name), scoped: scoped}
}

func (a AnnotationKind) ID() uint32     { return a.id }
func (a AnnotationKind) Scoped() bool   { return a.scoped }
func (a AnnotationKind) String() string { return handleName(handleAnnotation, a.id) }

// Armed is the core annotation kind any policy may create: a structure
// the first delete armed.
var Armed = NewAnnotationKind("armed", true)

// ScopeClass is the class of a scope key.
type ScopeClass uint8

const (
	// ScopeNode is a node of a kind.
	ScopeNode ScopeClass = iota + 1
	// ScopeSlot is the caret's slot, whatever node holds it.
	ScopeSlot
	// ScopeFact is a fact of a kind, or of one key of it.
	ScopeFact
	// ScopeRole is any stack entry providing a role.
	ScopeRole
	// ScopeAnnotation is a scoped annotation containing the caret.
	ScopeAnnotation
)

// Scope is a scope key: what a stack level answers to. Key is set for a
// keyed fact kind's facts of one key.
type Scope struct {
	Class ScopeClass
	ID    uint32
	Key   any
}

func nodeScope(k doc.Kind) Scope             { return Scope{Class: ScopeNode, ID: uint32(k)} }
func factScope(k FactID) Scope               { return Scope{Class: ScopeFact, ID: uint32(k)} }
func roleScope(r vocab.Role) Scope           { return Scope{Class: ScopeRole, ID: uint32(r)} }
func annotationScope(a AnnotationKind) Scope { return Scope{Class: ScopeAnnotation, ID: a.id} }

var slotScope = Scope{Class: ScopeSlot}

func (s Scope) String() string {
	switch s.Class {
	case ScopeNode:
		return fmt.Sprintf("node(%d)", s.ID)
	case ScopeSlot:
		return "slot"
	case ScopeFact:
		if s.Key != nil {
			return fmt.Sprintf("fact:%s[%v]", FactID(s.ID), s.Key)
		}
		return "fact:" + FactID(s.ID).String()
	case ScopeRole:
		return "role:" + vocab.Role(s.ID).String()
	case ScopeAnnotation:
		return "annotation:" + handleName(handleAnnotation, s.ID)
	}
	return "scope?"
}

// --- patterns -------------------------------------------------------------

// Pattern matches intents of type I and, optionally, their parameters.
type Pattern[I vocab.Intent] struct{ m matcher }

type matcher struct {
	kind  vocab.IntentKind
	texts []string
	class vocab.CharClass
	unit  vocab.Unit
	dir   vocab.Direction
	// extend is whether the intent extends the selection: +1 it does, -1
	// it does not, 0 either.
	extend int8
	// clicks is a Place's click count; zero for any.
	clicks int
}

// Intent matches every intent of type I.
func Intent[I vocab.Intent]() Pattern[I] {
	var zero I
	return Pattern[I]{matcher{kind: zero.Kind()}}
}

// Kind matches every intent of kind k, bound as an intent.
func Kind(k vocab.IntentKind) Pattern[vocab.Intent] { return Pattern[vocab.Intent]{matcher{kind: k}} }

// Typed matches typing exactly one of texts.
func Typed(texts ...string) Pattern[vocab.Type] {
	return Pattern[vocab.Type]{matcher{kind: vocab.IntentType, texts: texts}}
}

// TypedClass matches typed text whose first character is of class c,
// however much text arrives at once.
func TypedClass(c vocab.CharClass) Pattern[vocab.Type] {
	return Pattern[vocab.Type]{matcher{kind: vocab.IntentType, class: c}}
}

// Deleting matches deleting by unit in dir; a zero unit or direction
// matches any.
func Deleting(u vocab.Unit, d vocab.Direction) Pattern[vocab.Delete] {
	return Pattern[vocab.Delete]{matcher{kind: vocab.IntentDelete, unit: u, dir: d}}
}

// Moving matches a logical motion by unit in dir that moves the caret,
// not extending the selection; a zero unit or direction matches any.
func Moving(u vocab.Unit, d vocab.Direction) Pattern[vocab.Move] {
	return Pattern[vocab.Move]{matcher{kind: vocab.IntentMove, unit: u, dir: d, extend: -1}}
}

// Extending matches a logical motion by unit in dir that extends the
// selection.
func Extending(u vocab.Unit, d vocab.Direction) Pattern[vocab.Move] {
	return Pattern[vocab.Move]{matcher{kind: vocab.IntentMove, unit: u, dir: d, extend: 1}}
}

// Clicking matches placing the caret with n clicks, not extending: a click
// that extends extends, whatever its count.
func Clicking(n int) Pattern[vocab.Place] {
	return Pattern[vocab.Place]{matcher{kind: vocab.IntentPlace, clicks: n, extend: -1}}
}

func (m matcher) bare() bool {
	return m.texts == nil && m.class == 0 && m.unit == 0 && m.dir == 0 && m.extend == 0 && m.clicks == 0
}

// extends reports whether the matcher takes an intent's extend flag.
func (m matcher) extends(extend bool) bool {
	return m.extend == 0 || (m.extend > 0) == extend
}

func (m matcher) match(in vocab.Intent) bool {
	if in.Kind() != m.kind {
		return false
	}
	switch v := in.(type) {
	case vocab.Type:
		if m.texts != nil && !slices.Contains(m.texts, v.Text) {
			return false
		}
		return m.class == 0 || vocab.FirstClass(v.Text)&m.class != 0
	case vocab.Delete:
		return (m.unit == 0 || m.unit == v.Unit) && (m.dir == 0 || m.dir == v.Dir)
	case vocab.Move:
		return (m.unit == 0 || m.unit == v.Unit) && (m.dir == 0 || m.dir == v.Dir) && m.extends(v.Extend)
	case vocab.Place:
		return (m.clicks == 0 || m.clicks == v.Clicks) && m.extends(v.Extend)
	}
	return true
}

// --- subjects and places ----------------------------------------------------

// Subject is something a rule stands in or beside: the caret's slot, a
// node kind, a role, or a fact kind. T is what a match binds: the slot,
// the node, the role's implementation, the fact's data.
type Subject[T any] struct {
	scope Scope
	bind  func(c *Ctx, l *Level) (T, bool)
}

// Slot is the caret's slot.
var Slot = Subject[doc.SlotRef]{scope: slotScope, bind: func(_ *Ctx, l *Level) (doc.SlotRef, bool) { return l.Slot, true }}

// Node is a node of kind k.
func Node(k doc.Kind) Subject[*doc.Node] {
	return Subject[*doc.Node]{scope: nodeScope(k), bind: func(_ *Ctx, l *Level) (*doc.Node, bool) { return l.Node, l.Node != nil }}
}

// Role is whatever provides role r, bound as its implementation R.
func Role[R any](r vocab.Role) Subject[R] {
	return Subject[R]{scope: roleScope(r), bind: func(c *Ctx, l *Level) (R, bool) {
		v, ok := l.Role(c.r, r)
		impl, iok := v.(R)
		return impl, ok && iok
	}}
}

// zone is where the caret stands in a level, between its inner edges.
type zone uint8

const (
	zoneStart  zone = 1 << iota // at the start, not the end
	zoneEnd                     // at the end, not the start
	zoneEmpty                   // at both: nothing between them
	zoneWithin                  // at neither
	zoneAll    = zoneStart | zoneEnd | zoneEmpty | zoneWithin
)

type placeHow uint8

const (
	placeIn placeHow = iota + 1
	placeAhead
	placeBehind
	placeSelected
)

// alt is one place a rule stands: in a subject's level at some of its
// zones, beside it, or with it selected.
type alt struct {
	scope Scope
	how   placeHow
	zones zone
	// own narrows a node's or slot's edges to its own text slot, which the
	// caret must be in: an item's head, not a block it holds.
	own bool
}

// index is the scope the place is found at: beside a subject, the slot.
func (a alt) index() Scope {
	if a.how == placeAhead || a.how == placeBehind {
		return slotScope
	}
	return a.scope
}

// Place is where a rule stands, and binds a T there.
type Place[T any] struct {
	alts []alt
	bind func(c *Ctx, l *Level) (T, bool)
}

func (s Subject[T]) place(a alt) Place[T] {
	a.scope = s.scope
	return Place[T]{alts: []alt{a}, bind: s.bind}
}

// Inside is anywhere inside the subject, its inner edges included.
func (s Subject[T]) Inside() Place[T] { return s.place(alt{how: placeIn, zones: zoneAll}) }

// AtStart is at the subject's inner start, with something before its end;
// AtEnd at its inner end, with something after its start; Empty at both,
// nothing inside; Within at neither.
func (s Subject[T]) AtStart() Place[T] { return s.place(alt{how: placeIn, zones: zoneStart}) }
func (s Subject[T]) AtEnd() Place[T]   { return s.place(alt{how: placeIn, zones: zoneEnd}) }
func (s Subject[T]) Empty() Place[T]   { return s.place(alt{how: placeIn, zones: zoneEmpty}) }
func (s Subject[T]) Within() Place[T]  { return s.place(alt{how: placeIn, zones: zoneWithin}) }

// Own is inside the subject's own text: a node's text slot, not a block it
// holds; an inline slot, not a gap between blocks.
func (s Subject[T]) Own() Place[T] { return s.Inside().Own() }

// Ahead is with the subject right after the caret, outside it, nothing
// between them; Behind with it right before.
func (s Subject[T]) Ahead() Place[T]  { return s.place(alt{how: placeAhead}) }
func (s Subject[T]) Behind() Place[T] { return s.place(alt{how: placeBehind}) }

// Selected is with the subject selected whole: a node selection of it, or
// a text range from its inner start to its inner end.
func (s Subject[T]) Selected() Place[T] { return s.place(alt{how: placeSelected}) }

// Own narrows the place's edges to the subject's own text slot.
func (p Place[T]) Own() Place[T] {
	alts := slices.Clone(p.alts)
	for i := range alts {
		alts[i].own = true
	}
	return Place[T]{alts: alts, bind: p.bind}
}

// Or is wherever any of places is.
func Or[T any](places ...Place[T]) Place[T] {
	out := Place[T]{bind: places[0].bind}
	for _, p := range places {
		out.alts = append(out.alts, p.alts...)
	}
	return out
}

// --- conditions ---------------------------------------------------------------

// Cond is a condition at the caret a rule may add to its place.
type Cond struct {
	test func(c *Ctx) bool
	// scopes are the subjects the condition reads beside the caret.
	scopes []Scope
}

// PrevChar and NextChar test the class of the neighbouring grapheme.
func PrevChar(cl vocab.CharClass) Cond { return charCond(cl, true) }
func NextChar(cl vocab.CharClass) Cond { return charCond(cl, false) }

func charCond(cl vocab.CharClass, before bool) Cond {
	return Cond{test: func(c *Ctx) bool {
		pos := c.Head().Pos
		in, ok := c.Inline(pos.Ref())
		if !ok {
			return false
		}
		class, _ := vocab.GraphemeClassAt(in.Flat(), pos.Offset, before)
		return class&cl != 0
	}}
}

// Has holds when an annotation of kind a covers or touches the caret.
func Has(a AnnotationKind) Cond {
	return Cond{test: func(c *Ctx) bool { return len(c.view.Annotations.At(a, c.Head())) > 0 }}
}

// subject is any Subject, for conditions on what stands beside the caret.
type subject interface{ key() Scope }

func (s Subject[T]) key() Scope { return s.scope }

// Ahead holds when s stands right after the caret; Behind right before.
func Ahead(s subject) Cond  { return besideCond(s.key(), true) }
func Behind(s subject) Cond { return besideCond(s.key(), false) }

func besideCond(s Scope, ahead bool) Cond {
	return Cond{scopes: []Scope{s}, test: func(c *Ctx) bool {
		_, ok := c.prog.besideLevel(c, c.Head(), s, ahead)
		return ok
	}}
}

// Not negates a condition.
func Not(cond Cond) Cond {
	return Cond{scopes: cond.scopes, test: func(c *Ctx) bool { return !cond.test(c) }}
}
