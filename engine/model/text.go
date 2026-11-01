package model

import (
	"github.com/caelyreth/neverland/engine/text/rope"
)

// TextRoot is an immutable text version. Pointer identity distinguishes a
// retained text version from a newly constructed root with equal contents.
type TextRoot struct {
	tree rope.Tree
}

// NewTextRoot creates an immutable semantic text root.
func NewTextRoot(text string) TextRoot {
	return TextRoot{tree: rope.New(text)}
}

// Tree returns the underlying immutable rope.
func (root TextRoot) Tree() rope.Tree {
	return root.tree
}

// String materializes the root's complete text.
func (root TextRoot) String() string {
	return root.tree.String()
}

// Same reports whether two roots are the exact same immutable text version.
func (root TextRoot) Same(other TextRoot) bool {
	return root.tree.Same(other.tree)
}

func sameText(left, right *TextRoot) bool {
	if left == right {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	if left.Same(*right) {
		return true
	}
	if left.tree.ByteLen() != right.tree.ByteLen() {
		return false
	}
	return left.tree.String() == right.tree.String()
}

func validateText(text string) error {
	if !ValidDocumentText(text) {
		return invalidDocument("document text must be valid UTF-8 without NUL")
	}
	return nil
}
