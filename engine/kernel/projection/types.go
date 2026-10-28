package projection

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

type ProjectionID string

type Target struct {
	projection ProjectionID
	version    Version
	range16    model.UTF16Range
	affinity   model.Affinity
}

func NewTarget(id ProjectionID, version Version, range16 model.UTF16Range) Target {
	return Target{projection: id, version: version, range16: range16, affinity: model.AffinityRight}
}

func (t Target) WithAffinity(affinity model.Affinity) Target {
	t.affinity = affinity
	return t
}

func (t Target) Projection() ProjectionID     { return t.projection }
func (t Target) Version() Version             { return t.version }
func (t Target) RangeUTF16() model.UTF16Range { return t.range16 }
func (t Target) Affinity() model.Affinity     { return t.affinity }

type AttributeCaretSide uint8

const (
	AttributeCaretOpening AttributeCaretSide = iota + 1
	AttributeCaretClosing
)

type CaretKind uint8

const (
	CaretText CaretKind = iota + 1
	CaretAttribute
)

// Caret is a version-independent address inside one projection.
type Caret struct {
	Kind   CaretKind
	Text   model.TextAnchor
	Node   model.NodeID
	Key    string
	Side   AttributeCaretSide
	Offset int
}

func TextCaret(anchor model.TextAnchor) Caret {
	return Caret{Kind: CaretText, Text: anchor}
}

func AttributeCaret(node model.NodeID, key string, side AttributeCaretSide, offset int) Caret {
	return Caret{Kind: CaretAttribute, Node: node, Key: key, Side: side, Offset: offset}
}

type Selection struct {
	anchor Caret
	head   Caret
}

func NewSelection(anchor, head Caret) Selection {
	return Selection{anchor: anchor, head: head}
}

func CollapsedSelection(at Caret) Selection {
	return Selection{anchor: at, head: at}
}

func (s Selection) Anchor() Caret { return s.anchor }
func (s Selection) Head() Caret   { return s.head }

type SegmentKind uint8

const (
	SegmentGenerated SegmentKind = iota + 1
	SegmentEditable
	SegmentSemantic
	SegmentAttribute
)

type Segment struct {
	range16   model.UTF16Range
	kind      SegmentKind
	text      *kernel.TextTarget
	attribute *kernel.AttributeTarget
}

func GeneratedSegment(range16 model.UTF16Range) Segment {
	return Segment{range16: range16, kind: SegmentGenerated}
}

func EditableSegment(range16 model.UTF16Range, target kernel.TextTarget) Segment {
	return Segment{range16: range16, kind: SegmentEditable, text: &target}
}

func SemanticSegment(range16 model.UTF16Range, target kernel.TextTarget) Segment {
	return Segment{range16: range16, kind: SegmentSemantic, text: &target}
}

func AttributeSegment(range16 model.UTF16Range, target kernel.AttributeTarget) Segment {
	return Segment{range16: range16, kind: SegmentAttribute, attribute: &target}
}

func (s Segment) ProjectedRange() model.UTF16Range { return s.range16 }
func (s Segment) Kind() SegmentKind                { return s.kind }

func (s Segment) SemanticTarget() (kernel.TextTarget, bool) {
	if s.text == nil {
		return kernel.TextTarget{}, false
	}
	return *s.text, true
}

func (s Segment) AttributeTarget() (kernel.AttributeTarget, bool) {
	if s.attribute == nil {
		return kernel.AttributeTarget{}, false
	}
	return *s.attribute, true
}

func (s Segment) Editable() bool {
	return s.kind == SegmentEditable || s.kind == SegmentAttribute
}

type InputKind uint8

const (
	InputText InputKind = iota + 1
	InputAttribute
)

type Input struct {
	Kind      InputKind
	Text      kernel.TextInput
	Attribute kernel.AttributeInput
}

func (i Input) Event(origin kernel.EventOrigin) kernel.Event {
	switch i.Kind {
	case InputAttribute:
		return kernel.NewAttributeEvent(i.Attribute, origin)
	default:
		return kernel.NewTextEvent(i.Text, origin)
	}
}

type ErrorKind uint8

const (
	ErrorEmptyID ErrorKind = iota + 1
	ErrorSource
	ErrorInvalidRange
	ErrorInvalidBoundary
	ErrorOutOfBounds
	ErrorOverlap
	ErrorSourceRevision
	ErrorStaleTarget
	ErrorWrongProjection
	ErrorNonEditable
	ErrorNoCorrespondence
	ErrorAmbiguousCorrespondence
	ErrorInvalidMapping
	ErrorLengthOverflow
)

type ProjectionError struct {
	Kind        ErrorKind
	Err         error
	Start       int
	End         int
	Length      int
	Offset      int
	Expected    any
	Actual      any
	PreviousEnd int
	NextStart   int
}

func (e *ProjectionError) Error() string {
	switch e.Kind {
	case ErrorEmptyID:
		return "projection id cannot be empty"
	case ErrorSource:
		return e.Err.Error()
	case ErrorInvalidRange:
		return fmt.Sprintf("invalid projected range %d..%d", e.Start, e.End)
	case ErrorInvalidBoundary:
		return fmt.Sprintf("projected UTF-16 offset %d splits a character", e.Offset)
	case ErrorOutOfBounds:
		return fmt.Sprintf("projected segment %d..%d exceeds text length %d", e.Start, e.End, e.Length)
	case ErrorOverlap:
		return fmt.Sprintf("projected segments overlap at %d before %d", e.NextStart, e.PreviousEnd)
	case ErrorSourceRevision:
		return fmt.Sprintf("projection source revision %v does not match %v", e.Actual, e.Expected)
	case ErrorStaleTarget:
		return fmt.Sprintf("projection target version %v is stale; expected %v", e.Actual, e.Expected)
	case ErrorWrongProjection:
		return fmt.Sprintf("target belongs to %v, expected %v", e.Actual, e.Expected)
	case ErrorNonEditable:
		return "projected range is not editable"
	case ErrorAmbiguousCorrespondence:
		return "semantic anchor has multiple projected locations"
	case ErrorNoCorrespondence:
		return "projected range has no semantic correspondence"
	case ErrorInvalidMapping:
		return "projected and semantic ranges cannot be mapped"
	case ErrorLengthOverflow:
		return "projected text length exceeds addressable memory"
	default:
		return "unknown projection error"
	}
}

func (e *ProjectionError) Unwrap() error { return e.Err }
