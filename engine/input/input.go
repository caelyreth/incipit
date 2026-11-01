// Package input defines revision-bound semantic targets and checked input
// values shared by the kernel, projection, and editor without package cycles.
package input

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

// TextTarget is a semantic UTF-16 range captured at an engine revision.
type TextTarget struct {
	node                  model.NodeID
	range16               model.UTF16Range
	revision              uint64
	startAffinity         model.Affinity
	endAffinity           model.Affinity
	startPositionAffinity model.Affinity
	endPositionAffinity   model.Affinity
}

func NewTextTarget(node model.NodeID, range16 model.UTF16Range, revision uint64) TextTarget {
	target := TextTarget{
		node:                  node,
		range16:               range16,
		revision:              revision,
		startAffinity:         model.AffinityLeft,
		endAffinity:           model.AffinityRight,
		startPositionAffinity: model.AffinityLeft,
		endPositionAffinity:   model.AffinityRight,
	}
	if range16.Empty() {
		// A zero-width target is one position, so both endpoints move with the
		// same positional affinity. Right matches a default collapsed caret.
		target.startPositionAffinity = model.AffinityRight
		target.endPositionAffinity = model.AffinityRight
	}
	return target
}

// WithAffinity chooses which side of an insertion leaves an empty inline
// mark at the target boundary. It does not choose how the target itself moves
// across later edits; use WithPositionAffinity for that.
func (t TextTarget) WithAffinity(affinity model.Affinity) TextTarget {
	t.startAffinity = affinity
	t.endAffinity = affinity
	return t
}

// WithBoundaryAffinity chooses independent insertion sides for the two
// boundaries of a replacement range.
func (t TextTarget) WithBoundaryAffinity(
	start, end model.Affinity,
) TextTarget {
	t.startAffinity = start
	t.endAffinity = end
	return t
}

// WithPositionAffinity chooses which side of an insertion owns both target
// endpoints when the target is rebased. A collapsed target uses the same side
// for both endpoints and therefore remains collapsed.
func (t TextTarget) WithPositionAffinity(affinity model.Affinity) TextTarget {
	t.startPositionAffinity = affinity
	t.endPositionAffinity = affinity
	return t
}

// WithPositionBoundaryAffinity chooses independent movement sides for a
// nonempty target's start and end endpoints. A collapsed target is one
// position and follows the end affinity, as a collapsed selection does.
func (t TextTarget) WithPositionBoundaryAffinity(
	start, end model.Affinity,
) TextTarget {
	if t.range16.Empty() {
		start = end
	}
	t.startPositionAffinity = start
	t.endPositionAffinity = end
	return t
}

func TextTargetFromAnchor(anchor model.TextAnchor) TextTarget {
	return NewTextTarget(
		anchor.Node(),
		model.EmptyUTF16Range(anchor.OffsetUTF16()),
		anchor.Revision(),
	).
		WithAffinity(anchor.Affinity()).
		WithPositionAffinity(anchor.Affinity())
}

func TextTargetFromAnchors(start, end model.TextAnchor) (TextTarget, error) {
	range16, err := start.RangeTo(end)
	if err != nil {
		return TextTarget{}, err
	}
	if range16.Empty() {
		return TextTargetFromAnchor(end), nil
	}
	low, high := start, end
	if low.OffsetUTF16() > high.OffsetUTF16() {
		low, high = high, low
	}
	return NewTextTarget(start.Node(), range16, start.Revision()).
		WithPositionBoundaryAffinity(low.Affinity(), high.Affinity()), nil
}

func TextTargetFromSelection(selection model.TextSelection) (TextTarget, error) {
	return TextTargetFromAnchors(selection.Anchor(), selection.Head())
}

func (t TextTarget) Node() model.NodeID            { return t.node }
func (t TextTarget) RangeUTF16() model.UTF16Range  { return t.range16 }
func (t TextTarget) Revision() uint64              { return t.revision }
func (t TextTarget) StartAffinity() model.Affinity { return t.startAffinity }
func (t TextTarget) EndAffinity() model.Affinity   { return t.endAffinity }
func (t TextTarget) StartPositionAffinity() model.Affinity {
	return t.startPositionAffinity
}
func (t TextTarget) EndPositionAffinity() model.Affinity {
	return t.endPositionAffinity
}

func (t TextTarget) Anchors() (model.TextAnchor, model.TextAnchor) {
	return model.NewTextAnchor(t.node, t.range16.Start, t.startPositionAffinity, t.revision),
		model.NewTextAnchor(t.node, t.range16.End, t.endPositionAffinity, t.revision)
}

