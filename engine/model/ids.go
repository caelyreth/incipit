package model

import "strconv"

// NodeId is the stable identity of a semantic node within one document.
type NodeId uint64

// NodeID is an alias for callers favoring Go's initialism style.
type NodeID = NodeId

// Value returns the numeric identity.
func (id NodeId) Value() uint64 {
	return uint64(id)
}

// String formats a node identity for diagnostics.
func (id NodeId) String() string {
	return "node-" + strconv.FormatUint(uint64(id), 10)
}

// NodeTypeId is an application-defined semantic node type identity.
type NodeTypeId string

// NodeTypeID is an alias for callers favoring Go's initialism style.
type NodeTypeID = NodeTypeId

// NewNodeTypeID creates a node type identity.
func NewNodeTypeID(value string) NodeTypeId {
	return NodeTypeId(value)
}

// String returns the stable application-defined identity.
func (id NodeTypeId) String() string {
	return string(id)
}

// MarkId is the stable identity of an inline mark within one node.
type MarkId uint64

// MarkID is an alias for callers favoring Go's initialism style.
type MarkID = MarkId

// Value returns the numeric identity.
func (id MarkId) Value() uint64 {
	return uint64(id)
}

// String formats a mark identity for diagnostics.
func (id MarkId) String() string {
	return "mark-" + strconv.FormatUint(uint64(id), 10)
}
