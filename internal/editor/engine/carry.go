package engine

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
)

// around is what stood at a place's position before an edit: its first
// and last stop, and whether it was its slot's end.
type around struct {
	first, last int
	end         bool
}

// noted is what stood around each place a view holds, read before an
// edit, so the edit can carry each place by the content beside it.
type noted map[doc.Pos]around

// note reads what stands around the view's places in r, before an edit.
func (e *Engine) note(viewID uint64, r doc.Reader, v *policy.View) noted {
	stops := e.stopsFor(viewID)(r, v)
	n := noted{}
	add := func(c doc.Caret) {
		if _, ok := n[c.Pos]; ok {
			return
		}
		first, last := stops.Around(c.Pos)
		n[c.Pos] = around{first: first, last: last, end: atSlotEnd(r, c.Pos)}
	}
	if v.Selection.Kind == doc.SelectText {
		add(v.Selection.Anchor)
		add(v.Selection.Head)
	}
	for _, c := range v.Annotations.Places() {
		add(c)
	}
	return n
}

// carry carries places through m into r, the document after it (§4.4).
// Projected items move with the content they attach to, so a place keeps
// its side of each by the content beside it: where the content after it
// survives, it keeps its distance from that content; where only the
// content before it does, from that. Content inserted at a place, kept
// after it, leaves it against that content. An edge of content holds
// every stop at its offset wherever it goes.
func (e *Engine) carry(viewID uint64, before noted, r doc.Reader, v *policy.View, m doc.Mapper) doc.Carry {
	var stops policy.Stops
	return func(c doc.Caret, bias doc.Bias) (doc.Caret, bool) {
		at, ok := m.Map(c.Pos, bias)
		was, noted := before[c.Pos]
		if c.Stop == doc.StopFirst || c.Stop == doc.StopLast || !noted {
			c.Pos = at
			return c, ok
		}
		if stops == nil {
			stops = e.stopsFor(viewID)(r, v)
		}
		first, last := stops.Around(at)
		other, _ := m.Map(c.Pos, -bias)
		switch {
		case other != at && bias == doc.BiasAfter:
			// Pushed by content inserted at it: against that content.
			c.Stop = first
		case other == at && (was.end || afterKept(m, c.Pos, at)):
			c.Stop = last - (was.last - c.Stop)
		default:
			c.Stop = first + (c.Stop - was.first)
		}
		c.Pos = at
		return c, ok
	}
}

// afterKept reports whether the content right after a place is still
// right after it once mapped.
func afterKept(m doc.Mapper, from, to doc.Pos) bool {
	next, ok := m.Map(doc.At(from.Ref(), from.Offset+1), doc.BiasBefore)
	return ok && next == doc.At(to.Ref(), to.Offset+1)
}

func atSlotEnd(r doc.Reader, p doc.Pos) bool {
	n, ok := r.Node(p.Node)
	if !ok || p.Slot >= n.NumSlots() {
		return false
	}
	s := n.Slot(p.Slot)
	return s.IsInline() && p.Offset == s.Content().Len()
}
