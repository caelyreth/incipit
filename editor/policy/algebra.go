package policy

import (
	"fmt"
	"sync"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// The rule algebra: fact and annotation handles, scope keys, relations
// and intent patterns. Handles' names exist only for traces and compile
// messages.

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

// The core annotation kinds, which any policy may create.
var (
	// Armed marks a structure the first delete armed.
	Armed = NewAnnotationKind("armed", true)
	// PendingMarks holds marks the next typed text takes.
	PendingMarks = NewAnnotationKind("pendingMarks", false)
)

// ScopeClass is the class of a scope key.
type ScopeClass uint8

const (
	// ScopeNode is a node of a kind: the page, a container, the caret's
	// node.
	ScopeNode ScopeClass = iota + 1
	// ScopeSlot is the caret's slot, whatever node holds it.
	ScopeSlot
	// ScopeFact is a fact span of a kind containing the caret.
	ScopeFact
	// ScopeRole is any stack entry providing a role.
	ScopeRole
	// ScopeAnnotation is a scoped annotation containing the caret.
	ScopeAnnotation
	// ScopeParent, for Of, is the scope enclosing the rule's own.
	ScopeParent
	// ScopeRoot is the root node, whatever its kind.
	ScopeRoot
)

// Scope is a scope key: a kind or a role a rule is placed In.
type Scope struct {
	Class ScopeClass
	ID    uint32
}

// Scope keys.
var (
	Slot   = Scope{Class: ScopeSlot}
	Parent = Scope{Class: ScopeParent}
	Root   = Scope{Class: ScopeRoot}
)

func NodeScope(k doc.Kind) Scope             { return Scope{Class: ScopeNode, ID: uint32(k)} }
func FactScope(k FactID) Scope               { return Scope{Class: ScopeFact, ID: uint32(k)} }
func RoleScope(r vocab.Role) Scope           { return Scope{Class: ScopeRole, ID: uint32(r)} }
func AnnotationScope(a AnnotationKind) Scope { return Scope{Class: ScopeAnnotation, ID: a.id} }
func (s Scope) Is(class ScopeClass) bool     { return s.Class == class }
func (s Scope) String() string {
	switch s.Class {
	case ScopeNode:
		return fmt.Sprintf("node(%d)", s.ID)
	case ScopeSlot:
		return "slot"
	case ScopeFact:
		return "fact:" + FactID(s.ID).String()
	case ScopeRole:
		return "role:" + vocab.Role(s.ID).String()
	case ScopeAnnotation:
		return "annotation:" + handleName(handleAnnotation, s.ID)
	case ScopeParent:
		return "parent"
	case ScopeRoot:
		return "root"
	}
	return "scope?"
}

// RelationKind is an atom of the closed relation vocabulary.
type RelationKind uint8

const (
	RelAtInnerStart RelationKind = iota + 1
	RelAtInnerEnd
	RelBefore
	RelAfter
	RelHas
	RelCollapsed
	RelTextRange
	RelNodeSelected
	RelCellRange
	RelCovers
	RelIntersects
	RelCuts
	RelEquals
	RelPrevChar
	RelNextChar
	RelSlotStart
	RelSlotEnd
	RelSlotEmpty
	RelComposing
	RelOwnText
)

// Relation is one atom with its parameters, evaluated relative to the
// rule's scope, or to the scope Of names.
type Relation struct {
	Kind       RelationKind
	Target     Scope
	Class      vocab.CharClass
	Annotation AnnotationKind
	Node       doc.Kind
	At         Scope
}

// Of evaluates a structural atom at an enclosing scope.
func (r Relation) Of(s Scope) Relation { r.At = s; return r }

// The parameterless atoms.
var (
	AtInnerStart = Relation{Kind: RelAtInnerStart}
	AtInnerEnd   = Relation{Kind: RelAtInnerEnd}
	Collapsed    = Relation{Kind: RelCollapsed}
	TextRange    = Relation{Kind: RelTextRange}
	CellRange    = Relation{Kind: RelCellRange}
	SlotStart    = Relation{Kind: RelSlotStart}
	SlotEnd      = Relation{Kind: RelSlotEnd}
	SlotEmpty    = Relation{Kind: RelSlotEmpty}
	Composing    = Relation{Kind: RelComposing}
	Equals       = Relation{Kind: RelEquals}
	// OwnText holds when the caret is in the scope node's own text slot,
	// as in an item's head, and not in a block the node holds.
	OwnText = Relation{Kind: RelOwnText}
)

// Before holds when a fact, node or ghost of s is right after the caret,
// outside it; After when one is right before it.
func Before(s Scope) Relation { return Relation{Kind: RelBefore, Target: s} }
func After(s Scope) Relation  { return Relation{Kind: RelAfter, Target: s} }

// Has holds when an annotation of kind a covers or touches the caret.
func Has(a AnnotationKind) Relation { return Relation{Kind: RelHas, Annotation: a} }

// NodeSelected holds when a node of kind k is selected whole.
func NodeSelected(k doc.Kind) Relation { return Relation{Kind: RelNodeSelected, Node: k} }

// Covers, Intersects and Cuts relate the selection to a span of s: it
// contains it, overlaps it, or contains exactly one of its sides.
func Covers(s Scope) Relation     { return Relation{Kind: RelCovers, Target: s} }
func Intersects(s Scope) Relation { return Relation{Kind: RelIntersects, Target: s} }
func Cuts(s Scope) Relation       { return Relation{Kind: RelCuts, Target: s} }

// PrevChar and NextChar test the class of the neighbouring grapheme.
func PrevChar(c vocab.CharClass) Relation { return Relation{Kind: RelPrevChar, Class: c} }
func NextChar(c vocab.CharClass) Relation { return Relation{Kind: RelNextChar, Class: c} }

func (r Relation) String() string {
	names := [...]string{"", "AtInnerStart", "AtInnerEnd", "Before", "After", "Has", "Collapsed", "TextRange",
		"NodeSelected", "CellRange", "Covers", "Intersects", "Cuts", "Equals", "PrevChar", "NextChar",
		"SlotStart", "SlotEnd", "SlotEmpty", "Composing", "OwnText"}
	s := names[r.Kind]
	switch r.Kind {
	case RelBefore, RelAfter, RelCovers, RelIntersects, RelCuts:
		s += "(" + r.Target.String() + ")"
	case RelHas:
		s += "(" + r.Annotation.String() + ")"
	case RelPrevChar, RelNextChar:
		s += fmt.Sprintf("(%#x)", r.Class)
	case RelNodeSelected:
		s += fmt.Sprintf("(%d)", r.Node)
	}
	if r.At.Class != 0 {
		s += ".Of(" + r.At.String() + ")"
	}
	return s
}

// Pattern matches an intent kind and, optionally, its parameters: an
// exact text or a character class for Type, a unit and direction for
// Delete and Move.
type Pattern struct {
	Kind  vocab.IntentKind
	Text  string
	Class vocab.CharClass
	Unit  vocab.Unit
	Dir   vocab.Direction
}

// On matches every intent of kind k.
func On(k vocab.IntentKind) Pattern { return Pattern{Kind: k} }

// Typed matches typing exactly text.
func Typed(text string) Pattern { return Pattern{Kind: vocab.IntentType, Text: text} }

// TypedClass matches typing one character of class c.
func TypedClass(c vocab.CharClass) Pattern { return Pattern{Kind: vocab.IntentType, Class: c} }

// Deleting matches deleting by unit in dir; a zero unit or direction
// matches any.
func Deleting(u vocab.Unit, d vocab.Direction) Pattern {
	return Pattern{Kind: vocab.IntentDelete, Unit: u, Dir: d}
}

// Moving matches a logical motion by unit in dir.
func Moving(u vocab.Unit, d vocab.Direction) Pattern {
	return Pattern{Kind: vocab.IntentMove, Unit: u, Dir: d}
}

// Specific reports whether the pattern names parameters, which ranks it
// above a pattern of the same kind that names none.
func (p Pattern) Specific() bool { return p.Text != "" || p.Class != 0 || p.Unit != 0 || p.Dir != 0 }

// Match reports whether in matches the pattern.
func (p Pattern) Match(in vocab.Intent) bool {
	if in.Kind() != p.Kind {
		return false
	}
	switch v := in.(type) {
	case vocab.Type:
		if p.Text != "" && v.Text != p.Text {
			return false
		}
		if p.Class != 0 {
			r, n := vocab.FirstRune(v.Text)
			if n != len(v.Text) || vocab.ClassOf(r)&p.Class == 0 {
				return false
			}
		}
	case vocab.Delete:
		return (p.Unit == 0 || p.Unit == v.Unit) && (p.Dir == 0 || p.Dir == v.Dir)
	case vocab.Move:
		return (p.Unit == 0 || p.Unit == v.Unit) && (p.Dir == 0 || p.Dir == v.Dir)
	}
	return true
}

// Overlaps reports whether some intent matches both patterns.
func (p Pattern) Overlaps(o Pattern) bool {
	if p.Kind != o.Kind {
		return false
	}
	if p.Text != "" && o.Text != "" && p.Text != o.Text {
		return false
	}
	if p.Text != "" && o.Class != 0 {
		r, n := vocab.FirstRune(p.Text)
		return n == len(p.Text) && vocab.ClassOf(r)&o.Class != 0
	}
	if o.Text != "" && p.Class != 0 {
		return o.Overlaps(p)
	}
	if p.Class != 0 && o.Class != 0 && p.Class&o.Class == 0 {
		return false
	}
	if p.Unit != 0 && o.Unit != 0 && p.Unit != o.Unit {
		return false
	}
	if p.Dir != 0 && o.Dir != 0 && p.Dir != o.Dir {
		return false
	}
	return true
}
