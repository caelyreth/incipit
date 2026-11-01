package projection

import "github.com/caelyreth/neverland/engine/model"

func (p *Projection) ProjectAnchor(anchor model.TextAnchor) (Target, error) {
	if anchor.Revision() != p.version.SourceRevision() {
		return Target{}, &ProjectionError{
			Kind:     ErrorSourceRevision,
			Expected: p.version.SourceRevision(),
			Actual:   anchor.Revision(),
		}
	}
	offset := int(anchor.OffsetUTF16())
	type candidate struct {
		index    int
		segment  Segment
		semantic model.UTF16Range
	}
	candidates := make([]candidate, 0, 2)
	for index, segment := range p.segments {
		if !segment.Editable() {
			continue
		}
		target, ok := segment.SemanticTarget()
		if !ok || target.Node() != anchor.Node() {
			continue
		}
		semantic := target.RangeUTF16()
		if int(semantic.Start) <= offset && offset <= int(semantic.End) {
			candidates = append(candidates, candidate{index: index, segment: segment, semantic: semantic})
		}
	}
	preferred := make([]candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.semantic.Empty() ||
			anchor.Affinity() == model.AffinityLeft && candidate.semantic.Start < model.UTF16Offset(offset) ||
			anchor.Affinity() == model.AffinityRight && model.UTF16Offset(offset) < candidate.semantic.End {
			preferred = append(preferred, candidate)
		}
	}
	empty := make([]candidate, 0, 1)
	for _, candidate := range candidates {
		if candidate.semantic.Empty() {
			empty = append(empty, candidate)
		}
	}
	chosen := candidateByAffinity(empty, anchor.Affinity())
	var err error
	if chosen == nil {
		chosen, err = uniqueCandidate(preferred)
		if err != nil {
			return Target{}, err
		}
	}
	if chosen == nil {
		chosen, err = uniqueCandidate(candidates)
		if err != nil {
			return Target{}, err
		}
	}
	if chosen == nil {
		return Target{}, &ProjectionError{Kind: ErrorNoCorrespondence}
	}
	semantic := chosen.semantic
	interior := int(chosen.segment.range16.Start) + offset - int(semantic.Start)
	projected := interior
	affinity := anchor.Affinity()
	switch {
	case anchor.Affinity() == model.AffinityLeft && offset == int(semantic.Start):
		if exterior, ok := p.generatedExteriorBefore(chosen.index, anchor.Node(), interior); ok {
			projected = exterior
		}
	case anchor.Affinity() == model.AffinityRight && offset == int(semantic.End) && !semantic.Empty():
		if exterior, ok := p.generatedExteriorAfter(chosen.index, anchor.Node(), interior); ok {
			projected = exterior
		}
	default:
		if offset == int(semantic.Start) {
			affinity = model.AffinityRight
		} else if offset == int(semantic.End) {
			affinity = model.AffinityLeft
		}
	}
	target, err := p.Target(model.EmptyUTF16Range(model.UTF16Offset(projected)))
	if err != nil {
		return Target{}, err
	}
	return target.WithAffinity(affinity), nil
}

// candidateByAffinity chooses among zero-width attachments at one semantic
// offset. Projection order is stable; left affinity takes the first, right
// affinity the last. This gives the same anchor a deterministic side when
// several empty marks coexist at one position.
func candidateByAffinity[T any](candidates []T, affinity model.Affinity) *T {
	if len(candidates) == 0 {
		return nil
	}
	index := 0
	if affinity == model.AffinityRight {
		index = len(candidates) - 1
	}
	return &candidates[index]
}

