package model

import "slices"

// SetNodeKind changes what a node is, keeping its text, marks and place:
// a paragraph becomes a heading as its marker is typed.
func (d *DocumentDraft) SetNodeKind(node NodeID, kind NodeTypeID) error {
	if err := validateNodeType(kind); err != nil {
		return err
	}
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	d.written = true
	if target.kind == kind {
		return nil
	}
	state := d.copyState()
	updated := state.nodes[node].clone()
	updated.kind = kind
	state.nodes[node] = updated
	d.state = state
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return nil
}

// SplitText splits node's text at a UTF-16 offset: the text from there on,
// with its marks, moves into a new node placed right after it, of the same
// kind and attributes. A mark across the split continues on both sides.
// Positions in the moved text follow it, and one at the split goes by its
// affinity.
func (d *DocumentDraft) SplitText(node NodeID, at UTF16Offset) (NodeID, error) {
	target, ok := d.state.nodes[node]
	if !ok {
		return 0, &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	parent, ok := target.Parent()
	if !ok {
		return 0, &DocumentError{Kind: DocumentErrorRootMutation, Node: node}
	}
	byteRange, err := target.TextByteRangeUTF16(at, UTF16Offset(target.Text().UTF16Len()))
	if err != nil {
		return 0, err
	}
	movedMarks := 0
	for _, mark := range target.marks {
		if mark.rangeUTF16.End > at {
			movedMarks++
		}
	}
	// The new node starts with the same mark-identity supply as any new node.
	if err := checkNextMarkIDSupply(1, movedMarks); err != nil {
		return 0, err
	}
	index := slices.Index(d.state.nodes[parent].children, node)
	created, err := d.CreateNode(parent, index+1, target.kind)
	if err != nil {
		return 0, err
	}
	length := UTF16Offset(target.Text().UTF16Len())
	tree := target.Text()
	head, tail := NewTextRoot(tree.Slice(0, int(byteRange.Start))), NewTextRoot(tree.Slice(int(byteRange.Start), tree.ByteLen()))

	state := d.copyState()
	first := state.nodes[node].clone()
	second := state.nodes[created].clone()
	for key, value := range first.attributes {
		second.attributes[key] = value
	}
	first.marks = first.marks[:0]
	for _, mark := range target.marks {
		start, end := mark.rangeUTF16.Start, mark.rangeUTF16.End
		if start < at || start == at && end == at {
			kept := mark
			kept.rangeUTF16 = NewUTF16Range(start, min(end, at))
			first.marks = append(first.marks, kept)
		}
		if end > at {
			moved := mark
			moved.id, err = allocateMarkID(second)
			if err != nil {
				return 0, err
			}
			moved.rangeUTF16 = NewUTF16Range(max(start, at)-at, end-at)
			second.marks = append(second.marks, moved)
		}
	}
	if target.text != nil {
		first.text, second.text = &head, &tail
	}
	canonicalizeMarks(&first.marks, nil)
	canonicalizeMarks(&second.marks, nil)
	state.nodes[node], state.nodes[created] = first, second
	d.state = state
	d.textEdits = append(d.textEdits,
		NewTextTransfer(node, at, created, 0, false),
		NewTextEdit(node, NewUTF16Range(at, length), 0),
	)
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return created, nil
}

// JoinText appends second's text and marks to first and removes second, as
// two blocks become one. Positions in second follow its text into first.
func (d *DocumentDraft) JoinText(first, second NodeID) error {
	left, ok := d.state.nodes[first]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: first}
	}
	right, ok := d.state.nodes[second]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: second}
	}
	if first == second || len(right.children) > 0 {
		return invalidDocument("only a different, childless node can be joined")
	}
	if second == d.state.root {
		return &DocumentError{Kind: DocumentErrorRootMutation, Node: second}
	}
	parent, ok := right.Parent()
	if !ok {
		return invalidDocument("only a child node can be joined")
	}
	if _, ok := d.state.nodes[parent]; !ok {
		return &DocumentError{Kind: DocumentErrorMissingParent, Parent: parent}
	}
	if err := checkMarkIDSupply(left, len(right.marks)); err != nil {
		return err
	}
	offset := UTF16Offset(left.Text().UTF16Len())
	state := d.copyState()
	joined := state.nodes[first].clone()
	if left.text != nil || right.text != nil {
		tree := left.Text()
		root := TextRoot{tree: tree.Replace(tree.ByteLen(), tree.ByteLen(), right.Text().String())}
		joined.text = &root
	}
	for _, mark := range right.marks {
		moved := mark
		id, err := allocateMarkID(joined)
		if err != nil {
			return err
		}
		moved.id = id
		moved.rangeUTF16 = NewUTF16Range(mark.rangeUTF16.Start+offset, mark.rangeUTF16.End+offset)
		joined.marks = append(joined.marks, moved)
	}
	canonicalizeMarks(&joined.marks, nil)
	state.nodes[first] = joined
	d.state = state
	d.textEdits = append(d.textEdits, NewTextTransfer(second, 0, first, offset, true))
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: first})
	return d.RemoveNode(second)
}

// RestoreDocument returns the draft to a snapshot's nodes, kinds, text,
// marks and order, as an undo does. Identities never rewind: nodes and marks
// made since keep the supply ahead of them.
func (d *DocumentDraft) RestoreDocument(source DocumentSnapshot) error {
	if source.state == nil || source.state.root != d.state.root {
		return invalidDocument("a document restores only its own snapshot")
	}
	state := d.copyState()
	for id := range d.state.nodes {
		if _, kept := source.state.nodes[id]; !kept {
			delete(state.nodes, id)
			d.changes = append(d.changes, Change{Kind: ChangeRemoved, Node: id})
		}
	}
	for id, restored := range source.state.nodes {
		current, existed := d.state.nodes[id]
		if existed && sameNode(current, restored) {
			continue
		}
		node := restored.clone()
		if existed {
			node.nextMarkID = max(node.nextMarkID, current.nextMarkID)
			if !sameText(current.text, restored.text) {
				d.textEdits = append(d.textEdits, NewTextEdit(
					id,
					NewUTF16Range(0, UTF16Offset(current.Text().UTF16Len())),
					UTF16Offset(restored.Text().UTF16Len()),
				))
			}
			d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: id})
		} else {
			d.changes = append(d.changes, Change{Kind: ChangeCreated, Node: id})
		}
		state.nodes[id] = node
	}
	state.nextID = max(state.nextID, source.state.nextID)
	d.state = state
	d.written = true
	return nil
}

// MoveNode moves node to index among its parent's children, counted
// without it, keeping its identity, text and marks: a table's row goes
// where it is dragged, the caret in it going along.
func (d *DocumentDraft) MoveNode(node NodeID, index int) error {
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	parent, ok := target.Parent()
	if !ok {
		return &DocumentError{Kind: DocumentErrorRootMutation, Node: node}
	}
	children := slices.DeleteFunc(slices.Clone(d.state.nodes[parent].children), func(child NodeID) bool {
		return child == node
	})
	if index < 0 || index > len(children) {
		return &DocumentError{Kind: DocumentErrorChildIndexOutOfBounds, Parent: parent, Index: index}
	}
	d.written = true
	state := d.copyState()
	container := state.nodes[parent].clone()
	container.children = slices.Insert(children, index, node)
	state.nodes[parent] = container
	d.state = state
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: parent})
	return nil
}
