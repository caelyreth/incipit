package surface

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

type offsetCheckpoint struct {
	byte  int
	units int
}

// Flow is an immutable flattened projected text plus contiguous style runs and
// sparse UTF-16 checkpoints.
type Flow struct {
	version projection.Version
	text    string
	runs    []StyleRun
	offsets []offsetCheckpoint
	ascii   bool
}

func EmptyFlow(version projection.Version) *Flow {
	return &Flow{version: version, ascii: true}
}

func NewFlow(version projection.Version, text string, runs []StyleRun) (*Flow, error) {
	end := 0
	for _, run := range runs {
		if int(run.Bytes.Start) != end || run.Bytes.Empty() {
			return nil, fmt.Errorf("style runs must be contiguous and non-empty")
		}
		if int(run.Bytes.Start) < 0 || int(run.Bytes.End) > len(text) ||
			!utf8Boundary(text, int(run.Bytes.Start)) ||
			!utf8Boundary(text, int(run.Bytes.End)) {
			return nil, fmt.Errorf("style run has an invalid byte boundary")
		}
		end = int(run.Bytes.End)
	}
	if end != len(text) {
		return nil, fmt.Errorf("style runs do not cover the text")
	}
	ascii := model.StringLenUTF16(text) == len(text)
	flow := &Flow{
		version: version,
		text:    text,
		runs:    append([]StyleRun(nil), runs...),
		ascii:   ascii,
	}
	if !ascii {
		units := 0
		for index, r := range text {
			if index%256 == 0 {
				flow.offsets = append(flow.offsets, offsetCheckpoint{byte: index, units: units})
			}
			units += model.RuneUTF16Len(r)
		}
		flow.offsets = append(flow.offsets, offsetCheckpoint{byte: len(text), units: units})
	}
	return flow, nil
}

func (f *Flow) Version() projection.Version { return f.version }

func (f *Flow) Text() string { return f.text }

func (f *Flow) SharedText() string { return f.text }

func (f *Flow) UTF16Len() model.UTF16Offset {
	if f.ascii {
		return model.UTF16Offset(len(f.text))
	}
	if len(f.offsets) == 0 {
		return 0
	}
	return model.UTF16Offset(f.offsets[len(f.offsets)-1].units)
}

func (f *Flow) Runs() []StyleRun {
	return append([]StyleRun(nil), f.runs...)
}

func (f *Flow) ByteOfUTF16(offset model.UTF16Offset) (model.ByteOffset, bool) {
	target := int(offset)
	if target < 0 || target > int(f.UTF16Len()) {
		return 0, false
	}
	if f.ascii {
		return model.ByteOffset(target), true
	}
	index := checkpointFloor(f.offsets, target, func(checkpoint offsetCheckpoint) int {
		return checkpoint.units
	})
	if index < 0 {
		return 0, false
	}
	checkpoint := f.offsets[index]
	local, ok := model.UTF16OffsetToByte(f.text[checkpoint.byte:], model.UTF16Offset(target-checkpoint.units))
	if !ok {
		return 0, false
	}
	return model.ByteOffset(checkpoint.byte) + local, true
}

func (f *Flow) UTF16OfByte(offset model.ByteOffset) (model.UTF16Offset, bool) {
	byteOffset := int(offset)
	if byteOffset < 0 || byteOffset > len(f.text) || !utf8Boundary(f.text, byteOffset) {
		return 0, false
	}
	if f.ascii {
		return model.UTF16Offset(byteOffset), true
	}
	index := checkpointFloor(f.offsets, byteOffset, func(checkpoint offsetCheckpoint) int {
		return checkpoint.byte
	})
	if index < 0 {
		return 0, false
	}
	checkpoint := f.offsets[index]
	local, ok := model.ByteOffsetToUTF16(f.text[checkpoint.byte:], model.ByteOffset(byteOffset-checkpoint.byte))
	if !ok {
		return 0, false
	}
	return model.UTF16Offset(checkpoint.units) + local, true
}

func (f *Flow) byteRange(r model.UTF16Range) (model.ByteRange, error) {
	if r.Start > r.End {
		return model.ByteRange{}, fmt.Errorf("invalid projected range")
	}
	start, ok := f.ByteOfUTF16(r.Start)
	if !ok {
		return model.ByteRange{}, fmt.Errorf("invalid projected boundary")
	}
	end, ok := f.ByteOfUTF16(r.End)
	if !ok {
		return model.ByteRange{}, fmt.Errorf("invalid projected boundary")
	}
	return model.NewByteRange(start, end), nil
}

func checkpointFloor[T any](items []T, target int, field func(T) int) int {
	low, high := 0, len(items)
	for low < high {
		mid := (low + high) / 2
		if field(items[mid]) <= target {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return low - 1
}

func utf8Boundary(text string, at int) bool {
	return at == len(text) || at >= 0 && at < len(text) && (text[at]&0xc0) != 0x80
}
