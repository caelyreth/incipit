package model

import (
	"maps"
	"slices"
)

// NodeRecord is a node as it is kept outside the process: everything that
// makes it, in plain values. Records round-trip, so a document restored from
// the records of a snapshot is that snapshot's document.
type NodeRecord struct {
	ID         NodeID
	Kind       NodeTypeID
	Parent     NodeID
	HasParent  bool
	Children   []NodeID
	Attributes map[string]AttributeValue
	Marks      []InlineMark
	NextMarkID uint64
	// Text is nil for a node that holds no text.
	Text *string
}

// Record returns the node as plain values.
func (n *Node) Record() NodeRecord {
	record := NodeRecord{
		ID:         n.id,
		Kind:       n.kind,
		Parent:     n.parent,
		HasParent:  n.hasParent,
		Children:   slices.Clone(n.children),
		Attributes: maps.Clone(n.attributes),
		Marks:      slices.Clone(n.marks),
		NextMarkID: n.nextMarkID,
	}
	if n.text != nil {
		text := n.text.String()
		record.Text = &text
	}
	return record
}

// NextID is the identity the snapshot's next created node takes.
func (s DocumentSnapshot) NextID() uint64 {
	return s.state.nextID
}

// Since reports what differs from an earlier snapshot of the same document:
// the nodes written since, and the identities removed since. Nodes are
// immutable and a write replaces the node, so an unchanged node is the same
// node in both and the comparison never reads content.
func (s DocumentSnapshot) Since(earlier DocumentSnapshot) (written []*Node, removed []NodeID) {
	if s.state == earlier.state {
		return nil, nil
	}
	for id, node := range s.state.nodes {
		if earlier.state == nil || earlier.state.nodes[id] != node {
			written = append(written, node)
		}
	}
	if earlier.state != nil {
		for id := range earlier.state.nodes {
			if _, ok := s.state.nodes[id]; !ok {
				removed = append(removed, id)
			}
		}
	}
	slices.SortFunc(written, func(left, right *Node) int { return compare(left.id, right.id) })
	slices.Sort(removed)
	return written, removed
}

// RestoreDocument rebuilds a document from its node records. The records
// must form exactly one tree under root, each child naming the parent that
// lists it, with every identity below nextID; the document is checked as a
// transaction would be, so a restored document is one an edit could have
// made.
func RestoreDocument(root NodeID, nextID uint64, records []NodeRecord) (*Document, error) {
	state := &documentState{
		root:   root,
		nodes:  make(map[NodeID]*Node, len(records)),
		nextID: nextID,
	}
	for _, record := range records {
		if _, exists := state.nodes[record.ID]; exists {
			return nil, invalidDocument("node " + record.ID.String() + " is recorded twice")
		}
		if uint64(record.ID) == 0 || uint64(record.ID) >= nextID {
			return nil, invalidDocument("node " + record.ID.String() + " is outside the identity range")
		}
		if record.Text != nil {
			if err := validateText(*record.Text); err != nil {
				return nil, err
			}
		}
		node := newNode(record.ID, record.Kind, record.Parent, record.HasParent)
		node.children = slices.Clone(record.Children)
		maps.Copy(node.attributes, record.Attributes)
		node.marks = slices.Clone(record.Marks)
		node.nextMarkID = record.NextMarkID
		if record.Text != nil {
			text := NewTextRoot(*record.Text)
			node.text = &text
		}
		state.nodes[record.ID] = node
	}
	draft := &DocumentDraft{base: state, state: state}
	if err := draft.Validate(); err != nil {
		return nil, err
	}
	// Validate checks each listed child names its parent; a node no parent
	// lists, or one listed twice, would be kept but never shown.
	reached := make([]NodeID, 0, len(state.nodes))
	collectSubtree(state.nodes, root, &reached)
	seen := make(map[NodeID]struct{}, len(reached))
	for _, id := range reached {
		if _, twice := seen[id]; twice {
			return nil, invalidDocument("node " + id.String() + " is listed twice")
		}
		seen[id] = struct{}{}
	}
	if len(seen) != len(state.nodes) {
		return nil, invalidDocument("a recorded node is not part of the tree")
	}
	return &Document{state: state}, nil
}
