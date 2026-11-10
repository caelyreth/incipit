package vocab

import (
	"fmt"
	"sync"

	"github.com/caelyreth/neverland/editor/doc"
)

// Handles: fact kinds, annotation kinds and highlight groups are typed
// handles. Their names exist only for traces and compile messages.

type handleClass uint8

const (
	handleFact handleClass = iota + 1
	handleAnnotation
	handleGroup
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

// FactKind names a kind of fact an analyzer emits.
type FactKind uint32

// NewFactKind allocates a fact kind; the policy framework calls it.
func NewFactKind(name string) FactKind { return FactKind(newHandle(handleFact, name)) }

func (k FactKind) String() string { return handleName(handleFact, uint32(k)) }

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

// Role is a shared interface that fact kinds and node kinds provide.
// Rules match on roles, never on providers.
type Role uint8

const (
	RolePair Role = iota + 1
	RoleLeveled
	RoleReshapeable
	RoleBox
	RoleLeavable
	RoleStructure
	RoleItem
)

var roleNames = [...]string{"", "Pair", "Leveled", "Reshapeable", "Box", "Leavable", "Structure", "Item"}

func (r Role) String() string { return roleNames[r] }

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
)

func NodeScope(k doc.Kind) Scope             { return Scope{Class: ScopeNode, ID: uint32(k)} }
func FactScope(k FactKind) Scope             { return Scope{Class: ScopeFact, ID: uint32(k)} }
func RoleScope(r Role) Scope                 { return Scope{Class: ScopeRole, ID: uint32(r)} }
func AnnotationScope(a AnnotationKind) Scope { return Scope{Class: ScopeAnnotation, ID: a.id} }
func (s Scope) Is(class ScopeClass) bool     { return s.Class == class }
func (s Scope) String() string {
	switch s.Class {
	case ScopeNode:
		return "node:" + Schema().KindName(doc.Kind(s.ID))
	case ScopeSlot:
		return "slot"
	case ScopeFact:
		return "fact:" + FactKind(s.ID).String()
	case ScopeRole:
		return "role:" + Role(s.ID).String()
	case ScopeAnnotation:
		return "annotation:" + handleName(handleAnnotation, s.ID)
	case ScopeParent:
		return "parent"
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
	Class      CharClass
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
func PrevChar(c CharClass) Relation { return Relation{Kind: RelPrevChar, Class: c} }
func NextChar(c CharClass) Relation { return Relation{Kind: RelNextChar, Class: c} }

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
		s += "(" + Schema().KindName(r.Node) + ")"
	}
	if r.At.Class != 0 {
		s += ".Of(" + r.At.String() + ")"
	}
	return s
}

// Group is a highlight group: how text, notation, objects and decorations
// are styled. A group may link to a fallback.
type Group uint32

var groupFallback = struct {
	sync.Mutex
	of map[Group]Group
}{of: map[Group]Group{}}

// NewGroup declares a group linked to fallback (zero for none).
func NewGroup(name string, fallback Group) Group {
	g := Group(newHandle(handleGroup, name))
	groupFallback.Lock()
	groupFallback.of[g] = fallback
	groupFallback.Unlock()
	return g
}

func (g Group) String() string { return handleName(handleGroup, uint32(g)) }

// Fallback is the group g links to, or zero.
func (g Group) Fallback() Group {
	groupFallback.Lock()
	defer groupFallback.Unlock()
	return groupFallback.of[g]
}

// The vocabulary's highlight groups.
var (
	GroupText     = NewGroup("text", 0)
	GroupHeading  = NewGroup("heading", GroupText)
	GroupHeadings = [7]Group{0,
		NewGroup("heading.1", GroupHeading), NewGroup("heading.2", GroupHeading), NewGroup("heading.3", GroupHeading),
		NewGroup("heading.4", GroupHeading), NewGroup("heading.5", GroupHeading), NewGroup("heading.6", GroupHeading)}
	GroupEmphasis      = NewGroup("mark.emphasis", 0)
	GroupStrong        = NewGroup("mark.strong", 0)
	GroupCodeMark      = NewGroup("mark.code", 0)
	GroupStrike        = NewGroup("mark.strike", 0)
	GroupGhost         = NewGroup("notation.ghost", 0)
	GroupCode          = NewGroup("code", GroupText)
	GroupFormulaSource = NewGroup("formula.source", GroupCode)
	GroupQuote         = NewGroup("quote", GroupText)
	GroupListMarker    = NewGroup("list.marker", GroupGhost)
	GroupCaption       = NewGroup("caption", GroupText)
	GroupCell          = NewGroup("cell", GroupText)
	GroupCellHeader    = NewGroup("cell.header", GroupCell)
	GroupComposition   = NewGroup("composition", 0)
	// GroupArmed styles what the next delete takes, armed by the first.
	GroupArmed = NewGroup("armed", 0)
	// GroupPairMatch styles the pair or mark a caret is inside.
	GroupPairMatch = NewGroup("pair.match", 0)
	// GroupScope styles the innermost textual scope at the caret.
	GroupScope = NewGroup("scope", 0)
	// GroupPulse styles text a rewrite or history changed.
	GroupPulse = NewGroup("feedback.pulse", 0)
)

// MarkGroup is the group of a mark kind.
func MarkGroup(k doc.MarkKind) Group {
	switch k {
	case Emphasis:
		return GroupEmphasis
	case Strong:
		return GroupStrong
	case CodeMark:
		return GroupCodeMark
	case Strike:
		return GroupStrike
	}
	return 0
}

// Phases and origin classes are the document's.
type (
	Phase       = doc.Phase
	OriginClass = doc.OriginClass
)
