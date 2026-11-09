package policy

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// AnnotationID identifies an annotation within its view.
type AnnotationID uint64

// Anchor is where an annotation sits: a point, or a span in one slot.
type Anchor struct {
	From, To doc.Pos
}

// PointAnchor anchors at a point.
func PointAnchor(p doc.Pos) Anchor { return Anchor{From: p, To: p} }

// SpanAnchor anchors over a span of one slot.
func SpanAnchor(s vocab.Span) Anchor {
	return Anchor{From: doc.At(s.Slot, s.From), To: doc.At(s.Slot, s.To)}
}

// Span is the anchor as a span.
func (a Anchor) Span() vocab.Span {
	return vocab.Span{Slot: a.From.Ref(), From: a.From.Offset, To: a.To.Offset}
}

// IsPoint reports whether the anchor is a point.
func (a Anchor) IsPoint() bool { return a.From == a.To }

// Touches reports whether p is inside the anchor or at one of its edges.
func (a Anchor) Touches(p doc.Pos) bool {
	return p.Ref() == a.From.Ref() && p.Offset >= a.From.Offset && p.Offset <= a.To.Offset
}

// LifetimeKind is how long an annotation lives.
type LifetimeKind uint8

const (
	// NextIntent: until the next intent has been processed.
	NextIntent LifetimeKind = iota + 1
	// CaretLeaves: until the selection head leaves the lifetime's span.
	CaretLeaves
	// UntilEdit: until a later transaction edits the document.
	UntilEdit
	// Deadline: until the time passes.
	Deadline
	// Transaction: until the transaction that made it ends; observers and
	// rewrites of that transaction see it.
	Transaction
	// ForView: as long as the view.
	ForView
)

// Lifetime is an annotation's lifetime.
type Lifetime struct {
	Kind LifetimeKind
	// Span is the span CaretLeaves watches; empty means the anchor.
	Span *Anchor
	// At is the Deadline time.
	At time.Time
}

// Lifetimes.
var (
	UntilNextIntent = Lifetime{Kind: NextIntent}
	UntilEdited     = Lifetime{Kind: UntilEdit}
	ForTransaction  = Lifetime{Kind: Transaction}
	WithView        = Lifetime{Kind: ForView}
)

// UntilCaretLeaves lives while the selection head stays in span; nil
// watches the anchor.
func UntilCaretLeaves(span *Anchor) Lifetime { return Lifetime{Kind: CaretLeaves, Span: span} }

// Until lives until t.
func Until(t time.Time) Lifetime { return Lifetime{Kind: Deadline, At: t} }

// Annotation is state a rule created that content alone can't reveal. It
// belongs to a view, is mapped through every transaction, and is dropped
// when its lifetime ends or its owner's check rejects it.
type Annotation struct {
	ID       AnnotationID
	Kind     vocab.AnnotationKind
	Owner    string
	Anchor   Anchor
	Data     any
	Origin   uint64
	Lifetime Lifetime
}

// Annotations is a view's immutable annotation set.
type Annotations struct {
	list []Annotation
	next AnnotationID
}

// All returns the annotations in creation order.
func (s Annotations) All() []Annotation { return s.list }

// Len is the number of annotations.
func (s Annotations) Len() int { return len(s.list) }

// Of returns annotations of a kind.
func (s Annotations) Of(k vocab.AnnotationKind) []Annotation {
	var out []Annotation
	for _, a := range s.list {
		if a.Kind == k {
			out = append(out, a)
		}
	}
	return out
}

// At returns annotations of a kind touching p.
func (s Annotations) At(k vocab.AnnotationKind, p doc.Pos) []Annotation {
	var out []Annotation
	for _, a := range s.list {
		if a.Kind == k && a.Anchor.Touches(p) {
			out = append(out, a)
		}
	}
	return out
}

// Get returns an annotation by id.
func (s Annotations) Get(id AnnotationID) (Annotation, bool) {
	for _, a := range s.list {
		if a.ID == id {
			return a, true
		}
	}
	return Annotation{}, false
}

// Owned returns the annotations a policy owns in a slot.
func (s Annotations) Owned(policy string, slot doc.SlotRef) []Annotation {
	var out []Annotation
	for _, a := range s.list {
		if a.Owner == policy && a.Anchor.From.Ref() == slot {
			out = append(out, a)
		}
	}
	return out
}