func (p *Projection) SemanticAnchor(
	offset model.UTF16Offset,
	affinity model.Affinity,
) (model.TextAnchor, error) {
	if _, err := p.Target(model.EmptyUTF16Range(offset)); err != nil {
		return model.TextAnchor{}, err
	}
	type bestCandidate struct {
		distance int
		rank     int
		anchor   model.TextAnchor
	}
	owner, owned := p.owner(offset, affinity)
	var best *bestCandidate
	for _, segment := range p.segments {
		target, ok := segment.SemanticTarget()
		if !ok || owned && target.Node() != owner {
			continue
		}
		start, end := int(segment.range16.Start), int(segment.range16.End)
		projected := clamp(int(offset), start, end)
		distance := absInt(int(offset) - projected)
		rank := end
		if affinity == model.AffinityLeft {
			rank = int(^uint(0)>>1) - start
		}
		candidate := bestCandidate{
			distance: distance,
			rank:     rank,
			anchor: model.NewTextAnchor(
				target.Node(),
				target.RangeUTF16().Start+model.UTF16Offset(projected-start),
				inferredAffinity(int(offset), start, end, affinity),
				p.version.SourceRevision(),
			),
		}
		if best == nil || candidate.distance < best.distance ||
			candidate.distance == best.distance && candidate.rank < best.rank {
			copy := candidate
			best = &copy
		}
	}
	if best == nil {
		return model.TextAnchor{}, &ProjectionError{Kind: ErrorNoCorrespondence}
	}
	return best.anchor, nil
}

func (p *Projection) generatedExteriorBefore(index int, node model.NodeID, interior int) (int, bool) {
	boundary := interior
	projected := 0
	found := false
	for candidate := index - 1; candidate >= 0; candidate-- {
		segment := p.segments[candidate]
		if int(segment.range16.End) != boundary || !attributeBelongsTo(segment, node) {
			break
		}
		boundary = int(segment.range16.Start)
		projected = boundary
		found = true
	}
	return projected, found
}

func (p *Projection) generatedExteriorAfter(index int, node model.NodeID, interior int) (int, bool) {
	boundary := interior
	projected := 0
	found := false
	for candidate := index + 1; candidate < len(p.segments); candidate++ {
		segment := p.segments[candidate]
		if int(segment.range16.Start) != boundary || !attributeBelongsTo(segment, node) {
			break
		}
		boundary = int(segment.range16.End)
		projected = boundary
		found = true
	}
	return projected, found
}

func attributeBelongsTo(segment Segment, node model.NodeID) bool {
	target, ok := segment.AttributeTarget()
	return ok && target.Node() == node
}

func inferredAffinity(offset, start, end int, declared model.Affinity) model.Affinity {
	switch {
	case start == end:
		return declared
	case offset < start:
		return model.AffinityLeft
	case end < offset || offset == start:
		return model.AffinityRight
	case offset == end:
		return model.AffinityLeft
	default:
		return declared
	}
}

func uniqueCandidate[T any](candidates []T) (*T, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	if len(candidates) > 1 {
		return nil, &ProjectionError{Kind: ErrorAmbiguousCorrespondence}
	}
	return &candidates[0], nil
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// owner is the node a position belongs to: the one whose text or notation
// holds it, or touches it on the side it leans. Generated text, as the
// break between blocks, belongs to no node, so a position beside it
// belongs to the node touching it on the other side: a line's start is its
// own block's whichever way it leans, never the end of the line before.
func (p *Projection) owner(offset model.UTF16Offset, affinity model.Affinity) (model.NodeID, bool) {
	nodeOf := func(segment Segment) (model.NodeID, bool) {
		if target, ok := segment.SemanticTarget(); ok {
			return target.Node(), true
		}
		if target, ok := segment.AttributeTarget(); ok {
			return target.Node(), true
		}
		return 0, false
	}
	var before, after *Segment
	for index := range p.segments {
		segment := &p.segments[index]
		span := segment.range16
		switch {
		case span.Empty():
		case span.Start < offset && offset < span.End:
			return nodeOf(*segment)
		case span.End == offset:
			before = segment
		case span.Start == offset && after == nil:
			after = segment
		}
	}
	first, second := after, before
	if affinity == model.AffinityLeft {
		first, second = before, after
	}
	for _, segment := range []*Segment{first, second} {
		if segment == nil {
			continue
		}
		if node, ok := nodeOf(*segment); ok {
			return node, true
		}
	}
	return 0, false
}
