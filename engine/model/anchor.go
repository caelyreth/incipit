package model

import "fmt"

// Affinity chooses which side of an insertion owns a boundary.
type Affinity uint8

const (
	AffinityLeft Affinity = iota
	AffinityRight
)

// TextAnchor is a revision-bound semantic position in one node's UTF-16 text.
type TextAnchor struct {
	node     NodeID
	offset   UTF16Offset
	affinity Affinity
	revision uint64
}

// NewTextAnchor captures a semantic position.
func NewTextAnchor(node NodeID, offset UTF16Offset, affinity Affinity, revision uint64) TextAnchor {
	return TextAnchor{
		node:     node,
		offset:   offset,
		affinity: affinity,
		revision: revision,
	}
}

// Node returns the anchor's node identity.
func (a TextAnchor) Node() NodeID {
	return a.node
}

// OffsetUTF16 returns the anchor's UTF-16 offset.
func (a TextAnchor) OffsetUTF16() UTF16Offset {
	return a.offset
}

// Offset is synonymous with OffsetUTF16.
func (a TextAnchor) Offset() UTF16Offset {
	return a.offset
}

// Affinity returns the boundary ownership.
func (a TextAnchor) Affinity() Affinity {
	return a.affinity
}

// Revision returns the captured document revision.
func (a TextAnchor) Revision() uint64 {
	return a.revision
}

// RangeTo returns a normalized range when both anchors share node and revision.
func (a TextAnchor) RangeTo(other TextAnchor) (UTF16Range, error) {
	if a.node != other.node {
		return UTF16Range{}, &AnchorError{
			Kind:   AnchorErrorDifferentNode,
			First:  a.node,
			Second: other.node,
		}
	}
	if a.revision != other.revision {
		return UTF16Range{}, &AnchorError{
			Kind:      AnchorErrorRevisionMismatch,
			FirstRev:  a.revision,
			SecondRev: other.revision,
		}
	}
	return NewUTF16Range(min(a.offset, other.offset), max(a.offset, other.offset)), nil
}

// ValidateDocument checks staleness, node existence, and scalar alignment.
func (a TextAnchor) ValidateDocument(snapshot DocumentSnapshot) error {
	if snapshot.Revision() != a.revision {
		return &AnchorError{
			Kind:     AnchorErrorStale,
			Captured: a.revision,
			Current:  snapshot.Revision(),
		}
	}
	node, ok := snapshot.Node(a.node)
	if !ok {
		return &AnchorError{
			Kind: AnchorErrorDocument,
			Err:  &DocumentError{Kind: DocumentErrorMissingNode, Node: a.node},
		}
	}
	if err := node.ValidateTextRangeUTF16(a.offset, a.offset); err != nil {
		return &AnchorError{Kind: AnchorErrorDocument, Err: err}
	}
	return nil
}

// Rebase maps this anchor across one successful replacement.
func (a TextAnchor) Rebase(edit TextEdit, revision uint64) (TextAnchor, bool) {
	if a.node != edit.Node() {
		a.revision = revision
		return a, true
	}
	offset, ok := edit.mapOffset(a.offset, a.affinity)
	if !ok {
		return TextAnchor{}, false
	}
	a.offset = offset
	a.revision = revision
	return a, true
}

// TextSelection is a directional selection with two semantic anchors.
type TextSelection struct {
	anchor TextAnchor
	head   TextAnchor
}

// NewTextSelection creates a directional selection.
func NewTextSelection(anchor, head TextAnchor) TextSelection {
	return TextSelection{anchor: anchor, head: head}
}

// CollapsedSelection creates a selection with identical endpoints.
func CollapsedSelection(at TextAnchor) TextSelection {
	return TextSelection{anchor: at, head: at}
}

// Anchor returns the non-moving selection endpoint.
func (s TextSelection) Anchor() TextAnchor {
	return s.anchor
}

// Head returns the moving selection endpoint.
func (s TextSelection) Head() TextAnchor {
	return s.head
}

// Node returns the anchor's node identity.
func (s TextSelection) Node() NodeID {
	return s.anchor.node
}

// Revision returns the selection's captured revision.
func (s TextSelection) Revision() uint64 {
	return s.anchor.revision
}

// IsCollapsed reports whether both endpoints have the same UTF-16 offset.
func (s TextSelection) IsCollapsed() bool {
	return s.anchor.offset == s.head.offset
}

// Range returns the normalized semantic range.
func (s TextSelection) Range() (UTF16Range, error) {
	return s.anchor.RangeTo(s.head)
}

// ValidateDocument checks the normalized range and both endpoints.
func (s TextSelection) ValidateDocument(snapshot DocumentSnapshot) error {
	if _, err := s.Range(); err != nil {
		return err
	}
	if err := s.anchor.ValidateDocument(snapshot); err != nil {
		return err
	}
	return s.head.ValidateDocument(snapshot)
}

