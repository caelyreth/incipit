package engine

import (
	"slices"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
)

// sites are the places a rewrite visits: from the records of the first
// visible steps (the handler's and earlier phases'), each mapped through
// every step after its own to the transaction's current state, widened to
// the rewrite's extent and merged. A site is visited at most once per
// transaction.
func sites(r doc.Reader, rw policy.Rewriter, steps []*doc.Step, visible int) []policy.Site {
	t := rw.Trigger()
	reader := r
	var out []policy.Site
	for i, st := range steps[:visible] {
		later := doc.Steps(steps[i+1:])
		for _, r := range st.Records() {
			r := r
			if !t.Matches(r) || !textual(r) {
				continue
			}
			from, ok1 := later.Map(doc.At(r.Slot, r.From), doc.BiasBefore)
			to, ok2 := later.Map(doc.At(r.Slot, recordEnd(r)), doc.BiasAfter)
			if !ok1 || !ok2 || from.Ref() != to.Ref() {
				continue
			}
			site := policy.Site{Slot: from.Ref(), From: from.Offset, To: max(from.Offset, to.Offset), Records: []doc.Record{r}}
			out = merge(out, expand(reader, site, rw.Extent()), rw.Extent())
		}
	}
	return out
}

// textual reports whether a record names a slot range.
func textual(r doc.Record) bool {
	switch r.Kind {
	case doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks, doc.RecordTransferred:
		return true
	}
	return false
}

func recordEnd(r doc.Record) int {
	if r.Kind == doc.RecordTextDeleted {
		return r.From
	}
	return r.To
}

// merge adds a site, joining it with one in the same extent.
func merge(sites []policy.Site, s policy.Site, ext policy.Extent) []policy.Site {
	for i, o := range sites {
		if o.Slot != s.Slot {
			continue
		}
		join := ext == policy.TouchedSlots || s.From <= o.To && o.From <= s.To
		if join {
			o.From, o.To = min(o.From, s.From), max(o.To, s.To)
			o.Records = append(slices.Clip(o.Records), s.Records...)
			sites[i] = o
			return sites
		}
	}
	return append(sites, s)
}

// expand widens a site to its extent: the lines it touches, or its slot.
func expand(r doc.Reader, s policy.Site, ext policy.Extent) policy.Site {
	if ext == policy.ChangedRanges {
		return s
	}
	n, ok := r.Node(s.Slot.Node)
	if !ok || s.Slot.Slot >= n.NumSlots() || !n.Slot(s.Slot.Slot).IsInline() {
		return s
	}
	text := n.Slot(s.Slot.Slot).Content().Text()
	if ext == policy.TouchedSlots {
		s.From, s.To = 0, len(text)
		return s
	}
	for s.From > 0 && text[s.From-1] != '\n' {
		s.From--
	}
	for s.To < len(text) && text[s.To] != '\n' {
		s.To++
	}
	return s
}

// remap maps a site through steps applied since it was computed, as the
// rewrite's own earlier sites edited the slot.
func remap(s policy.Site, steps doc.Steps) (policy.Site, bool) {
	if len(steps) == 0 {
		return s, true
	}
	from, ok1 := steps.Map(doc.At(s.Slot, s.From), doc.BiasBefore)
	to, ok2 := steps.Map(doc.At(s.Slot, s.To), doc.BiasAfter)
	if !ok1 || !ok2 || from.Ref() != to.Ref() {
		return s, false
	}
	s.Slot, s.From, s.To = from.Ref(), from.Offset, max(from.Offset, to.Offset)
	return s, true
}
