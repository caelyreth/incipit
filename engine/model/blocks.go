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
			moved.id = MarkID(second.nextMarkID)
			second.nextMarkID++
			moved.rangeUTF16 = NewUTF16Range(max(start, at)-at, end-at)
			second.marks = append(second.marks, moved)
		}
	}
	if target.text != nil {
		first.text, second.text = &head, &tail
	}
	sortMarks(first.marks, nil)
	sortMarks(second.marks, nil)
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
		moved.id = MarkID(joined.nextMarkID)
		joined.nextMarkID++
		moved.rangeUTF16 = NewUTF16Range(mark.rangeUTF16.Start+offset, mark.rangeUTF16.End+offset)
		joined.marks = append(joined.marks, moved)
	}
	sortMarks(joined.marks, nil)
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
