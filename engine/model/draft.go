package model

import (
	"slices"

	"github.com/caelyreth/neverland/engine/text/rope"
)

// DocumentDraft is a copy-on-write semantic transaction. Its first write
// copies the node map and later writes change that copy in place, until a
// checkpoint or a candidate snapshot shares it, after which the next write
// copies again. Nodes themselves are immutable: a write replaces the node.
type DocumentDraft struct {
	base      *documentState
	state     *documentState
	owned     bool
	changes   []Change
	textEdits []TextEdit
	written   bool
}

// BaseSnapshot exposes the immutable starting state for checked validation.
// Its revision is supplied by the owning engine transaction.
func (d *DocumentDraft) BaseSnapshot() DocumentSnapshot {
	return DocumentSnapshot{state: d.base}
}

// CandidateSnapshot exposes the candidate state for transaction preparation.
// It must not be published until the caller validates and accepts the draft.
func (d *DocumentDraft) CandidateSnapshot(revision uint64) DocumentSnapshot {
	d.owned = false
	return DocumentSnapshot{state: d.state, revision: revision}
}

// Draft is a concise alias for DocumentDraft.
type Draft = DocumentDraft

// DraftCheckpoint restores document state and journals to one transaction
// position.
type DraftCheckpoint struct {
	state        *documentState
	changesLen   int
	textEditsLen int
	written      bool
}

// WasWritten reports whether any successful write intent was recorded since
// the draft began, including writes later restored.
func (d *DocumentDraft) WasWritten() bool {
	return d.written
}

// TextEdits returns the successful text-operation footprint in operation
// order. It is not a minimized diff.
func (d *DocumentDraft) TextEdits() []TextEdit {
	return slices.Clone(d.textEdits)
}

// TextEditCount returns the successful-operation footprint length.
func (d *DocumentDraft) TextEditCount() int {
	return len(d.textEdits)
}

// ChangeCount returns the number of recorded change intents. Intents are not
// deduplicated; use FinalChanges for the published structural diff.
func (d *DocumentDraft) ChangeCount() int {
	return len(d.changes)
}

// ChangesSince returns change intents recorded at or after one ChangeCount
// snapshot. It lets dispatch associate writes made by one handler.
func (d *DocumentDraft) ChangesSince(start int) []Change {
	start = max(0, min(start, len(d.changes)))
	return slices.Clone(d.changes[start:])
}

// Root returns the current draft's root identity.
func (d *DocumentDraft) Root() NodeID {
	return d.state.root
}

// Node returns the immutable current node value.
func (d *DocumentDraft) Node(id NodeID) (*Node, bool) {
	node, ok := d.state.nodes[id]
	if !ok {
		return nil, false
	}
	return node.value(), true
}

// Checkpoint captures state and all successful-write journals.
func (d *DocumentDraft) Checkpoint() DraftCheckpoint {
	d.owned = false
	return DraftCheckpoint{
		state:        d.state,
		changesLen:   len(d.changes),
		textEditsLen: len(d.textEdits),
		written:      d.written,
	}
}

// Rollback restores a checkpoint exactly.
func (d *DocumentDraft) Rollback(checkpoint DraftCheckpoint) {
	d.state, d.owned = checkpoint.state, false
	d.changes = d.changes[:checkpoint.changesLen]
	d.textEdits = d.textEdits[:checkpoint.textEditsLen]
	d.written = checkpoint.written
}

