package editor

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Backspace and Delete at a caret are planned as one value (planDeletion)
// and done by one interpreter (deleteAtCaret). A deletion that would take
// more than a character, or join what reads differently, never does so
// unseen: it arms first, marking what a second press takes, and any other
// intent disarms it. Structure steps back one level a press.

// deletionKind is what a planned deletion does.
type deletionKind uint8

const (
	// plannedMove moves the caret without deleting: across a table's edge.
	plannedMove deletionKind = iota + 1
	// plannedArm marks Range for a second press to take, or, for a join,
	// to go on as the press would have.
	plannedArm
	// plannedEdit replaces Range with Text.
	plannedEdit
	// plannedNothing is handled, with nothing to do.
	plannedNothing
)

// deletion is a planned Backspace or Delete.
type deletion struct {
	kind deletionKind
	to   model.TextSelection
	span model.UTF16Range
	text string
	// join, for an arm, goes on with the press once armed, rather than
	// taking Range.
	join bool
}

// armed is a deletion waiting for its second press, while the caret and the
// page stay as they were.
type armed struct {
	span     model.UTF16Range
	join     bool
	backward bool
	caret    model.UTF16Offset
	revision uint64
}

// armedFor returns the armed deletion a press in direction completes.
func (s *Session) armedFor(backward bool) (*armed, bool) {
	a := s.armed
	if a == nil || a.backward != backward || a.revision != s.runtime.Revision() ||
		!s.presentation.VisualCollapsed() || s.presentation.Caret.RangeUTF16().Start != a.caret {
		return nil, false
	}
	return a, true
}

// disarm drops an armed deletion: any intent but its second press does.
func (s *Session) disarm() { s.armed = nil }

// deleteAtCaret does Backspace (backward) or Delete at a collapsed caret as
// planned, reporting whether a plan took it.
func (s *Session) deleteAtCaret(backward bool) (kernel.Outcome, bool, error) {
	if a, ok := s.armedFor(backward); ok {
		s.disarm()
		if a.join {
			return 0, false, nil
		}
		if err := s.SelectProjected(a.span.Start, a.span.End); err != nil {
			return 0, true, err
		}
		outcome, err := s.replaceSelection("", kernel.EventOriginSemantic)
		return outcome.Outcome, true, err
	}
	s.disarm()
	plan, ok := s.planDeletion(backward)
	if !ok {
		return 0, false, nil
	}
	switch plan.kind {
	case plannedMove:
		return kernel.OutcomeHandled, true, s.Select(plan.to)
	case plannedArm:
		s.armed = &armed{
			span: plan.span, join: plan.join, backward: backward,
			caret: s.presentation.Caret.RangeUTF16().Start, revision: s.runtime.Revision(),
		}
		return kernel.OutcomeHandled, true, nil
	case plannedEdit:
		edit, err := FromProjectedRange(s.presentation.Projection, plan.span, plan.text, kernel.EventOriginSemantic)
		if err != nil {
			return 0, true, err
		}
		outcome, err := s.applyProjectedEdit(edit)
		return outcome.Outcome, true, err
	}
	return kernel.OutcomeHandled, true, nil
}

// planDeletion is what Backspace or Delete at the caret does, if anything
// other than taking a character: the first plan that holds.
func (s *Session) planDeletion(backward bool) (deletion, bool) {
	if !s.presentation.VisualCollapsed() {
		return deletion{}, false
	}
	document := s.runtime.Snapshot().Document()
	head := s.presentation.Selection.Head()
	node, ok := document.Node(head.Node())
	if !ok {
		return deletion{}, false
	}
	step := 1
	if backward {
		step = -1
	}
	atEdge := backward && head.Offset() == 0 || !backward && head.Offset() == blockLength(node)
	beside, besideOK := neighborBlock(document, node.ID(), step)
	for _, plan := range []func() (deletion, bool){
		func() (deletion, bool) { return s.atomDeletion(node, beside, besideOK, backward, atEdge) },
		func() (deletion, bool) { return s.cellDeletion(node, beside, besideOK, backward, atEdge) },
		func() (deletion, bool) { return s.joinDeletion(node, beside, besideOK, atEdge) },
		func() (deletion, bool) { return s.levelDeletion(backward) },
		func() (deletion, bool) { return s.notationDeletion(backward) },
	} {
		if d, ok := plan(); ok {
			return d, true
		}
	}
	return deletion{}, false
}

// atomDeletion arms an atomic block whole: Backspace at its text's start
// or at the start of a block with text below it, Delete at the end of the
// block above it. An empty block below one goes as any empty block does.
func (s *Session) atomDeletion(node, beside *model.Node, besideOK, backward, atEdge bool) (deletion, bool) {
	if !atEdge {
		return deletion{}, false
	}
	atom := node
	if !backward || !shapeOf(node).atomic() {
		if !besideOK || backward && blockLength(node) == 0 {
			return deletion{}, false
		}
		atom = beside
	}
	if !shapeOf(atom).atomic() {
		return deletion{}, false
	}
	line, ok := s.BlockRange(atom.ID())
	return deletion{kind: plannedArm, span: line}, ok
}

