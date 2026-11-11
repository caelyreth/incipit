// Package doc is the document: a tree of immutable nodes with typed slots,
// inline text with interned mark sets, positions addressed by node id, the
// seven primitive operations with their mappings and inverses, transactions
// that normalise and validate what they touch, the journal, and history.
//
// It depends on nothing. What kinds and marks exist, and what they accept,
// comes from a Schema the vocabulary declares.
package doc

import "fmt"

// ID is a node's identity: a 64-bit random value, unique within a page.
// Ids label nodes for caches, storage rows, sync, agents and transitions.
type ID uint64

func (id ID) String() string { return fmt.Sprintf("%016x", uint64(id)) }
