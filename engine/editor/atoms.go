package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// An atomic block stands on an object: one character of its own,
// figureNotation, its notation, that is what the block shows apart
// from its text, as an image's picture. Every command meets it alike,
// whatever its text, because each reads the trait rather than a kind:
//
//   - the caret steps over the object, never resting before it;
//   - deletion never takes it a character at a time, nor joins text to it
//     unseen: Backspace at its text's start or at the start of the block
//     below it with text, or Delete at the end of the block above it,
//     selects the block whole (guardAtom), and deleting that selection, or
//     an edit across its whole line, takes it away;
//   - selecting all from it widens past it, never into its text;
//   - the object is no text: what is copied leaves it out (ReadableText).

// atomic reports whether a block shape stands on an object.
func (s blockShape) atomic() bool {
	return s.kind == ImageNode
}

// guardAtom is Backspace at the start of an atomic block's text or of a
// block with text below one, or Delete at the end of the block above one:
// each selects the atomic block, for a second press to delete. It reports
// whether it did; an empty block below an atomic one goes as any empty
// block does, the caret going to the end of the atomic block's text.
func (s *Session) guardAtom(backward bool) (kernel.Outcome, bool, error) {
	selection := s.presentation.Selection
	if !s.presentation.VisualCollapsed() {
		return 0, false, nil
	}
	document := s.runtime.Snapshot().Document()
	node, ok := document.Node(selection.Head().Node())
	if !ok {
		return 0, false, nil
	}
	atom := node
	switch {
	case backward && selection.Head().Offset() != 0:
		ok = false
	case backward && !shapeOf(node).atomic():
		atom, ok = neighborBlock(document, node.ID(), -1)
		ok = ok && blockLength(node) > 0
	case !backward && selection.Head().Offset() == blockLength(node):
		atom, ok = neighborBlock(document, node.ID(), 1)
	case !backward:
		ok = false
	}
	if !ok || !shapeOf(atom).atomic() {
		return 0, false, nil
	}
	line, ok := s.BlockRange(atom.ID())
	if !ok {
		return 0, false, nil
	}
	return kernel.OutcomeHandled, true, s.SelectProjected(line.Start, line.End)
}

// BlockRange is the projected range of a block's whole line: its
// notation and its text, as selecting the block, a click on a picture,
// selects it.
func (s *Session) BlockRange(node model.NodeID) (model.UTF16Range, bool) {
	start, end, found := model.UTF16Offset(0), model.UTF16Offset(0), false
	s.presentation.Projection.EachSegment(func(_ int, segment projection.Segment) bool {
		if owner, ok := segmentNode(segment); !ok || owner != node {
			return true
		}
		span := segment.ProjectedRange()
		if !found {
			start, found = span.Start, true
		}
		end = span.End
		return true
	})
	return model.NewUTF16Range(start, end), found
}

// ReadableText is the page's text across a projected range as a reader
// takes it, as copying does: the notation is the page's, but the object an
// atomic block stands on is no text.
func (s *Session) ReadableText(range16 model.UTF16Range) (string, error) {
	p := s.presentation.Projection
	flow, err := s.SurfaceFlow()
	if err != nil {
		return "", err
	}
	// The break before a cell that does not start its row reads as a tab.
	tabs := map[model.UTF16Offset]bool{}
	if tables := flow.Tables(); len(tables) > 0 {
		for _, table := range tables {
			for _, row := range table.Rows {
				for _, cell := range row[1:] {
					tabs[cell.Range.Start-1] = true
				}
			}
		}
	}
	var text strings.Builder
	var failure error
	p.EachSegment(func(_ int, segment projection.Segment) bool {
		span := segment.ProjectedRange()
		from, to := max(span.Start, range16.Start), min(span.End, range16.End)
		if from >= to {
			return true
		}
		if role, ok := p.RoleAt(span.Start); ok && role.Role == roleFigure {
			return true
		}
		part, err := p.TextRange(model.NewUTF16Range(from, to))
		if err != nil {
			failure = err
			return false
		}
		if part == "\n" && tabs[from] {
			part = "\t"
		}
		text.WriteString(part)
		return true
	})
	return text.String(), failure
}

// segmentNode is the block a segment of the projection shows, its text or
// its notation.
func segmentNode(segment projection.Segment) (model.NodeID, bool) {
	if target, ok := segment.AttributeTarget(); ok {
		return target.Node(), true
	}
	if target, ok := segment.SemanticTarget(); ok {
		return target.Node(), true
	}
	return 0, false
}
