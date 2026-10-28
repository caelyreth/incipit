package projection

import (
	"strings"
	"sync"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/text/rope"
)

const inlineCompareBytes = 64

type textPart struct {
	root    rope.Tree
	bytes   model.ByteRange
	range16 model.UTF16Range
}

func wholeTextPart(root rope.Tree) textPart {
	return textPart{
		root:    root,
		bytes:   model.NewByteRange(0, model.ByteOffset(root.ByteLen())),
		range16: model.NewUTF16Range(0, model.UTF16Offset(root.UTF16Len())),
	}
}

func (p textPart) content() string {
	return p.root.Slice(int(p.bytes.Start), int(p.bytes.End))
}

func (p textPart) sameText(other textPart) bool {
	if p.bytes.Len() != other.bytes.Len() || p.range16.Len() != other.range16.Len() {
		return false
	}
	if p.root.Same(other.root) && p.bytes == other.bytes {
		return true
	}
	return p.bytes.Len() <= inlineCompareBytes && p.content() == other.content()
}

type positionedPart struct {
	text       textPart
	byteStart  int
	utf16Start int
}

type projectionTextStorage struct {
	parts    []positionedPart
	byteLen  int
	utf16Len int
	flatOnce sync.Once
	flat     string
}

type projectionText struct {
	storage *projectionTextStorage
}

func textFromString(text string) projectionText {
	output := textFromParts([]textPart{wholeTextPart(rope.New(text))})
	output.storage.flat = text
	output.storage.flatOnce.Do(func() {})
	return output
}

func textFromParts(parts []textPart) projectionText {
	positioned := make([]positionedPart, 0, len(parts))
	byteLen, utf16Len := 0, 0
	for _, part := range parts {
		positioned = append(positioned, positionedPart{
			text:       part,
			byteStart:  byteLen,
			utf16Start: utf16Len,
		})
		byteLen += part.bytes.Len()
		utf16Len += part.range16.Len()
	}
	return projectionText{storage: &projectionTextStorage{
		parts:    positioned,
		byteLen:  byteLen,
		utf16Len: utf16Len,
	}}
}

func (t projectionText) isASCII() bool {
	return t.storage.byteLen == t.storage.utf16Len
}

func (t projectionText) UTF16Len() int {
	return t.storage.utf16Len
}

func (t projectionText) Flat() string {
	t.storage.flatOnce.Do(func() {
		var output strings.Builder
		output.Grow(t.storage.byteLen)
		for _, part := range t.storage.parts {
			output.WriteString(part.text.content())
		}
		t.storage.flat = output.String()
	})
	return t.storage.flat
}

func (t projectionText) ByteOffset(offset int) (int, bool) {
	if offset < 0 || offset > t.storage.utf16Len {
		return 0, false
	}
	if t.isASCII() {
		return offset, true
	}
	if offset == t.storage.utf16Len {
		return t.storage.byteLen, true
	}
	index := 0
	for index < len(t.storage.parts) &&
		t.storage.parts[index].utf16Start+t.storage.parts[index].text.range16.Len() <= offset {
		index++
	}
	if index == len(t.storage.parts) {
		return 0, false
	}
	part := t.storage.parts[index]
	local := part.text.range16.Start + model.UTF16Offset(offset-part.utf16Start)
	byteOffset, ok := part.text.root.CheckedByteOfUTF16(int(local))
	if !ok {
		return 0, false
	}
	return part.byteStart + byteOffset - int(part.text.bytes.Start), true
}

func (t projectionText) Range(start, end int) string {
	if start >= end {
		return ""
	}
	var output strings.Builder
	output.Grow(end - start)
	for _, part := range t.storage.parts {
		partEnd := part.byteStart + part.text.bytes.Len()
		from, to := max(start, part.byteStart), min(end, partEnd)
		if from >= to {
			continue
		}
		base := int(part.text.bytes.Start)
		output.WriteString(part.text.root.Slice(
			base+from-part.byteStart,
			base+to-part.byteStart,
		))
	}
	return output.String()
}

func (t projectionText) Chunks(start, end int, fn func(string) bool) {
	for _, part := range t.storage.parts {
		partEnd := part.byteStart + part.text.bytes.Len()
		from, to := max(start, part.byteStart), min(end, partEnd)
		if from >= to {
			continue
		}
		base := int(part.text.bytes.Start)
		if !fn(part.text.root.Slice(base+from-part.byteStart, base+to-part.byteStart)) {
			return
		}
	}
}

func (t projectionText) changedRanges(previous projectionText) []Patch {
	if t.storage == previous.storage {
		return nil
	}
	oldParts, newParts := previous.storage.parts, t.storage.parts
	if len(oldParts) != len(newParts) {
		return []Patch{{previous: model.NewUTF16Range(0, model.UTF16Offset(previous.UTF16Len())),
			current: model.NewUTF16Range(0, model.UTF16Offset(t.UTF16Len()))}}
	}
	patches := make([]Patch, 0)
	pendingOld, pendingNew := -1, -1
	for index := range oldParts {
		if oldParts[index].text.sameText(newParts[index].text) {
			if pendingOld >= 0 {
				patches = append(patches, Patch{
					previous: model.NewUTF16Range(model.UTF16Offset(pendingOld), model.UTF16Offset(oldParts[index].utf16Start)),
					current:  model.NewUTF16Range(model.UTF16Offset(pendingNew), model.UTF16Offset(newParts[index].utf16Start)),
				})
				pendingOld, pendingNew = -1, -1
			}
			continue
		}
		if pendingOld < 0 {
			pendingOld, pendingNew = oldParts[index].utf16Start, newParts[index].utf16Start
		}
	}
	if pendingOld >= 0 {
		patches = append(patches, Patch{
			previous: model.NewUTF16Range(model.UTF16Offset(pendingOld), model.UTF16Offset(previous.UTF16Len())),
			current:  model.NewUTF16Range(model.UTF16Offset(pendingNew), model.UTF16Offset(t.UTF16Len())),
		})
	}
	return patches
}

func (t *projectionText) retainIfEqual(previous projectionText) bool {
	if t.storage == previous.storage {
		*t = previous
		return true
	}
	if len(t.storage.parts) != len(previous.storage.parts) {
		return false
	}
	for index := range t.storage.parts {
		if !t.storage.parts[index].text.sameText(previous.storage.parts[index].text) {
			return false
		}
	}
	*t = previous
	return true
}

func (t projectionText) Equal(other projectionText) bool {
	if t.storage == other.storage || t.Flat() == other.Flat() {
		return true
	}
	return t.storage.byteLen == other.storage.byteLen && t.Flat() == other.Flat()
}
