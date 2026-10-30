package model

import (
	"slices"

	"github.com/caelyreth/neverland/engine/text/rope"
)

// Node is an immutable semantic node value. Document snapshots and drafts
// return copies so callers cannot mutate retained state.
type Node struct {
	id         NodeID
	kind       NodeTypeID
	parent     NodeID
	hasParent  bool
	children   []NodeID
	attributes map[string]AttributeValue
	marks      []InlineMark
	nextMarkID uint64
	text       *TextRoot
}

func newNode(id NodeID, kind NodeTypeID, parent NodeID, hasParent bool) *Node {
	return &Node{
		id:         id,
		kind:       kind,
		parent:     parent,
		hasParent:  hasParent,
		attributes: make(map[string]AttributeValue),
		nextMarkID: 1,
	}
}

// ID returns the stable node identity.
func (n *Node) ID() NodeID {
	return n.id
}

// Kind returns the application-defined node type.
func (n *Node) Kind() NodeTypeID {
	return n.kind
}

// Parent returns the parent identity when this is not the root.
func (n *Node) Parent() (NodeID, bool) {
	return n.parent, n.hasParent
}

// Children returns a copy of the ordered child identities.
func (n *Node) Children() []NodeID {
	return slices.Clone(n.children)
}

// TextPresent reports whether the node has an allocated text root.
func (n *Node) TextPresent() bool {
	return n.text != nil
}

// Text returns the immutable rope. An absent text value yields the empty tree;
// use TextPresent when None and Some("") must be distinguished.
func (n *Node) Text() rope.Tree {
	if n.text == nil {
		return rope.Tree{}
	}
	return n.text.tree
}

// TextRoot returns a copy of the immutable text-root handle.
func (n *Node) textRoot() (*TextRoot, bool) {
	if n.text == nil {
		return nil, false
	}
	return n.text, true
}

// Attribute returns an attribute value by key.
func (n *Node) Attribute(key string) (AttributeValue, bool) {
	value, ok := n.attributes[key]
	return value, ok
}

// Attributes returns a copy of all attributes.
func (n *Node) Attributes() map[string]AttributeValue {
	attributes := make(map[string]AttributeValue, len(n.attributes))
	for key, value := range n.attributes {
		attributes[key] = value
	}
	return attributes
}

// MarkCount returns the number of semantic inline marks.
func (n *Node) MarkCount() int {
	return len(n.marks)
}

// Marks returns a copy of the ordered inline marks.
func (n *Node) Marks() []InlineMark {
	return slices.Clone(n.marks)
}

// ValidateTextRangeUTF16 checks ordered scalar-aligned UTF-16 endpoints. A
// node without text accepts only 0..0. Grapheme boundaries are intentionally
// not required; combining marks are valid semantic positions.
func (n *Node) ValidateTextRangeUTF16(from, to UTF16Offset) error {
	_, err := n.TextByteRangeUTF16(from, to)
	return err
}

// TextByteRangeUTF16 validates a UTF-16 range and adapts it to bytes.
func (n *Node) TextByteRangeUTF16(from, to UTF16Offset) (ByteRange, error) {
	length := 0
	if n.text != nil {
		length = n.text.tree.UTF16Len()
	}
	if from < 0 || to < 0 || from > to || int(to) > length {
		return ByteRange{}, &DocumentError{
			Kind:   DocumentErrorInvalidTextRange,
			Node:   n.id,
			From:   int(from),
			To:     int(to),
			Length: length,
		}
	}
	if n.text == nil {
		return ByteRange{}, nil
	}
	start, ok := n.text.tree.CheckedByteOfUTF16(int(from))
	if !ok {
		return ByteRange{}, &DocumentError{
			Kind:   DocumentErrorInvalidTextBoundary,
			Node:   n.id,
			Offset: int(from),
		}
	}
	end := start
	if from != to {
		end, ok = n.text.tree.CheckedByteOfUTF16(int(to))
		if !ok {
			return ByteRange{}, &DocumentError{
				Kind:   DocumentErrorInvalidTextBoundary,
				Node:   n.id,
				Offset: int(to),
			}
		}
	}
	return NewByteRange(ByteOffset(start), ByteOffset(end)), nil
}

func (n *Node) clone() *Node {
	attributes := make(map[string]AttributeValue, len(n.attributes))
	for key, value := range n.attributes {
		attributes[key] = value
	}
	return &Node{
		id:         n.id,
		kind:       n.kind,
		parent:     n.parent,
		hasParent:  n.hasParent,
		children:   slices.Clone(n.children),
		attributes: attributes,
		marks:      slices.Clone(n.marks),
		nextMarkID: n.nextMarkID,
		text:       n.text,
	}
}

func sameNode(left, right *Node) bool {
	if left == right {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	if left.id != right.id ||
		left.kind != right.kind ||
		left.parent != right.parent ||
		left.hasParent != right.hasParent ||
		left.nextMarkID != right.nextMarkID ||
		!slices.Equal(left.children, right.children) ||
		len(left.attributes) != len(right.attributes) ||
		len(left.marks) != len(right.marks) ||
		!sameText(left.text, right.text) {
		return false
	}
	for key, value := range left.attributes {
		if other, ok := right.attributes[key]; !ok || other != value {
			return false
		}
	}
	for index := range left.marks {
		if left.marks[index] != right.marks[index] {
			return false
		}
	}
	return true
}

// TextRoot returns the immutable text-root handle.
func (n *Node) TextRoot() (TextRoot, bool) {
	if n.text == nil {
		return TextRoot{}, false
	}
	return *n.text, true
}

// value is the node as callers see it. Nodes never change once published
// to a state, and every accessor copies what it returns, so the node itself
// is shared rather than copied on each lookup.
func (n *Node) value() *Node {
	return n
}
