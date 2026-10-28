package model

import (
	"errors"
	"fmt"
)

// DocumentErrorKind identifies a semantic document failure.
type DocumentErrorKind uint8

const (
	DocumentErrorEmptyNodeType DocumentErrorKind = iota + 1
	DocumentErrorEmptyAttributeKey
	DocumentErrorMissingNode
	DocumentErrorMissingParent
	DocumentErrorRootMutation
	DocumentErrorChildIndexOutOfBounds
	DocumentErrorInvalidDocument
	DocumentErrorInvalidTextRange
	DocumentErrorInvalidTextBoundary
	DocumentErrorIdentityExhausted
	DocumentErrorRevisionExhausted
)

// DocumentError reports a semantic document or draft operation failure.
type DocumentError struct {
	Kind    DocumentErrorKind
	Node    NodeID
	Parent  NodeID
	Index   int
	From    int
	To      int
	Length  int
	Offset  int
	Message string
}

func (e *DocumentError) Error() string {
	switch e.Kind {
	case DocumentErrorEmptyNodeType:
		return "node type cannot be empty"
	case DocumentErrorEmptyAttributeKey:
		return "attribute key cannot be empty"
	case DocumentErrorMissingNode:
		return fmt.Sprintf("missing node %s", e.Node)
	case DocumentErrorMissingParent:
		return fmt.Sprintf("missing parent %s", e.Parent)
	case DocumentErrorRootMutation:
		return fmt.Sprintf("cannot remove or reparent root %s", e.Node)
	case DocumentErrorChildIndexOutOfBounds:
		return fmt.Sprintf("child index %d is out of bounds for %s", e.Index, e.Parent)
	case DocumentErrorInvalidDocument:
		return e.Message
	case DocumentErrorInvalidTextRange:
		return fmt.Sprintf("text range %d..%d is invalid for %s with length %d", e.From, e.To, e.Node, e.Length)
	case DocumentErrorInvalidTextBoundary:
		return fmt.Sprintf("UTF-16 offset %d splits a character in %s", e.Offset, e.Node)
	case DocumentErrorIdentityExhausted:
		return "document node identities are exhausted"
	case DocumentErrorRevisionExhausted:
		return "document revisions are exhausted"
	default:
		return "unknown document error"
	}
}

// Is matches the same error kind, allowing errors.Is without hiding payload
// details available through errors.As.
func (e *DocumentError) Is(target error) bool {
	var other *DocumentError
	return errors.As(target, &other) && other.Kind == e.Kind
}

func invalidDocument(message string) *DocumentError {
	return &DocumentError{Kind: DocumentErrorInvalidDocument, Message: message}
}
