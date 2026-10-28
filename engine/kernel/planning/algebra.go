package planning

import (
	"fmt"
	"sort"
)

// DefaultControlPlanBudget bounds control-plan compilation work.
const DefaultControlPlanBudget = 65_536

// ControlID identifies a node in a control program.
type ControlID uint32

// Value returns the numeric control identifier.
func (id ControlID) Value() uint32 { return uint32(id) }

// LeafID identifies a callback invoked by a control program.
type LeafID uint32

// NewLeafID constructs a leaf identifier.
func NewLeafID(value uint32) LeafID { return LeafID(value) }

// Value returns the numeric leaf identifier.
func (id LeafID) Value() uint32 { return uint32(id) }

// Outcome is the result of a control callback.
type Outcome uint8

const (
	// OutcomeHandled commits a callback's effects.
	OutcomeHandled Outcome = iota + 1
	// OutcomePass declines a callback, allowing ordered choice to continue.
	OutcomePass
)

func (o Outcome) String() string {
	switch o {
	case OutcomeHandled:
		return "handled"
	case OutcomePass:
		return "pass"
	default:
		return fmt.Sprintf("outcome(%d)", o)
	}
}

// AlgebraError reports a root control identifier that does not exist in the
// built program.
type AlgebraError struct {
	InvalidControlID ControlID
}

func (e *AlgebraError) Error() string {
	return fmt.Sprintf("invalid control id %d", e.InvalidControlID.Value())
}

type controlKind uint8

const (
	controlIdentity controlKind = iota + 1
	controlPass
	controlLeaf
	controlThen
	controlFirst
)

type controlNode struct {
	kind     controlKind
	leaf     LeafID
	children []ControlID
}

// ControlProgram is an immutable control structure. Build it with
// ControlBuilder.
type ControlProgram struct {
	nodes []controlNode
	root  ControlID
}

// ControlBuilder constructs normalized control programs.
type ControlBuilder struct {
	nodes []controlNode
}

// Identity appends a node that reports Handled.
func (b *ControlBuilder) Identity() ControlID {
	return b.push(controlNode{kind: controlIdentity})
}

// Pass appends a node that reports Pass.
func (b *ControlBuilder) Pass() ControlID {
	return b.push(controlNode{kind: controlPass})
}

// Leaf appends a callback leaf.
func (b *ControlBuilder) Leaf(leaf LeafID) ControlID {
	return b.push(controlNode{kind: controlLeaf, leaf: leaf})
}

// Then builds an ordered sequence. Identities are removed, nested sequences are
// flattened, and every node after the first Pass is discarded.
func (b *ControlBuilder) Then(children ...ControlID) ControlID {
	normalized := make([]ControlID, 0, len(children))
	for _, child := range children {
		switch b.node(child).kind {
		case controlIdentity:
			continue
		case controlPass:
			normalized = append(normalized, child)
			return b.finishThen(normalized)
		case controlThen:
			normalized = append(normalized, b.node(child).children...)
		default:
			normalized = append(normalized, child)
		}
	}
	return b.finishThen(normalized)
}

func (b *ControlBuilder) finishThen(children []ControlID) ControlID {
	switch len(children) {
	case 0:
		return b.Identity()
	case 1:
		return children[0]
	default:
		return b.push(controlNode{kind: controlThen, children: cloneControlIDs(children)})
	}
}

// First builds an ordered choice. Pass alternatives are removed, nested choices
// are flattened, and every node after the first Identity is discarded.
func (b *ControlBuilder) First(children ...ControlID) ControlID {
	normalized := make([]ControlID, 0, len(children))
	for _, child := range children {
		switch b.node(child).kind {
		case controlPass:
			continue
		case controlFirst:
			normalized = append(normalized, b.node(child).children...)
		case controlIdentity:
			normalized = append(normalized, child)
			return b.finishFirst(normalized)
		default:
			normalized = append(normalized, child)
		}
	}
	return b.finishFirst(normalized)
}

func (b *ControlBuilder) finishFirst(children []ControlID) ControlID {
	switch len(children) {
	case 0:
		return b.Pass()
	case 1:
		return children[0]
	default:
		return b.push(controlNode{kind: controlFirst, children: cloneControlIDs(children)})
	}
}

// Build snapshots the builder and validates root.
func (b *ControlBuilder) Build(root ControlID) (*ControlProgram, error) {
	if int(root.Value()) >= len(b.nodes) {
		return nil, &AlgebraError{InvalidControlID: root}
	}
	nodes := cloneControlNodes(b.nodes)
	return &ControlProgram{nodes: nodes, root: root}, nil
}

func (b *ControlBuilder) push(node controlNode) ControlID {
	id := ControlID(len(b.nodes))
	b.nodes = append(b.nodes, node)
	return id
}

func (b *ControlBuilder) node(id ControlID) *controlNode {
	if int(id.Value()) >= len(b.nodes) {
		panic(fmt.Sprintf("planning: invalid control id %d", id.Value()))
	}
	return &b.nodes[id.Value()]
}

func cloneControlIDs(children []ControlID) []ControlID {
	if len(children) == 0 {
		return nil
	}
	cloned := make([]ControlID, len(children))
	copy(cloned, children)
	return cloned
}

func cloneControlNodes(nodes []controlNode) []controlNode {
	cloned := make([]controlNode, len(nodes))
	for index, node := range nodes {
		node.children = cloneControlIDs(node.children)
		cloned[index] = node
	}
	return cloned
}

// Root returns the program's entry control.
func (p *ControlProgram) Root() ControlID { return p.root }

// NodeCount returns all nodes retained by the builder, including unreachable
// ones.
func (p *ControlProgram) NodeCount() int { return len(p.nodes) }

// Leaves returns unique reachable callback identities in ascending order.
func (p *ControlProgram) Leaves() []LeafID {
	leaves := make(map[LeafID]struct{})
	pending := []ControlID{p.root}
	for len(pending) > 0 {
		last := len(pending) - 1
		id := pending[last]
		pending = pending[:last]
		node := p.node(id)
		switch node.kind {
		case controlIdentity, controlPass:
		case controlLeaf:
			leaves[node.leaf] = struct{}{}
		case controlThen, controlFirst:
			for index := len(node.children) - 1; index >= 0; index-- {
				pending = append(pending, node.children[index])
			}
		default:
			panic("planning: invalid control node")
		}
	}
	result := make([]LeafID, 0, len(leaves))
	for leaf := range leaves {
		result = append(result, leaf)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Value() < result[j].Value()
	})
	return result
}

func (p *ControlProgram) node(id ControlID) *controlNode {
	if int(id.Value()) >= len(p.nodes) {
		panic(fmt.Sprintf("planning: invalid control id %d", id.Value()))
	}
	return &p.nodes[id.Value()]
}