// Add returns the set with a new annotation, and its id.
func (s Annotations) Add(a Annotation) (Annotations, AnnotationID) {
	s.next++
	a.ID = s.next
	s.list = append(slices.Clip(s.list), a)
	return s, a.ID
}

// Drop returns the set without the annotations keep rejects.
func (s Annotations) Drop(reject func(Annotation) bool) Annotations {
	if !slices.ContainsFunc(s.list, reject) {
		return s
	}
	out := make([]Annotation, 0, len(s.list))
	for _, a := range s.list {
		if !reject(a) {
			out = append(out, a)
		}
	}
	s.list = out
	return s
}

// Map maps every anchor through a transaction's steps. An annotation whose
// anchor was deleted is dropped; a span whose ends cross collapses.
func (s Annotations) Map(m doc.Mapper) Annotations {
	if len(s.list) == 0 {
		return s
	}
	out := make([]Annotation, 0, len(s.list))
	for _, a := range s.list {
		from, ok1 := m.Map(a.Anchor.From, doc.BiasAfter)
		to, ok2 := m.Map(a.Anchor.To, doc.BiasBefore)
		if a.Anchor.IsPoint() {
			to = from
		}
		if !ok1 || !ok2 || from.Ref() != to.Ref() {
			continue
		}
		if to.Offset < from.Offset {
			to = from
		}
		a.Anchor = Anchor{From: from, To: to}
		if a.Lifetime.Span != nil {
			sf, ok1 := m.Map(a.Lifetime.Span.From, doc.BiasAfter)
			st, ok2 := m.Map(a.Lifetime.Span.To, doc.BiasBefore)
			if !ok1 || !ok2 {
				continue
			}
			a.Lifetime.Span = &Anchor{From: sf, To: st}
		}
		out = append(out, a)
	}
	s.list = out
	return s
}

// Settlement is what happened in a transaction, for ending lifetimes.
type Settlement struct {
	// Tx is the transaction's sequence number.
	Tx uint64
	// Edited reports whether it changed the document.
	Edited bool
	// Head is the selection head after it.
	Head doc.Pos
	// Now is the intent's time.
	Now time.Time
	// Kinds are the annotation kinds the program still has.
	Kinds func(vocab.AnnotationKind) bool
}

// Settle ends the lifetimes a finished transaction ends. Annotations the
// transaction itself created survive NextIntent, UntilEdit and
// CaretLeaves tests of that transaction.
func (s Annotations) Settle(st Settlement) Annotations {
	return s.Drop(func(a Annotation) bool {
		if st.Kinds != nil && !st.Kinds(a.Kind) {
			return true
		}
		fresh := a.Origin == st.Tx
		switch a.Lifetime.Kind {
		case Transaction:
			return true
		case NextIntent:
			return !fresh
		case UntilEdit:
			return !fresh && st.Edited
		case CaretLeaves:
			span := a.Anchor
			if a.Lifetime.Span != nil {
				span = *a.Lifetime.Span
			}
			return !span.Touches(st.Head)
		case Deadline:
			return !st.Now.Before(a.Lifetime.At)
		}
		return false
	})
}

// Deadlines returns the deadlines of the set's annotations.
func (s Annotations) Deadlines() []time.Time {
	var out []time.Time
	for _, a := range s.list {
		if a.Lifetime.Kind == Deadline {
			out = append(out, a.Lifetime.At)
		}
	}
	return out
}

// AnnotationCheck is an owner's re-check of its annotation against the
// current document; false drops it.
type AnnotationCheck func(r doc.Reader, a Annotation) bool

type annotationDecl struct {
	kind  vocab.AnnotationKind
	check AnnotationCheck
}

// Annotation declares that the policy creates annotations of kind, and how
// it re-checks them after each transaction. Core kinds from the
// vocabulary may be declared by any policy; the policy that creates an
// annotation owns it.
func (p *P) Annotation(kind vocab.AnnotationKind, check AnnotationCheck) {
	p.annotations = append(p.annotations, annotationDecl{kind: kind, check: check})
}

// Decorate shows annotations of a kind as a view decoration styled by
// group, such as the armed highlight.
func (p *P) Decorate(kind vocab.AnnotationKind, group vocab.Group) {
	p.decorations = append(p.decorations, Decoration{Kind: kind, Group: group})
}

// Decoration is a presentation contribution over annotations.
type Decoration struct {
	Kind  vocab.AnnotationKind
	Group vocab.Group
}
