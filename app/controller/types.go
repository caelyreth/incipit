package controller

import (
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
)

// UTF16Range is a half-open range in native UTF-16 code units.
type UTF16Range struct {
	Start int
	End   int
}

// NewUTF16Range creates a controller-owned native range.
func NewUTF16Range(start, end int) UTF16Range {
	return UTF16Range{Start: start, End: end}
}

// Empty reports whether the range has no extent.
func (r UTF16Range) Empty() bool {
	return r.Start >= r.End
}

// Length returns the range extent without turning a reversed range positive.
func (r UTF16Range) Length() int {
	return max(0, r.End-r.Start)
}

// Normalized orders the endpoints without changing a valid range.
func (r UTF16Range) Normalized() UTF16Range {
	return UTF16Range{Start: min(r.Start, r.End), End: max(r.Start, r.End)}
}

// UTF16Selection is the primary native selection and its direction.
// Reversed puts the caret at Range.Start.
type UTF16Selection struct {
	Range    UTF16Range
	Reversed bool
}

// Caret returns the active end of the selection.
func (s UTF16Selection) Caret() int {
	if s.Reversed {
		return s.Range.Start
	}
	return s.Range.End
}

// Point is an element-relative position in device-independent pixels.
type Point struct {
	X float32
	Y float32
}

// Rect is element-relative text or caret geometry.
type Rect struct {
	X float32
	Y float32
	W float32
	H float32
}

// TextInputClient mirrors MyGo's native text-input client semantics without
// importing MyGo types. All ranges are UTF-16 code units of the current
// document, including marked preedit text.
type TextInputClient interface {
	TextForRange(UTF16Range) (string, UTF16Range)
	Selection() UTF16Selection
	MarkedRange() (UTF16Range, bool)
	ReplaceText(*UTF16Range, string)
	SetMarkedText(*UTF16Range, string, UTF16Range)
	UnmarkText()
	BoundsForRange(UTF16Range) (Rect, UTF16Range, bool)
	IndexForPoint(Point) (int, bool)
}

// Geometry supplies retained layout queries for an input client. It receives
// the exact visible text, including a composition preview when one is active.
type Geometry interface {
	BoundsForRange(text string, range16 UTF16Range) (Rect, UTF16Range, bool)
	IndexForPoint(text string, point Point) (int, bool)
}

// VerticalGeometry is the retained-layout capability required by visual-row
// movement. CaretLine returns the caret's current line box in the same native
// coordinate space consumed by IndexForPoint.
type VerticalGeometry interface {
	Geometry
	CaretLine(text string, caret int) (Rect, bool)
}

// Command carries application-owned command routing. Policy commands are
// executed by the session when possible; unrecognized commands are retained
// by the controller for the workspace host.
type Command struct {
	ID     string
	Origin kernel.EventOrigin
	Range  *UTF16Range
	Text   string
}

// Status is the outcome of the latest controller operation.
type Status struct {
	Outcome   kernel.Outcome
	Revision  uint64
	Changed   bool
	Composing bool
	FrameAt   time.Time
}

// Activity is one retained projection publication.
type Activity struct {
	Revision   uint64
	Characters int
	Marks      int
}

// Metrics are semantic counts for the current node, excluding generated
// notation and transient preedit text.
type Metrics struct {
	Characters int
	Words      int
	Paragraphs int
}

var _ TextInputClient = (*DocumentController)(nil)
