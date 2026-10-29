package editor

import (
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// A semantic offset at a mark boundary has several projected positions:
// "a|*bc*" and "a*|bc*" are both offset 1, one outside the mark and one
// inside it. Which one the caret occupies decides where typing goes, so the
// caret has exactly one identity, its projected position, and two inverse
// functions over it:
//
//   - caretMembership reads the innermost mark whose generated notation
//     brackets a position: the mark typing there extends.
//   - placeCaret chooses, among the positions of one semantic head, the one
//     whose membership is the requested mark.
//
// For any admissible membership m at head h,
// caretMembership(placeCaret(h, m)) = m, and every collapsed caret the
// session installs comes from placeCaret or from a position the person
// chose. The active mark is never stored apart from the caret, so what the
// surface draws and where a keystroke lands cannot disagree.

// caretMembership is the innermost mark whose opener ends at or before at
// and whose closer starts at or after it.
func caretMembership(
	node *model.Node,
	p *projection.Projection,
	at model.UTF16Offset,
) *model.MarkID {
	if node == nil || p == nil {
		return nil
	}
	type bracket struct{ open, close model.UTF16Offset }
	brackets := make(map[string]*bracket)
	for _, segment := range p.Segments() {
		target, ok := segment.AttributeTarget()
		if !ok || target.Node() != node.ID() {
			continue
		}
		span := segment.ProjectedRange()
		if found, ok := brackets[target.Key()]; ok {
			found.close = span.Start
		} else {
			brackets[target.Key()] = &bracket{open: span.End, close: -1}
		}
	}
	var best *model.InlineMark
	for _, mark := range node.Marks() {
		found, ok := brackets[MarkerKeyForMark(mark.ID())]
		if !ok || found.close < 0 || at < found.open || found.close < at {
			continue
		}
		if best == nil || mark.Range().Len() < best.Range().Len() {
			value := mark
			best = &value
		}
	}
	if best == nil {
		return nil
	}
	id := best.ID()
	return &id
}

// placeCaret projects head to the position whose membership is member. A
// membership head cannot have, such as a mark the edit removed or one that
// does not reach head, yields the projection's own choice for head, whose
// membership then follows from where it lands.
func placeCaret(
	p *projection.Projection,
	node *model.Node,
	head model.TextAnchor,
	member *model.MarkID,
) (projection.Target, error) {
	base, err := p.ProjectAnchor(head)
	if err != nil {
		return projection.Target{}, err
	}
	at := base.RangeUTF16().Start
	if node != nil && node.ID() == head.Node() {
		best, distance := at, -1
		for _, candidate := range headPositions(p, head.Node(), at) {
			if !sameMark(caretMembership(node, p, candidate), member) {
				continue
			}
			if gap := absOffset(candidate - at); distance < 0 || gap < distance {
				best, distance = candidate, gap
			}
		}
		at = best
	}
	return caretTarget(p, at, head.Affinity())
}

// headPositions are the projected positions sharing at's semantic offset:
// at, and every boundary reached by crossing the zero-width generated
// notation of node on either side. Positions within one marker run are
// notation, not caret positions, and are not included.
func headPositions(p *projection.Projection, node model.NodeID, at model.UTF16Offset) []model.UTF16Offset {
	positions := []model.UTF16Offset{at}
	notation := func(segment projection.Segment) bool {
		target, ok := segment.AttributeTarget()
		return ok && target.Node() == node && !segment.ProjectedRange().Empty()
	}
	segments := p.Segments()
	for left := at; ; {
		next := left
		for _, segment := range segments {
			if notation(segment) && segment.ProjectedRange().End == left {
				next = segment.ProjectedRange().Start
				break
			}
		}
		if next == left {
			break
		}
		left = next
		positions = append(positions, left)
	}
	for right := at; ; {
		next := right
		for _, segment := range segments {
			if notation(segment) && segment.ProjectedRange().Start == right {
				next = segment.ProjectedRange().End
				break
			}
		}
		if next == right {
			break
		}
		right = next
		positions = append(positions, right)
	}
	return positions
}

// caretTarget is the collapsed target at at, facing the text beside it, so
// its address is a text caret and typing inserts text rather than editing
// the neighboring notation. Between two notation runs it keeps fallback.
func caretTarget(
	p *projection.Projection,
	at model.UTF16Offset,
	fallback model.Affinity,
) (projection.Target, error) {
	target, err := p.Target(model.EmptyUTF16Range(at))
	if err != nil {
		return projection.Target{}, err
	}
	affinity := fallback
	var textBefore, textAfter, notationBefore, notationAfter bool
	for _, segment := range p.Segments() {
		span := segment.ProjectedRange()
		_, text := segment.SemanticTarget()
		_, notation := segment.AttributeTarget()
		textAfter = textAfter || text && span.Start == at
		textBefore = textBefore || text && span.End == at
		notationAfter = notationAfter || notation && span.Start == at && !span.Empty()
		notationBefore = notationBefore || notation && span.End == at && !span.Empty()
	}
	switch {
	case textAfter && !textBefore:
		affinity = model.AffinityRight
	case textBefore && !textAfter:
		affinity = model.AffinityLeft
	case textBefore || textAfter:
	// With no text beside it, the caret faces away from the notation it
	// stands outside: before an opener it leans left, past a closer right.
	case notationAfter && !notationBefore:
		affinity = model.AffinityLeft
	case notationBefore && !notationAfter:
		affinity = model.AffinityRight
	}
	return target.WithAffinity(affinity), nil
}

func sameMark(left, right *model.MarkID) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func absOffset(value model.UTF16Offset) int {
	if value < 0 {
		return int(-value)
	}
	return int(value)
}
