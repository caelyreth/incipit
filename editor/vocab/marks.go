package vocab

import (
	"errors"

	"github.com/caelyreth/neverland/editor/doc"
)

// The semantic helpers. Each returns an Edit made of primitive
// operations; policies edit only through helpers and primitives.
//
// A grid's edits are in grid.go. TODO(tables): fill cells from a grid
// slice (§5.2). A figure's and a table's shape name their kind, so they
// are the profile's (NewFigure, SetShape, TableShape, InsertColumns).

// ErrHelper reports a helper asked for something the document cannot do.
var ErrHelper = errors.New("vocab: the edit does not fit the document")

// --- marks ------------------------------------------------------------

// Coverage is how much of a range carries a mark.
type Coverage uint8

const (
	CoverNone Coverage = iota
	CoverPartial
	CoverFull
)

// TextSpans returns the stretches of text slots from from to to, in
// reading order.
func TextSpans(r doc.Reader, from, to doc.Pos) []Span {
	from, to = textBound(r, from, false), textBound(r, to, true)
	if doc.Compare(r, doc.CaretAt(to), doc.CaretAt(from)) < 0 {
		return nil
	}
	if from.Ref() == to.Ref() {
		return []Span{{Slot: from.Ref(), From: min(from.Offset, to.Offset), To: max(from.Offset, to.Offset)}}
	}
	var out []Span
	ref := from.Ref()
	start := from.Offset
	for {
		n, ok := r.Node(ref.Node)
		if !ok {
			return out
		}
		if ref == to.Ref() {
			out = append(out, Span{Slot: ref, From: start, To: to.Offset})
			return out
		}
		out = append(out, Span{Slot: ref, From: start, To: n.Slot(ref.Slot).Len()})
		next, ok := NextTextSlot(r, ref)
		if !ok {
			return out
		}
		ref, start = next, 0
	}
}

// textBound is the text position a range's end at a block gap stands
// for: the first text after the gap for a start, the last before it for an
// end. A text position is its own.
func textBound(r doc.Reader, p doc.Pos, end bool) doc.Pos {
	n, ok := r.Node(p.Node)
	if !ok || p.Slot >= n.NumSlots() || n.Slot(p.Slot).IsInline() {
		return p
	}
	s := n.Slot(p.Slot)
	if !end {
		for _, c := range s.Children()[min(p.Offset, s.Len()):] {
			if ref, ok := FirstTextSlot(c); ok {
				return doc.At(ref, 0)
			}
		}
		if ref, ok := NextTextSlot(r, doc.SlotRef{Node: p.Node, Slot: p.Slot}); ok {
			return doc.At(ref, 0)
		}
		return p
	}
	children := s.Children()[:min(max(p.Offset, 0), s.Len())]
	for i := len(children) - 1; i >= 0; i-- {
		if ref, ok := LastTextSlot(children[i]); ok {
			tn, _ := r.Node(ref.Node)
			return doc.At(ref, tn.Slot(ref.Slot).Len())
		}
	}
	return p
}

// CoverageOf is how much of spans carries a mark of kind k. Inline nodes
// do not count.
func CoverageOf(r doc.Reader, spans []Span, k doc.MarkKind) Coverage {
	with, without := 0, 0
	for _, s := range spans {
		n, ok := r.Node(s.Slot.Node)
		if !ok {
			continue
		}
		for _, it := range n.Slot(s.Slot.Slot).Content().Slice(s.From, s.To) {
			if it.Node != nil {
				continue
			}
			if it.Marks.Has(k) {
				with += len(it.Text)
			} else {
				without += len(it.Text)
			}
		}
	}
	switch {
	case with == 0:
		return CoverNone
	case without == 0:
		return CoverFull
	}
	return CoverPartial
}

// ApplyMark adds m over spans, skipping literal slots and slots that do
// not take it. Applying a mark never nests it.
func ApplyMark(spans []Span, m doc.Mark) Edit {
	return func(tx *doc.Tx) error {
		for _, s := range spans {
			spec, ok := SlotSpec(tx, s.Slot)
			if !ok || spec.Literal || spec.Marks == nil || !spec.Marks(m.Kind) || s.From == s.To {
				continue
			}
			op := doc.MarkText{At: s.Slot, From: s.From, To: s.To, Add: []doc.Mark{m}, Remove: excludedBy(tx.Schema(), m.Kind)}
			if err := tx.Apply(op); err != nil {
				return err
			}
		}
		return nil
	}
}

// InheritedMarks are the marks text typed at off takes where no ghost
// notation or pending marks decide: those of the text before it. Bounded
// marks are not extended past their end, so typing after inline code
// leaves it.
func InheritedMarks(s *doc.Schema, in *doc.Inline, off int) *doc.MarkSet {
	ms := in.MarksAt(off)
	if ms == nil || off == 0 {
		return ms
	}
	after := in.MarksAt(min(off+1, in.Len()))
	for _, m := range ms.Marks() {
		if spec, ok := s.Mark(m.Kind); ok && spec.Bounded && (off == in.Len() || after == nil || !after.Has(m.Kind)) {
			ms = ms.Without(m.Kind)
		}
	}
	return ms
}

// Excludes reports whether marks a and b may not share a set: either one
// excludes the other.
func Excludes(s *doc.Schema, a, b doc.MarkKind) bool {
	if a == b {
		return false
	}
	for _, p := range [][2]doc.MarkKind{{a, b}, {b, a}} {
		if spec, ok := s.Mark(p[0]); ok && spec.Excludes != nil && spec.Excludes(p[1]) {
			return true
		}
	}
	return false
}

// excludedBy are the marks adding k removes.
func excludedBy(s *doc.Schema, k doc.MarkKind) []doc.MarkKind {
	var out []doc.MarkKind
	for _, other := range s.MarkKinds() {
		if Excludes(s, k, other) {
			out = append(out, other)
		}
	}
	return out
}

// WithMark is set with m added and the marks m excludes removed: the
// newest mark wins.
func WithMark(s *doc.Schema, set *doc.MarkSet, m doc.Mark) *doc.MarkSet {
	for _, other := range excludedBy(s, m.Kind) {
		set = set.Without(other)
	}
	return set.With(m)
}

// ClearMark removes marks of kind k over spans.
func ClearMark(spans []Span, k doc.MarkKind) Edit {
	return func(tx *doc.Tx) error {
		for _, s := range spans {
			if s.From == s.To {
				continue
			}
			if err := tx.Apply(doc.MarkText{At: s.Slot, From: s.From, To: s.To, Remove: []doc.MarkKind{k}}); err != nil {
				return err
			}
		}
		return nil
	}
}