// FinalChanges derives surviving structural changes ordered by node identity.
// Touched nodes are deduplicated and changes that cancel out are omitted.
func (d *DocumentDraft) FinalChanges() []Change {
	touched := make(map[NodeID]struct{}, len(d.changes))
	for _, change := range d.changes {
		touched[change.Node] = struct{}{}
	}
	ids := make([]NodeID, 0, len(touched))
	for id := range touched {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	changes := make([]Change, 0, len(ids))
	for _, id := range ids {
		before, existedBefore := d.base.nodes[id]
		after, existsAfter := d.state.nodes[id]
		switch {
		case !existedBefore && existsAfter:
			changes = append(changes, Change{Kind: ChangeCreated, Node: id})
		case existedBefore && !existsAfter:
			changes = append(changes, Change{Kind: ChangeRemoved, Node: id})
		case existedBefore && existsAfter && !sameNode(before, after):
			changes = append(changes, Change{Kind: ChangeUpdated, Node: id})
		}
	}
	return changes
}

// Apply executes one semantic action.
func (d *DocumentDraft) Apply(action SemanticAction) (NodeID, bool, error) {
	switch action.Kind {
	case ActionCreateNode:
		id, err := d.CreateNode(action.Parent, action.Index, action.NodeType)
		return id, err == nil, err
	case ActionSetText:
		return 0, false, d.SetText(action.Node, action.Text)
	case ActionSetAttribute:
		return 0, false, d.SetAttribute(action.Node, action.Key, action.Value)
	case ActionRemoveAttribute:
		return 0, false, d.RemoveAttribute(action.Node, action.Key)
	case ActionRemoveNode:
		return 0, false, d.RemoveNode(action.Node)
	default:
		return 0, false, invalidDocument("unknown semantic action")
	}
}

// CreateNode appends or inserts a child with a fresh stable identity.
func (d *DocumentDraft) CreateNode(parent NodeID, index int, kind NodeTypeID) (NodeID, error) {
	if err := validateNodeType(kind); err != nil {
		return 0, err
	}
	parentNode, ok := d.state.nodes[parent]
	if !ok {
		return 0, &DocumentError{Kind: DocumentErrorMissingParent, Parent: parent}
	}
	if index < 0 || index > len(parentNode.children) {
		return 0, &DocumentError{
			Kind:   DocumentErrorChildIndexOutOfBounds,
			Parent: parent,
			Index:  index,
		}
	}
	if d.state.nextID == ^uint64(0) {
		return 0, &DocumentError{Kind: DocumentErrorIdentityExhausted}
	}
	id := NodeID(d.state.nextID)
	nextID := d.state.nextID + 1

	state := d.copyState()
	state.nodes[id] = newNode(id, kind, parent, true)
	state.nodes[parent] = state.nodes[parent].clone()
	children := state.nodes[parent].children
	children = slices.Insert(children, index, id)
	state.nodes[parent].children = children
	state.nextID = nextID
	d.state = state
	d.changes = append(
		d.changes,
		Change{Kind: ChangeUpdated, Node: parent},
		Change{Kind: ChangeCreated, Node: id},
	)
	d.written = true
	return id, nil
}

// SetText replaces all text and clears every inline mark. Equal text and equal
// presence are no-ops that still record a write intent.
func (d *DocumentDraft) SetText(node NodeID, text *string) error {
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	if text != nil {
		value := NormalizeLineEndings(*text)
		if err := validateText(value); err != nil {
			return err
		}
		text = &value
	}
	var replacement *TextRoot
	if text != nil {
		root := NewTextRoot(*text)
		replacement = &root
	}
	d.written = true
	if sameText(target.text, replacement) {
		return nil
	}
	previousLength := 0
	if target.text != nil {
		previousLength = target.text.tree.UTF16Len()
	}
	insertedLength := 0
	if replacement != nil {
		insertedLength = replacement.tree.UTF16Len()
	}

	state := d.copyState()
	updated := state.nodes[node].clone()
	updated.text = replacement
	updated.marks = nil
	state.nodes[node] = updated
	d.state = state
	d.textEdits = append(d.textEdits, NewTextEdit(
		node,
		NewUTF16Range(0, UTF16Offset(previousLength)),
		UTF16Offset(insertedLength),
	))
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return nil
}

// SetTextValue is a pointer-free convenience for SetText.
func (d *DocumentDraft) SetTextValue(node NodeID, text string) error {
	return d.SetText(node, &text)
}

// ClearText removes the node's allocated text root.
func (d *DocumentDraft) ClearText(node NodeID) error {
	return d.SetText(node, nil)
}

// AppendText inserts at the current byte end without materializing prior text.
func (d *DocumentDraft) AppendText(node NodeID, text string) error {
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	text = NormalizeLineEndings(text)
	if err := validateText(text); err != nil {
		return err
	}
	d.written = true
	if text == "" {
		return nil
	}
	offset := 0
	var tree rope.Tree
	if target.text != nil {
		offset = target.text.tree.UTF16Len()
		tree = target.text.tree
	}
	tree = tree.Replace(tree.ByteLen(), tree.ByteLen(), text)
	root := TextRoot{tree: tree}

	state := d.copyState()
	updated := state.nodes[node].clone()
	rebaseMarks(&updated.marks, node, NewUTF16Range(
		UTF16Offset(offset),
		UTF16Offset(offset),
	), text, nil, AffinityRight, AffinityRight)
	updated.text = &root
	state.nodes[node] = updated
	d.state = state
	d.textEdits = append(d.textEdits, NewTextEditReplacement(
		node,
		NewUTF16Range(UTF16Offset(offset), UTF16Offset(offset)),
		text,
	))
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return nil
}

// ReplaceTextUTF16 replaces one scalar-aligned UTF-16 range.
func (d *DocumentDraft) ReplaceTextUTF16(
	node NodeID,
	from UTF16Offset,
	to UTF16Offset,
	replacement string,
) error {
	return d.ReplaceTextMarked(node, from, to, replacement, nil)
}

// ReplaceTextMarked replaces text and controls whether the active mark expands
// across insertions at its boundaries.
func (d *DocumentDraft) ReplaceTextMarked(
	node NodeID,
	from UTF16Offset,
	to UTF16Offset,
	replacement string,
	active *MarkID,
) error {
	return d.ReplaceTextMarkedWithBoundaryAffinity(
		node, from, to, replacement, active, AffinityRight, AffinityRight,
	)
}

// ReplaceTextMarkedWithBoundaryAffinity replaces text and chooses the sides
// that empty inline marks keep at the edit's start and end boundaries.
func (d *DocumentDraft) ReplaceTextMarkedWithBoundaryAffinity(
	node NodeID,
	from UTF16Offset,
	to UTF16Offset,
	replacement string,
	active *MarkID,
	startAffinity, endAffinity Affinity,
) error {
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	replacement = NormalizeLineEndings(replacement)
	if err := validateText(replacement); err != nil {
		return err
	}
	byteRange, err := target.TextByteRangeUTF16(from, to)
	if err != nil {
		return err
	}
	if target.text != nil {
		if target.text.tree.Slice(int(byteRange.Start), int(byteRange.End)) == replacement {
			d.written = true
			return nil
		}
	} else if replacement == "" {
		d.written = true
		return nil
	}

	var tree rope.Tree
	if target.text != nil {
		tree = target.text.tree
	}
	tree = tree.Replace(int(byteRange.Start), int(byteRange.End), replacement)
	root := TextRoot{tree: tree}

	state := d.copyState()
	updated := state.nodes[node].clone()
	rebaseMarks(
		&updated.marks,
		node,
		NewUTF16Range(from, to),
		replacement,
		active,
		startAffinity,
		endAffinity,
	)
	updated.text = &root
	state.nodes[node] = updated
	d.state = state
	d.textEdits = append(d.textEdits, NewTextEditReplacement(
		node,
		NewUTF16Range(from, to),
		replacement,
	))
	d.written = true
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return nil
}

// SetAttribute writes an attribute value. Equal values retain the existing
// state while still recording a successful write intent.
func (d *DocumentDraft) SetAttribute(node NodeID, key string, value AttributeValue) error {
	if key == "" {
		return &DocumentError{Kind: DocumentErrorEmptyAttributeKey}
	}
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	d.written = true
	if existing, ok := target.attributes[key]; ok && existing == value {
		return nil
	}
	state := d.copyState()
	updated := state.nodes[node].clone()
	updated.attributes[key] = value
	state.nodes[node] = updated
	d.state = state
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return nil
}

// RemoveAttribute removes a key when present. Missing attributes still record
// a successful write intent but no final change.
func (d *DocumentDraft) RemoveAttribute(node NodeID, key string) error {
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	d.written = true
	if _, ok := target.attributes[key]; !ok {
		return nil
	}
	state := d.copyState()
	updated := state.nodes[node].clone()
	delete(updated.attributes, key)
	state.nodes[node] = updated
	d.state = state
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return nil
}

// RemoveNode removes a non-root node and its whole subtree.
func (d *DocumentDraft) RemoveNode(node NodeID) error {
	if node == d.state.root {
		return &DocumentError{Kind: DocumentErrorRootMutation, Node: node}
	}
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	parent, ok := target.Parent()
	if !ok {
		return invalidDocument("node " + node.String() + " has no parent")
	}
	if _, ok := d.state.nodes[parent]; !ok {
		return &DocumentError{Kind: DocumentErrorMissingParent, Parent: parent}
	}
	removed := make([]NodeID, 0)
	collectSubtree(d.state.nodes, node, &removed)

	state := d.copyState()
	parentNode := state.nodes[parent].clone()
	parentNode.children = slices.DeleteFunc(parentNode.children, func(child NodeID) bool {
		return child == node
	})
	state.nodes[parent] = parentNode
	for _, id := range removed {
		delete(state.nodes, id)
	}
	d.state = state
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: parent})
	for _, id := range removed {
		d.changes = append(d.changes, Change{Kind: ChangeRemoved, Node: id})
	}
	d.written = true
	return nil
}

