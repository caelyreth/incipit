package editor

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Backspace and Delete at a caret are planned as one value, each planner a
// responder (responders.go), and done by one interpreter (carryOut). A deletion that would take
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

// deletionSite is where a Backspace (backward) or Delete at a collapsed
// caret happens: its block, the block beside it the press goes toward, and
// whether the caret is at the edge it goes toward.
type deletionSite struct {
	node, beside     *model.Node
	besideOK         bool
	backward, atEdge bool
}

func (s *Session) deletionSite(backward bool) (deletionSite, bool) {
	if !s.presentation.VisualCollapsed() {
		return deletionSite{}, false
	}
	document := s.runtime.Snapshot().Document()
	head := s.presentation.Selection.Head()
	node, ok := document.Node(head.Node())
	if !ok {
		return deletionSite{}, false
	}
	beside, besideOK := neighborBlock(document, node.ID(), pick(boolDirection(backward), -1, 1))
	atEdge := backward && head.Offset() == 0 || !backward && head.Offset() == blockLength(node)
	return deletionSite{node: node, beside: beside, besideOK: besideOK, backward: backward, atEdge: atEdge}, true
}

// carryOut does a planned deletion: an arm is left for the next press.
func (s *Session) carryOut(plan deletion, backward bool) (EditingOutcome, bool, error) {
	switch plan.kind {
	case plannedMove:
		return answered(kernel.OutcomeHandled, s.Select(plan.to))
	case plannedArm:
		s.follow(followUp{arm: &armedDeletion{span: plan.span, join: plan.join, backward: backward}})
	case plannedEdit:
		edit, err := FromProjectedRange(s.presentation.Projection, plan.span, plan.text, kernel.EventOriginSemantic)
		if err != nil {
			return EditingOutcome{}, true, err
		}
		outcome, err := s.applyProjectedEdit(edit)
		return outcome, true, err
	}
	return handled()
}

// atomDeletion arms an atomic block whole: Backspace at its text's start
// or at the start of a block with text below it, Delete at the end of the
// block above it. An empty block below one goes as any empty block does.
func (s *Session) atomDeletion(at deletionSite) (deletion, bool) {
	node, beside, besideOK, backward := at.node, at.beside, at.besideOK, at.backward
	if !at.atEdge {
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

// stepTo is a deletion stepping the caret into to, at the edge it comes in
// by.
func (s *Session) stepTo(to *model.Node, backward bool) (deletion, bool) {
	offset := model.UTF16Offset(0)
	if backward {
		offset = blockLength(to)
	}
	at := model.NewTextAnchor(to.ID(), offset, model.AffinityRight, s.runtime.Revision())
	return deletion{kind: plannedMove, to: model.NewTextSelection(at, at)}, true
}

// cellEdgeDeletion keeps Backspace and Delete at a cell's edge from joining
// it to what is beside it: they step to the cell before or after. A table
// holding nothing is armed whole.
func (s *Session) cellEdgeDeletion(at deletionSite) (deletion, bool) {
	if !at.atEdge {
		return deletion{}, false
	}
	document := s.runtime.Snapshot().Document()
	table, _ := TableOf(document, at.node.ID())
	cells := table.Cells()
	if next := slices.Index(cells, at.node.ID()) + pick(boolDirection(at.backward), -1, 1); next >= 0 && next < len(cells) {
		to, _ := document.Node(cells[next])
		return s.stepTo(to, at.backward)
	}
	if at.backward && emptyTable(document, table) {
		first, okFirst := s.BlockRange(table.First())
		last, okLast := s.BlockRange(table.Last())
		return deletion{kind: plannedArm, span: model.NewUTF16Range(first.Start, last.End)}, okFirst && okLast
	}
	if !at.besideOK || blockLength(at.beside) == 0 && kindOf(at.beside).Plain {
		return deletion{kind: plannedNothing}, true
	}
	return s.stepTo(at.beside, at.backward)
}

// intoTableDeletion steps from a block with text beside a table into it,
// rather than joining it to a cell.
func (s *Session) intoTableDeletion(at deletionSite) (deletion, bool) {
	if !at.atEdge || !at.besideOK || !kindOf(at.beside).Cell || blockLength(at.node) == 0 {
		return deletion{}, false
	}
	return s.stepTo(at.beside, at.backward)
}

// joinDeletion arms a join of prose with literal text, either way: what is
// taken as written and what is read as notation would run together. An
// empty block goes as any empty block does.
func (s *Session) joinDeletion(at deletionSite) (deletion, bool) {
	node, beside, besideOK := at.node, at.beside, at.besideOK
	if !at.atEdge || !besideOK || blockLength(node) == 0 || blockLength(beside) == 0 ||
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
func (s *Session) levelDeletion(at deletionSite) (deletion, bool) {
	if !at.backward || s.presentation.Selection.Head().Offset() != 0 {
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
func (s *Session) notationDeletion(at deletionSite) (deletion, bool) {
	backward := at.backward
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
	f, ok := s.live()
	if !ok || f.arm == nil {
		return nil
	}
	return []Guide{{
		Producer: surface.ProducerID(TextPolicyID),
		Local:    1,
		Command:  pick(boolDirection(f.arm.backward), DeleteBackwardCommandID, DeleteForwardCommandID),
		Area:     []model.UTF16Range{f.arm.span},
	}}
}

// boolDirection is Backward when backward is set.
func boolDirection(backward bool) Direction {
	if backward {
		return Backward
	}
	return Forward
}
