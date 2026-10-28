package model

import (
	"slices"

	"strings"
)

// InlineMark is semantic emphasis over a scalar-aligned UTF-16 range.
type InlineMark struct {
	id         MarkID
	rangeUTF16 UTF16Range
	level      uint8
}

// NewInlineMark constructs a mark value; drafts enforce identity and validity.
func NewInlineMark(id MarkID, rangeUTF16 UTF16Range, level uint8) InlineMark {
	return InlineMark{id: id, rangeUTF16: rangeUTF16, level: level}
}

// ID returns the stable mark identity.
func (m InlineMark) ID() MarkID {
	return m.id
}

// Range returns the semantic range.
func (m InlineMark) Range() UTF16Range {
	return m.rangeUTF16
}

// RangeUTF16 is synonymous with Range.
func (m InlineMark) RangeUTF16() UTF16Range {
	return m.rangeUTF16
}

// Level returns strength one or two.
func (m InlineMark) Level() uint8 {
	return m.level
}

// AddMark creates or updates a mark for an exact range.
func (d *DocumentDraft) AddMark(node NodeID, rangeUTF16 UTF16Range, level uint8) (MarkID, error) {
	target, ok := d.state.nodes[node]
	if !ok {
		return 0, &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	if err := target.ValidateTextRangeUTF16(rangeUTF16.Start, rangeUTF16.End); err != nil {
		return 0, err
	}
	if level < 1 || level > 2 {
		return 0, invalidDocument("emphasis level must be one or two")
	}
	if target.nextMarkID == ^uint64(0) {
		return 0, &DocumentError{Kind: DocumentErrorIdentityExhausted}
	}
	for _, mark := range target.marks {
		if crosses(mark.rangeUTF16, rangeUTF16) {
			return 0, invalidDocument("emphasis ranges cannot cross")
		}
	}
	for _, mark := range target.marks {
		if mark.rangeUTF16 == rangeUTF16 {
			if err := d.SetMarkLevel(node, mark.id, level); err != nil {
				return 0, err
			}
			return mark.id, nil
		}
	}

	state := d.copyState()
	updated := state.nodes[node].clone()
	id := MarkID(updated.nextMarkID)
	updated.nextMarkID++
	updated.marks = append(updated.marks, InlineMark{
		id:         id,
		rangeUTF16: rangeUTF16,
		level:      level,
	})
	sortMarks(updated.marks, nil)
	state.nodes[node] = updated
	d.state = state
	d.written = true
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return id, nil
}

// SetMarkLevel sets a mark's strength or removes it at level zero.
func (d *DocumentDraft) SetMarkLevel(node NodeID, id MarkID, level uint8) error {
	if level > 2 {
		return invalidDocument("emphasis level exceeds two")
	}
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	index := slices.IndexFunc(target.marks, func(mark InlineMark) bool {
		return mark.id == id
	})
	if index < 0 {
		return invalidDocument("emphasis mark no longer exists")
	}
	d.written = true
	if level == 0 {
		state := d.copyState()
		updated := state.nodes[node].clone()
		updated.marks = slices.Delete(updated.marks, index, index+1)
		state.nodes[node] = updated
		d.state = state
		d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
		return nil
	}
	if target.marks[index].level == level {
		return nil
	}
	state := d.copyState()
	updated := state.nodes[node].clone()
	updated.marks[index].level = level
	state.nodes[node] = updated
	d.state = state
	d.changes = append(d.changes, Change{Kind: ChangeUpdated, Node: node})
	return nil
}

// FormatEmphasis applies or toggles emphasis over a UTF-16 selection.
// Newlines delimit ranges and never acquire marks.
func (d *DocumentDraft) FormatEmphasis(node NodeID, rangeUTF16 UTF16Range, level uint8) error {
	target, ok := d.state.nodes[node]
	if !ok {
		return &DocumentError{Kind: DocumentErrorMissingNode, Node: node}
	}
	if err := target.ValidateTextRangeUTF16(rangeUTF16.Start, rangeUTF16.End); err != nil {
		return err
	}
	if level < 1 || level > 2 {
		return invalidDocument("emphasis level must be one or two")
	}
	if rangeUTF16.Empty() {
		for _, mark := range target.marks {
			if mark.rangeUTF16 == rangeUTF16 && mark.level == level {
				return d.SetMarkLevel(node, mark.id, 0)
			}
		}
		_, err := d.AddMark(node, rangeUTF16, level)
		return err
	}

	source := ""
	if target.text != nil {
		byteRange, err := target.TextByteRangeUTF16(rangeUTF16.Start, rangeUTF16.End)
		if err != nil {
			return err
		}
		source = target.text.tree.Slice(int(byteRange.Start), int(byteRange.End))
	}
	lines := splitNonEmptyLinesUTF16(source, rangeUTF16.Start)
	remove := true
	for _, line := range lines {
		end := line.Start
		for _, mark := range target.marks {
			if mark.level == level && mark.rangeUTF16.Start <= end && end < mark.rangeUTF16.End {
				end = max(end, mark.rangeUTF16.End)
			}
		}
		if end < line.End {
			remove = false
			break
		}
	}
	previous := make([]InlineMark, 0)
	for _, mark := range target.marks {
		if mark.rangeUTF16.Start < rangeUTF16.End && rangeUTF16.Start < mark.rangeUTF16.End {
			previous = append(previous, mark)
		}
	}
	for _, mark := range previous {
		if err := d.SetMarkLevel(node, mark.id, 0); err != nil {
			return err
		}
		if mark.rangeUTF16.Start < rangeUTF16.Start {
			if _, err := d.AddMark(node, NewUTF16Range(mark.rangeUTF16.Start, rangeUTF16.Start), mark.level); err != nil {
				return err
			}
		}
		if rangeUTF16.End < mark.rangeUTF16.End {
			if _, err := d.AddMark(node, NewUTF16Range(rangeUTF16.End, mark.rangeUTF16.End), mark.level); err != nil {
				return err
			}
		}
	}
	if !remove {
		for _, line := range lines {
			if _, err := d.AddMark(node, line, level); err != nil {
				return err
			}
		}
	}
	return nil
}

func splitNonEmptyLinesUTF16(text string, start UTF16Offset) []UTF16Range {
	ranges := make([]UTF16Range, 0)
	offset := start
	for len(text) > 0 {
		newline := strings.IndexByte(text, '\n')
		line := text
		if newline >= 0 {
			line = text[:newline+1]
		}
		content := strings.TrimRight(line, "\r\n")
		contentLength := UTF16Offset(StringLenUTF16(content))
		if contentLength > 0 {
			ranges = append(ranges, NewUTF16Range(offset, offset+contentLength))
		}
		lineLength := UTF16Offset(StringLenUTF16(line))
		offset += lineLength
		text = text[len(line):]
	}
	return ranges
}

func crosses(a, b UTF16Range) bool {
	overlap := a.Start < b.End && b.Start < a.End
	aContains := a.Start <= b.Start && b.End <= a.End
	bContains := b.Start <= a.Start && a.End <= b.End
	return overlap && !aContains && !bContains
}

func rebaseMarks(
	marks *[]InlineMark,
	node NodeID,
	rangeUTF16 UTF16Range,
	text string,
	active *MarkID,
) {
	edit := NewTextEditReplacement(node, rangeUTF16, text)
	var activeRange *UTF16Range
	for _, mark := range *marks {
		if active != nil && mark.id == *active {
			value := mark.rangeUTF16
			activeRange = &value
			break
		}
	}
	for index := range *marks {
		mark := &(*marks)[index]
		expanding := active != nil && mark.id == *active
		if activeRange != nil &&
			mark.rangeUTF16.Start <= activeRange.Start &&
			activeRange.End <= mark.rangeUTF16.End {
			expanding = true
		}
		startAffinity := AffinityRight
		if expanding {
			startAffinity = AffinityLeft
		}
		endAffinity := AffinityLeft
		if expanding {
			endAffinity = AffinityRight
		}
		start := NewTextAnchor(node, mark.rangeUTF16.Start, startAffinity, 0)
		end := NewTextAnchor(node, mark.rangeUTF16.End, endAffinity, 0)
		start, startOK := start.Rebase(edit, 0)
		end, endOK := end.Rebase(edit, 0)
		if startOK && endOK {
			mark.rangeUTF16 = NewUTF16Range(
				min(start.offset, end.offset),
				end.offset,
			)
		}
	}
	sortMarks(*marks, active)
	dedupeMarks(marks)
}

func sortMarks(marks []InlineMark, active *MarkID) {
	slices.SortFunc(marks, func(left, right InlineMark) int {
		if left.rangeUTF16.Start != right.rangeUTF16.Start {
			return compare(left.rangeUTF16.Start, right.rangeUTF16.Start)
		}
		if left.rangeUTF16.End != right.rangeUTF16.End {
			return compare(right.rangeUTF16.End, left.rangeUTF16.End)
		}
		leftActive := active != nil && left.id == *active
		rightActive := active != nil && right.id == *active
		if leftActive != rightActive {
			if leftActive {
				return -1
			}
			return 1
		}
		return compare(left.id, right.id)
	})
}

func compare[T ~int | ~uint64 | ~uint8](left, right T) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func dedupeMarks(marks *[]InlineMark) {
	output := (*marks)[:0]
	for _, mark := range *marks {
		if len(output) > 0 && output[len(output)-1].rangeUTF16 == mark.rangeUTF16 {
			output[len(output)-1].level = max(output[len(output)-1].level, mark.level)
			continue
		}
		output = append(output, mark)
	}
	*marks = output
}

func validateMarks(node *Node) error {
	ends := make([]UTF16Offset, 0, len(node.marks))
	identities := make(map[MarkID]struct{}, len(node.marks))
	for _, mark := range node.marks {
		if err := node.ValidateTextRangeUTF16(mark.rangeUTF16.Start, mark.rangeUTF16.End); err != nil {
			return err
		}
		if mark.level < 1 || mark.level > 2 ||
			uint64(mark.id) >= node.nextMarkID {
			return invalidDocument("invalid inline mark identity or strength")
		}
		if _, exists := identities[mark.id]; exists {
			return invalidDocument("invalid inline mark identity or strength")
		}
		identities[mark.id] = struct{}{}
		for len(ends) > 0 && ends[len(ends)-1] <= mark.rangeUTF16.Start {
			ends = ends[:len(ends)-1]
		}
		if len(ends) > 0 && mark.rangeUTF16.End > ends[len(ends)-1] {
			return invalidDocument("inline ranges cross after editing")
		}
		ends = append(ends, mark.rangeUTF16.End)
	}
	return nil
}