// cellDeletion keeps Backspace and Delete at a cell's edge from joining it
// to what is beside it: they step to the cell before or after, or from a
// block beside a table into it. A table holding nothing is armed whole.
func (s *Session) cellDeletion(node, beside *model.Node, besideOK, backward, atEdge bool) (deletion, bool) {
	if !atEdge {
		return deletion{}, false
	}
	document := s.runtime.Snapshot().Document()
	move := func(to *model.Node) (deletion, bool) {
		offset := model.UTF16Offset(0)
		if backward {
			offset = blockLength(to)
		}
		at := model.NewTextAnchor(to.ID(), offset, model.AffinityRight, s.runtime.Revision())
		return deletion{kind: plannedMove, to: model.NewTextSelection(at, at)}, true
	}
	if kindOf(node).Cell {
		table, _ := TableOf(document, node.ID())
		cells := table.Cells()
		step := 1
		if backward {
			step = -1
		}
		if next := slices.Index(cells, node.ID()) + step; next >= 0 && next < len(cells) {
			to, _ := document.Node(cells[next])
			return move(to)
		}
		if backward && emptyTable(document, table) {
			first, okFirst := s.BlockRange(table.First())
			last, okLast := s.BlockRange(table.Last())
			return deletion{kind: plannedArm, span: model.NewUTF16Range(first.Start, last.End)}, okFirst && okLast
		}
		if !besideOK || blockLength(beside) == 0 && kindOf(beside).Plain {
			return deletion{kind: plannedNothing}, true
		}
		return move(beside)
	}
	if !besideOK || !kindOf(beside).Cell || blockLength(node) == 0 {
		return deletion{}, false
	}
	return move(beside)
}

// joinDeletion arms a join of prose with literal text, either way: what is
// taken as written and what is read as notation would run together. An
// empty block goes as any empty block does.
func (s *Session) joinDeletion(node, beside *model.Node, besideOK, atEdge bool) (deletion, bool) {
	if !atEdge || !besideOK || blockLength(node) == 0 || blockLength(beside) == 0 ||
		kindOf(node).Literal == kindOf(beside).Literal || kindOf(node).Cell || kindOf(beside).Cell {
		return deletion{}, false
	}
	literal := node
	if kindOf(beside).Literal {
		literal = beside
	}
	line, ok := s.BlockRange(literal.ID())
	return deletion{kind: plannedArm, span: line, join: true}, ok
}

// levelDeletion is Backspace at the start of a block's text, after the
// notation that opens it: the block's structure steps back one level, as
// its shape's peel says, rather than a character of its notation going.
func (s *Session) levelDeletion(backward bool) (deletion, bool) {
	if !backward || s.presentation.Selection.Head().Offset() != 0 {
		return deletion{}, false
	}
	caret := s.presentation.Caret.RangeUTF16().Start
	for _, segment := range s.presentation.Projection.Segments() {
		target, ok := segment.AttributeTarget()
		if !ok || target.Key() != blockNotationKey || segment.ProjectedRange().End != caret {
			continue
		}
		node, ok := s.runtime.Snapshot().Document().Node(target.Node())
		if !ok {
			return deletion{}, false
		}
		peeled, _ := blockNotation(shapeOf(node).peel(), 1)
		return deletion{kind: plannedEdit, span: segment.ProjectedRange(), text: peeled}, true
	}
	return deletion{}, false
}

// notationDeletion takes the character of notation beside the caret, which
// reads back as the structure or mark it leaves.
func (s *Session) notationDeletion(backward bool) (deletion, bool) {
	p := s.presentation.Projection
	caret := s.presentation.Caret.RangeUTF16().Start
	for _, segment := range p.Segments() {
		if _, ok := segment.AttributeTarget(); !ok {
			continue
		}
		r := segment.ProjectedRange()
		if backward {
			if r.End == caret || caret == 0 && r.Start == 0 {
				return deletion{kind: plannedEdit, span: model.NewUTF16Range(max(0, caret-1), caret)}, true
			}
		} else if r.Start == caret || caret == p.UTF16Len() && r.End == caret {
			return deletion{kind: plannedEdit, span: model.NewUTF16Range(caret, min(caret+1, p.UTF16Len()))}, true
		}
	}
	return deletion{}, false
}

// armGuides mark what an armed deletion takes, with the key that takes it.
func (s *Session) armGuides(time.Time) []Guide {
	for _, backward := range []bool{true, false} {
		if a, ok := s.armedFor(backward); ok {
			return []Guide{{
				Producer: surface.ProducerID(TextPolicyID),
				Local:    1,
				Command:  pick(boolDirection(backward), DeleteBackwardCommandID, DeleteForwardCommandID),
				Area:     []model.UTF16Range{a.span},
			}}
		}
	}
	return nil
}

// boolDirection is Backward when backward is set.
func boolDirection(backward bool) Direction {
	if backward {
		return Backward
	}
	return Forward
}
