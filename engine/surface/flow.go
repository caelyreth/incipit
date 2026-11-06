package surface

import (
	"fmt"
	"slices"

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
	version    projection.Version
	text       string
	runs       []StyleRun
	offsets    []offsetCheckpoint
	ascii      bool
	figures    []Figure
	checkboxes []Checkbox
	formulas   []Formula
	tables     []Table
	rooms      []Room
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

// WithFigures is the flow with the pictures its lines show, in order, each
// at the start of a line.
func (f *Flow) WithFigures(figures []Figure) (*Flow, error) {
	previous := model.UTF16Offset(-1)
	for _, figure := range figures {
		if figure.Object.Start <= previous || figure.Object.Empty() || figure.Object.End > f.UTF16Len() ||
			figure.Object.Start > 0 && !f.lineStart(figure.Object.Start) {
			return nil, fmt.Errorf("a figure must start a line, in order, on its object")
		}
		if figure.Width <= 0 || figure.Height <= 0 || figure.Span < 0 || figure.Span > 1 {
			return nil, fmt.Errorf("invalid figure size")
		}
		previous = figure.Object.Start
	}
	next := *f
	next.figures = append([]Figure(nil), figures...)
	return &next, nil
}

func (f *Flow) lineStart(at model.UTF16Offset) bool {
	before, ok := f.ByteOfUTF16(at)
	return ok && before > 0 && f.text[before-1] == '\n'
}

// Figures are the pictures the flow's lines show, in order.
func (f *Flow) Figures() []Figure { return append([]Figure(nil), f.figures...) }

// FigureAt is the picture the line starting at offset shows.
func (f *Flow) FigureAt(at model.UTF16Offset) (Figure, bool) {
	index, found := slices.BinarySearchFunc(f.figures, at, func(figure Figure, at model.UTF16Offset) int {
		return int(figure.Object.Start - at)
	})
	if !found {
		return Figure{}, false
	}
	return f.figures[index], true
}

func (f *Flow) Version() projection.Version { return f.version }

// Equal reports whether two immutable flows have the same content and
// presentation metadata. It includes table geometry and room state, not only
// text, so a renderer may retain layout when a transformation is unchanged.
func (f *Flow) Equal(other *Flow) bool {
	if f == other {
		return true
	}
	if f == nil || other == nil || f.version != other.version || f.text != other.text ||
		!slices.Equal(f.runs, other.runs) || !slices.Equal(f.figures, other.figures) ||
		!slices.Equal(f.checkboxes, other.checkboxes) ||
		!slices.Equal(f.formulas, other.formulas) ||
		!slices.Equal(f.rooms, other.rooms) || len(f.tables) != len(other.tables) {
		return false
	}
	for index := range f.tables {
		if !f.tables[index].equal(other.tables[index]) {
			return false
		}
	}
	return true
}

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

// EachRun visits style runs in document order without cloning the backing
// slice. Callers must not retain or mutate the supplied value beyond callback
// execution.
func (f *Flow) EachRun(fn func(StyleRun) bool) {
	for _, run := range f.runs {
		if !fn(run) {
			return
		}
	}
}

// UTF16Units reports the UTF-16 length of bytes start to end without
// materializing the prefix.
func (f *Flow) UTF16Units(start, end int) int {
	start = max(0, min(start, len(f.text)))
	end = max(0, min(end, len(f.text)))
	if start >= end {
		return 0
	}
	startUnits := 0
	if start > 0 {
		if offset, ok := f.UTF16OfByte(model.ByteOffset(start)); ok {
			startUnits = int(offset)
		}
	}
	endUnits := int(f.UTF16Len())
	if end < len(f.text) {
		if offset, ok := f.UTF16OfByte(model.ByteOffset(end)); ok {
			endUnits = int(offset)
		}
	}
	return max(0, endUnits-startUnits)
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
