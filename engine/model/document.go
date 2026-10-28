package model

import (
	"slices"
)

// SemanticAction is one checked mutation expressible through a document draft.
type SemanticAction struct {
	Kind  SemanticActionKind
	Node  NodeID
	After NodeID

	Parent   NodeID
	Index    int
	NodeType NodeTypeID

	Text *string

	Key   string
	Value AttributeValue
}

// SemanticActionKind identifies a SemanticAction payload.
type SemanticActionKind uint8

const (
	ActionCreateNode SemanticActionKind = iota + 1
	ActionSetText
	ActionSetAttribute
	ActionRemoveAttribute
	ActionRemoveNode
)

// ChangeKind identifies a final semantic tree change.
type ChangeKind uint8

const (
	ChangeCreated ChangeKind = iota + 1
	ChangeUpdated
	ChangeRemoved
)

// Change is one deterministic final node change.
type Change struct {
	Kind ChangeKind
	Node NodeID
}

type documentState struct {
	root   NodeID
	nodes  map[NodeID]*node
	nextID uint64
}

// Document is a mutable semantic document publisher. Its snapshots remain
// immutable after later transactions.
type Document struct {
	state    *documentState
	revision uint64
}

// DocumentSnapshot is an immutable, revision-bound view of a document.
type DocumentSnapshot struct {
	state    *documentState
	revision uint64
}

// TransactionOutcome reports the published revision and final changes.
type TransactionOutcome struct {
	Revision uint64
	Changes  []Change
}

// NewDocument creates a document with a root node and revision zero.
func NewDocument(rootKind NodeTypeID) (*Document, error) {
	if err := validateNodeType(rootKind); err != nil {
		return nil, err
	}
	const root NodeID = 1
	state := &documentState{
		root:   root,
		nodes:  map[NodeID]*node{root: newNode(root, rootKind, 0, false)},
		nextID: 2,
	}
	return &Document{state: state}, nil
}

// Clone returns a document that initially shares the same immutable state.
func (d *Document) Clone() *Document {
	return &Document{state: d.state, revision: d.revision}
}

// Revision returns the published document revision.
func (d *Document) Revision() uint64 {
	return d.revision
}

// Snapshot captures the current immutable state and revision.
func (d *Document) Snapshot() DocumentSnapshot {
	return DocumentSnapshot{state: d.state, revision: d.revision}
}

// BeginDraft starts a copy-on-write transaction over the current state.
func (d *Document) BeginDraft() *DocumentDraft {
	return &DocumentDraft{
		base:  d.state,
		state: d.state,
	}
}

// Transact applies and validates a draft. No state or revision is published
// when the operation or validation fails, or when there are no final changes.
func (d *Document) Transact(operation func(*DocumentDraft) error) (TransactionOutcome, error) {
	draft := d.BeginDraft()
	if err := operation(draft); err != nil {
		return TransactionOutcome{}, err
	}
	if err := draft.Validate(); err != nil {
		return TransactionOutcome{}, err
	}
	changes := draft.FinalChanges()
	revision := d.revision
	if len(changes) > 0 {
		var err error
		revision, err = incrementRevision(d.revision)
		if err != nil {
			return TransactionOutcome{}, err
		}
		d.state = draft.state
		d.revision = revision
	}
	return TransactionOutcome{Revision: revision, Changes: changes}, nil
}

// PublishDraft installs an already-validated draft at a caller-owned revision.
// Engine publication uses this after validating all document and host state.
func (d *Document) PublishDraft(draft *DocumentDraft, revision uint64) error {
	if err := draft.Validate(); err != nil {
		return err
	}
	d.state = draft.state
	d.revision = revision
	return nil
}

func incrementRevision(revision uint64) (uint64, error) {
	if revision == ^uint64(0) {
		return 0, &DocumentError{Kind: DocumentErrorRevisionExhausted}
	}
	return revision + 1, nil
}

// Revision returns the snapshot's captured revision.
func (s DocumentSnapshot) Revision() uint64 {
	return s.revision
}

// Root returns the root node identity.
func (s DocumentSnapshot) Root() NodeID {
	return s.state.root
}

// Node returns an immutable node by identity.
func (s DocumentSnapshot) Node(id NodeID) (*Node, bool) {
	node, ok := s.state.nodes[id]
	if !ok {
		return nil, false
	}
	value := node.value()
	return &value, true
}

// Nodes returns all nodes in ascending identity order.
func (s DocumentSnapshot) Nodes() []*Node {
	ids := make([]NodeID, 0, len(s.state.nodes))
	for id := range s.state.nodes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	nodes := make([]*Node, 0, len(ids))
	for _, id := range ids {
		value := s.state.nodes[id].value()
		nodes = append(nodes, &value)
	}
	return nodes
}

func validateNodeType(kind NodeTypeID) error {
	if kind == "" {
		return &DocumentError{Kind: DocumentErrorEmptyNodeType}
	}
	return nil
}
