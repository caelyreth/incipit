package surface

import (
	"fmt"
	"math"

	"github.com/caelyreth/neverland/engine/model"
)

// Frame adds validated selection, caret, input style, and decorations to a
// retained flow.
type Frame struct {
	flow        *Flow
	selection   model.UTF16Range
	caret       model.UTF16Offset
	inputStyle  TextStyle
	decorations []Decoration
	whole       bool
}

func NewFrame(
	flow *Flow,
	selection model.UTF16Range,
	caret model.UTF16Offset,
	inputStyle TextStyle,
	decorations []Decoration,
) (*Frame, error) {
	if flow == nil {
		return nil, fmt.Errorf("surface frame requires a flow")
	}
	if _, err := flow.byteRange(selection); err != nil {
		return nil, err
	}
	if _, err := flow.byteRange(model.EmptyUTF16Range(caret)); err != nil {
		return nil, err
	}
	if caret != selection.Start && caret != selection.End {
		return nil, fmt.Errorf("caret must be a selection edge")
	}
	seen := make(map[DecorationID]struct{}, len(decorations))
	copied := make([]Decoration, 0, len(decorations))
	for _, decoration := range decorations {
		if _, ok := seen[decoration.ID]; ok {
			return nil, fmt.Errorf("duplicate decoration identity")
		}
		seen[decoration.ID] = struct{}{}
		switch decoration.Primitive.Kind {
		case PrimitiveFill:
			if !finiteNonNegative(decoration.Primitive.Radius) {
				return nil, fmt.Errorf("invalid fill radius")
			}
			if err := validateRanges(flow, decoration.Primitive.Ranges); err != nil {
				return nil, err
			}
		case PrimitiveRule:
			if !finiteNonNegative(decoration.Primitive.Thickness) {
				return nil, fmt.Errorf("invalid rule thickness")
			}
			if err := validateRanges(flow, decoration.Primitive.Ranges); err != nil {
				return nil, err
			}
		case PrimitiveArea:
			if len(decoration.Primitive.Ranges) == 0 {
				return nil, fmt.Errorf("area without ranges")
			}
			if err := validateRanges(flow, decoration.Primitive.Ranges); err != nil {
				return nil, err
			}
		case PrimitiveGuide:
			if len(decoration.Primitive.Keys) == 0 {
				return nil, fmt.Errorf("guide without keys")
			}
			if _, err := flow.byteRange(model.EmptyUTF16Range(decoration.Primitive.At)); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown decoration primitive")
		}
		copied = append(copied, decoration.Clone())
	}
	return &Frame{
		flow:        flow,
		selection:   selection,
		caret:       caret,
		inputStyle:  inputStyle,
		decorations: copied,
	}, nil
}

func (f *Frame) Flow() *Flow { return f.flow }

func (f *Frame) Selection() model.UTF16Range { return f.selection }

func (f *Frame) Caret() model.UTF16Offset { return f.caret }

func (f *Frame) InputStyle() TextStyle { return f.inputStyle }

// Whole reports whether the selection takes whole blocks, from the start of
// the first's line to the end of the last's, as selecting a block, a table
// or the page does, rather than text within them.
func (f *Frame) Whole() bool { return f.whole }

// WithWhole is the frame, its selection taking whole blocks as whole says.
func (f *Frame) WithWhole(whole bool) *Frame {
	next := *f
	next.whole = whole
	return &next
}

func (f *Frame) Decorations() []Decoration {
	decorations := make([]Decoration, len(f.decorations))
	for index, decoration := range f.decorations {
		decorations[index] = decoration.Clone()
	}
	return decorations
}

func (f *Frame) WithoutDecorations() *Frame {
	copy := *f
	copy.decorations = nil
	return &copy
}

// WithPreview mounts transient preedit text and a marked underline without
// mutating the published flow or semantic selection. The frame's decorations
// are deliberately replaced: composition owns the marked underline, and
// guides and other hints stay out of its transient surface.
func (f *Frame) WithPreview(
	original model.UTF16Range,
	text string,
	selected model.UTF16Range,
) (*Frame, error) {
	if selected.Start > selected.End || !model.ValidDocumentText(text) {
		return nil, fmt.Errorf("invalid preview selection or text")
	}
	if _, ok := model.UTF16RangeToByte(text, selected); !ok {
		return nil, fmt.Errorf("invalid preview selection")
	}
	flow, err := f.flow.replaceRange(replacement{
		original: original,
		text:     text,
		style:    f.inputStyle,
	})
	if err != nil {
		return nil, err
	}
	marked := model.NewUTF16Range(
		original.Start,
		original.Start+model.UTF16Offset(model.StringLenUTF16(text)),
	)
	decoration := Decoration{
		ID: DecorationID{Producer: ProducerID("surface.preview")},
		Primitive: RulePrimitive(
			[]model.UTF16Range{marked},
			ColorAccent,
			1,
		),
		Opacity:    FixedOpacity(255),
		Visibility: VisibilityAlways,
	}
	return NewFrame(
		flow,
		model.NewUTF16Range(
			original.Start+selected.Start,
			original.Start+selected.End,
		),
		original.Start+selected.End,
		f.inputStyle,
		[]Decoration{decoration},
	)
}

func validateRanges(flow *Flow, ranges []model.UTF16Range) error {
	for _, r := range ranges {
		if _, err := flow.byteRange(r); err != nil {
			return err
		}
	}
	return nil
}

func finiteNonNegative(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0) && value >= 0
}