// RestoreContent restores one node's text, attributes, and marks while
// preserving other node relationships and never rewinding mark identities.
func (d *DocumentDraft) RestoreContent(source *Node) error {
	if source == nil {
		return invalidDocument("restore source cannot be nil")
	}
	target, ok := d.state.nodes[source.id]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: source.id}
	}
	d.written = true
	previousLength := 0
	if target.text != nil {
		previousLength = target.text.tree.UTF16Len()
	}
	insertedLength := 0
	if source.text != nil {
		insertedLength = source.text.tree.UTF16Len()
	}
	if !sameText(target.text, source.text) {
		d.textEdits = append(d.textEdits, NewTextEdit(
			source.id,
			NewUTF16Range(0, UTF16Offset(previousLength)),
			UTF16Offset(insertedLength),
		))
	}
	state := d.copyState()
	updated := state.nodes[source.id].clone()
	updated.text = source.text
	updated.attributes = make(map[string]AttributeValue, len(source.attributes))
	for key, value := range source.attributes {
		updated.attributes[key] = value
	}
	updated.marks = slices.Clone(source.marks)
	updated.nextMarkID = max(updated.nextMarkID, source.nextMarkID)
	state.nodes[source.id] = updated
	d.state = state
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: source.id})
	return nil
}

// Validate checks root structure, node kinds, parent links, and inline marks.
func (d *DocumentDraft) Validate() error {
	root, ok := d.state.nodes[d.state.root]
	if !ok {
		return invalidDocument("root node is missing")
	}
	if root.hasParent {
		return invalidDocument("root has a parent")
	}
	ids := make([]NodeID, 0, len(d.state.nodes))
	for id := range d.state.nodes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		node := d.state.nodes[id]
		if err := validateNodeType(node.kind); err != nil {
			return err
		}
		if err := validateMarks(node); err != nil {
			return err
		}
		for _, child := range node.children {
			childNode, ok := d.state.nodes[child]
			if !ok {
				return &DocumentError{Kind: DocumentErrorMissingNode, Node: child}
			}
			parent, ok := childNode.Parent()
			if !ok || parent != node.id {
				return invalidDocument(
					"parent link for " + child.String() + " does not point to " + node.id.String(),
				)
			}
		}
	}
	return nil
}

func (d *DocumentDraft) copyState() *documentState {
	if d.owned {
		return d.state
	}
	d.owned = true
	state := &documentState{
		root:   d.state.root,
		nodes:  make(map[NodeID]*Node, len(d.state.nodes)),
		nextID: d.state.nextID,
	}
	for id, node := range d.state.nodes {
		state.nodes[id] = node
	}
	return state
}

func collectSubtree(nodes map[NodeID]*Node, root NodeID, output *[]NodeID) {
	*output = append(*output, root)
	node, ok := nodes[root]
	if !ok {
		return
	}
	for _, child := range node.children {
		collectSubtree(nodes, child, output)
	}
}