// Rebase maps both endpoints through one edit.
func (s TextSelection) Rebase(edit TextEdit, revision uint64) (TextSelection, bool) {
	anchor, ok := s.anchor.Rebase(edit, revision)
	if !ok {
		return TextSelection{}, false
	}
	head, ok := s.head.Rebase(edit, revision)
	if !ok {
		return TextSelection{}, false
	}
	return TextSelection{anchor: anchor, head: head}, true
}

// RebaseAll maps through edits in order and always stamps the final revision,
// including for an empty footprint.
func (s TextSelection) RebaseAll(edits []TextEdit, revision uint64) (TextSelection, bool) {
	for _, edit := range edits {
		var ok bool
		s, ok = s.Rebase(edit, revision)
		if !ok {
			return TextSelection{}, false
		}
	}
	s.anchor.revision = revision
	s.head.revision = revision
	return s, true
}

// AnchorErrorKind identifies an anchor contract failure.
type AnchorErrorKind uint8

const (
	AnchorErrorDocument AnchorErrorKind = iota + 1
	AnchorErrorStale
	AnchorErrorDifferentNode
	AnchorErrorRevisionMismatch
)

// AnchorError reports an anchor validation or range failure.
type AnchorError struct {
	Kind      AnchorErrorKind
	Err       error
	Captured  uint64
	Current   uint64
	First     NodeID
	Second    NodeID
	FirstRev  uint64
	SecondRev uint64
}

func (e *AnchorError) Error() string {
	switch e.Kind {
	case AnchorErrorDocument:
		return e.Err.Error()
	case AnchorErrorStale:
		return fmt.Sprintf("anchor from revision %d is stale at revision %d", e.Captured, e.Current)
	case AnchorErrorDifferentNode:
		return fmt.Sprintf("anchors belong to different nodes: %s and %s", e.First, e.Second)
	case AnchorErrorRevisionMismatch:
		return fmt.Sprintf("anchors belong to different revisions: %d and %d", e.FirstRev, e.SecondRev)
	default:
		return "unknown anchor error"
	}
}

func (e *AnchorError) Unwrap() error {
	return e.Err
}

// TextEdit is one semantic replacement in UTF-16 coordinates.
type TextEdit struct {
	node          NodeID
	rangeUTF16    UTF16Range
	insertedUTF16 UTF16Offset
}

// NewTextEdit creates a replacement whose inserted length is already known.
func NewTextEdit(node NodeID, rangeUTF16 UTF16Range, insertedUTF16 UTF16Offset) TextEdit {
	return TextEdit{
		node:          node,
		rangeUTF16:    rangeUTF16,
		insertedUTF16: insertedUTF16,
	}
}

// NewTextEditReplacement creates an edit from replacement text.
func NewTextEditReplacement(node NodeID, rangeUTF16 UTF16Range, replacement string) TextEdit {
	return NewTextEdit(node, rangeUTF16, UTF16Offset(StringLenUTF16(replacement)))
}

// Node returns the edited node identity.
func (e TextEdit) Node() NodeID {
	return e.node
}

// RangeUTF16 returns the replaced semantic range.
func (e TextEdit) RangeUTF16() UTF16Range {
	return e.rangeUTF16
}

// InsertedUTF16Len returns the replacement length.
func (e TextEdit) InsertedUTF16Len() UTF16Offset {
	return e.insertedUTF16
}

// RemovedUTF16Len returns the replaced length, saturating for reversed ranges.
func (e TextEdit) RemovedUTF16Len() UTF16Offset {
	return max(0, e.rangeUTF16.End-e.rangeUTF16.Start)
}

func (e TextEdit) mapOffset(offset UTF16Offset, affinity Affinity) (UTF16Offset, bool) {
	start := e.rangeUTF16.Start
	end := e.rangeUTF16.End
	if start > end {
		return 0, false
	}
	if offset < start {
		return offset, true
	}
	if offset > end {
		return shiftOffset(offset, e.insertedUTF16, e.RemovedUTF16Len())
	}
	if affinity == AffinityLeft {
		return start, true
	}
	if start > UTF16Offset(maxInt)-e.insertedUTF16 {
		return 0, false
	}
	return start + e.insertedUTF16, true
}

func shiftOffset(offset, inserted, removed UTF16Offset) (UTF16Offset, bool) {
	if inserted >= removed {
		delta := inserted - removed
		if offset > UTF16Offset(maxInt)-delta {
			return 0, false
		}
		return offset + delta, true
	}
	delta := removed - inserted
	if offset < delta {
		return 0, false
	}
	return offset - delta, true
}

const maxInt = int(^uint(0) >> 1)
