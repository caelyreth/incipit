package workspace

// SelectionKind is what a page's saved selection selects.
type SelectionKind uint8

const (
	SelectText SelectionKind = iota
	SelectNode
)

// Caret is one end of a saved selection. Node ids are the document's stable
// node ids within the page.
type Caret struct {
	Node   uint64
	Slot   int
	Offset int
	Stop   int
}

// ViewState is a page's selection as the workspace keeps it between
// mounts and launches. A whole-object selection uses Node; a text
// selection uses Anchor and Head, preserving direction.
type ViewState struct {
	Kind   SelectionKind
	Anchor Caret
	Head   Caret
	Node   uint64
}

// Valid reports whether the state names content that can be restored.
func (v ViewState) Valid() bool {
	switch v.Kind {
	case SelectText:
		return v.Anchor.Node != 0 && v.Head.Node != 0
	case SelectNode:
		return v.Node != 0
	default:
		return false
	}
}
