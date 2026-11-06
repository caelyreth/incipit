package editor

import (
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// doNest moves every block the selection covers that nests, items
// and quote lines, a level deeper or shallower. A block goes no deeper
// than one level past the block before it in its group, so a group never
// skips a level; nothing shallower than the outermost level exists.
func (s *Session) doNest(command nestingOp) (kernel.Outcome, error) {
	if !s.runtime.Configuration().IsEnabled(WritingPolicyID) {
		return kernel.OutcomePass, nil
	}
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	ranges := selectionBlockRanges(s.runtime.Snapshot().Document(), s.presentation.Selection)
	nodes := make([]model.NodeID, 0, len(ranges))
	for _, part := range ranges {
		nodes = append(nodes, part.Node)
	}
	return s.dispatchSelectionCommand(
		nestTrigger,
		s.command(nestRule, (&nestingInvoker{command: command, nodes: nodes}).run),
		func(*model.Node) *model.MarkID { return activeMark(s.inline) },
	)
}

type nestingInvoker struct {
	command nestingOp
	nodes   []model.NodeID
}

func (i *nestingInvoker) run(context *kernel.RuleContext) (kernel.Outcome, error) {
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	moved := false
	for _, id := range i.nodes {
		node, ok := document.Node(id)
		if !ok {
			continue
		}
		shape := shapeOf(node)
		if !shape.nests() {
			continue
		}
		depth := shape.depth - 1
		if i.command == nestDeeper {
			depth = min(shape.depth+1, maxDepth, nestingCeiling(document, node))
		}
		if depth < 0 || depth == shape.depth {
			continue
		}
		shape.depth = depth
		if err := shapeBlock(document, id, shape); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
		moved = true
	}
	if !moved {
		return kernel.OutcomePass, nil
	}
	return kernel.OutcomeHandled, nil
}

// nestingCeiling is the deepest node may go: one level past the block
// before it, when that block is of its group, and the outermost otherwise.
func nestingCeiling(document *model.DocumentDraft, node *model.Node) int {
	group, _ := groupOf(node)
	parent, _ := node.Parent()
	container, _ := document.Node(parent)
	previous := model.NodeID(0)
	for _, child := range container.Children() {
		if child == node.ID() {
			break
		}
		previous = child
	}
	before, ok := document.Node(previous)
	if !ok {
		return 0
	}
	other, ok := groupOf(before)
	if !ok || other.family != group.family {
		return 0
	}
	return other.depth + 1
}

// nestingOp moves blocks a level deeper or shallower.
type nestingOp uint8

const (
	nestDeeper nestingOp = iota + 1
	nestShallower
)