func (t TextTarget) Rebase(edits []model.TextEdit, revision uint64) (TextTarget, bool) {
	start, end := t.Anchors()
	selection, ok := model.NewTextSelection(start, end).RebaseAll(edits, revision)
	if !ok {
		return TextTarget{}, false
	}
	rebased, err := TextTargetFromSelection(selection)
	if err == nil {
		rebased = rebased.WithBoundaryAffinity(t.startAffinity, t.endAffinity)
	}
	return rebased, err == nil
}

func (t TextTarget) Validate(revision uint64, document model.DocumentSnapshot) error {
	if t.revision != revision {
		return &Error{
			Kind:     ErrorStaleTarget,
			Captured: t.revision,
			Current:  revision,
		}
	}
	node, ok := document.Node(t.node)
	if !ok {
		return &Error{
			Kind: ErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: t.node},
		}
	}
	if err := node.ValidateTextRangeUTF16(t.range16.Start, t.range16.End); err != nil {
		return &Error{Kind: ErrorDocument, Err: err}
	}
	return nil
}

// TextInput is a proposed replacement. Policies decide whether to store it
// literally or interpret it semantically.
type TextInput struct {
	target TextTarget
	text   string
}

func NewTextInput(target TextTarget, text string) TextInput {
	return TextInput{target: target, text: text}
}

func (i TextInput) Target() TextTarget { return i.target }
func (i TextInput) Text() string       { return i.text }

type AttributeTarget struct {
	node     model.NodeID
	key      string
	revision uint64
}

func NewAttributeTarget(node model.NodeID, key string, revision uint64) AttributeTarget {
	return AttributeTarget{node: node, key: key, revision: revision}
}

func (t AttributeTarget) Node() model.NodeID { return t.node }
func (t AttributeTarget) Key() string        { return t.key }
func (t AttributeTarget) Revision() uint64   { return t.revision }

func (t AttributeTarget) Validate(revision uint64, document model.DocumentSnapshot) error {
	if t.revision != revision {
		return &Error{
			Kind:     ErrorStaleTarget,
			Captured: t.revision,
			Current:  revision,
		}
	}
	if t.key == "" {
		return &Error{
			Kind: ErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorEmptyAttributeKey},
		}
	}
	if _, ok := document.Node(t.node); !ok {
		return &Error{
			Kind: ErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: t.node},
		}
	}
	return nil
}

type AttributeInput struct {
	target AttributeTarget
	text   string
}

func NewAttributeInput(target AttributeTarget, text string) AttributeInput {
	return AttributeInput{target: target, text: text}
}

func (i AttributeInput) Target() AttributeTarget { return i.target }
func (i AttributeInput) Text() string            { return i.text }
func (i AttributeInput) String() string {
	return fmt.Sprintf("%s:%s", i.target.node, i.target.key)
}

type ErrorKind uint8

const (
	ErrorDocument ErrorKind = iota + 1
	ErrorStaleTarget
	ErrorModifiedDocument
	ErrorConsumedTarget
	ErrorCompositionActive
	ErrorNoComposition
	ErrorInvalidAttributeInput
	ErrorMultipleTextInputs
)

type Error struct {
	Kind     ErrorKind
	Err      error
	Node     model.NodeID
	Key      string
	Captured uint64
	Current  uint64
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorDocument:
		return e.Err.Error()
	case ErrorStaleTarget:
		return fmt.Sprintf("input target from revision %d is stale at revision %d", e.Captured, e.Current)
	case ErrorModifiedDocument:
		return "input target requires a draft without earlier document writes"
	case ErrorConsumedTarget:
		return "text input has already been consumed by this event"
	case ErrorCompositionActive:
		return "finish composition before starting new input"
	case ErrorInvalidAttributeInput:
		return fmt.Sprintf("invalid projected input for %s attribute %s", e.Node, e.Key)
	case ErrorMultipleTextInputs:
		return "an atomic input batch may contain only one primary text input"
	case ErrorNoComposition:
		return "no composition is active"
	default:
		return "unknown input error"
	}
}

func (e *Error) Unwrap() error { return e.Err }

// SpanInput replaces the text from one position to another that lie in
// different nodes, as a selection across blocks does: the blocks between are
// taken out, the two ends join, and text goes in where they meet.
type SpanInput struct {
	from TextTarget
	to   TextTarget
	text string
}

// NewSpanInput spans from from's start to to's start; both are collapsed
// targets, from first in document order.
func NewSpanInput(from, to TextTarget, text string) SpanInput {
	return SpanInput{from: from, to: to, text: text}
}

func (i SpanInput) From() TextTarget { return i.from }
func (i SpanInput) To() TextTarget   { return i.to }
func (i SpanInput) Text() string     { return i.text }
