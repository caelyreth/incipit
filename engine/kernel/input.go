package kernel

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

const TextInputKey = "text:input"

// TextTarget is a semantic UTF-16 range captured at an engine revision.
type TextTarget struct {
	node     model.NodeID
	range16  model.UTF16Range
	revision uint64
}

func NewTextTarget(node model.NodeID, range16 model.UTF16Range, revision uint64) TextTarget {
	return TextTarget{node: node, range16: range16, revision: revision}
}

func TextTargetFromAnchor(anchor model.TextAnchor) TextTarget {
	return NewTextTarget(
		anchor.Node(),
		model.EmptyUTF16Range(anchor.OffsetUTF16()),
		anchor.Revision(),
	)
}

func TextTargetFromAnchors(start, end model.TextAnchor) (TextTarget, error) {
	range16, err := start.RangeTo(end)
	if err != nil {
		return TextTarget{}, err
	}
	return NewTextTarget(start.Node(), range16, start.Revision()), nil
}

func TextTargetFromSelection(selection model.TextSelection) (TextTarget, error) {
	return TextTargetFromAnchors(selection.Anchor(), selection.Head())
}

func (t TextTarget) Node() model.NodeID           { return t.node }
func (t TextTarget) RangeUTF16() model.UTF16Range { return t.range16 }
func (t TextTarget) Revision() uint64             { return t.revision }

func (t TextTarget) Anchors() (model.TextAnchor, model.TextAnchor) {
	return model.NewTextAnchor(t.node, t.range16.Start, model.AffinityLeft, t.revision),
		model.NewTextAnchor(t.node, t.range16.End, model.AffinityRight, t.revision)
}

func (t TextTarget) Rebase(edits []model.TextEdit, revision uint64) (TextTarget, bool) {
	start, end := t.Anchors()
	selection, ok := model.NewTextSelection(start, end).RebaseAll(edits, revision)
	if !ok {
		return TextTarget{}, false
	}
	rebased, err := TextTargetFromSelection(selection)
	return rebased, err == nil
}

func (t TextTarget) Validate(snapshot EngineSnapshot) error {
	return t.validate(snapshot.Revision(), snapshot.Document())
}

func (t TextTarget) ValidateDraft(draft *EngineDraft) error {
	if draft.Document().WasWritten() {
		return &InputError{Kind: InputErrorModifiedDocument}
	}
	return t.validate(draft.BaseRevision(), draft.Document().BaseSnapshot())
}

func (t TextTarget) validate(revision uint64, document model.DocumentSnapshot) error {
	if t.revision != revision {
		return &InputError{
			Kind:     InputErrorStaleTarget,
			Captured: t.revision,
			Current:  revision,
		}
	}
	node, ok := document.Node(t.node)
	if !ok {
		return &InputError{
			Kind: InputErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: t.node},
		}
	}
	if err := node.ValidateTextRangeUTF16(t.range16.Start, t.range16.End); err != nil {
		return &InputError{Kind: InputErrorDocument, Err: err}
	}
	return nil
}

// TextInput is a proposed replacement. Policies decide whether to store it
// literally or interpret it as attributes or another semantic effect.
type TextInput struct {
	target TextTarget
	text   string
}

func NewTextInput(target TextTarget, text string) TextInput {
	return TextInput{target: target, text: text}
}

func (i TextInput) Target() TextTarget { return i.target }
func (i TextInput) Text() string       { return i.text }

type InputErrorKind uint8

const (
	InputErrorDocument InputErrorKind = iota + 1
	InputErrorStaleTarget
	InputErrorModifiedDocument
	InputErrorConsumedTarget
	InputErrorCompositionActive
	InputErrorNoComposition
	InputErrorInvalidAttributeInput
)

type InputError struct {
	Kind     InputErrorKind
	Err      error
	Node     model.NodeID
	Key      string
	Captured uint64
	Current  uint64
}

func (e *InputError) Error() string {
	switch e.Kind {
	case InputErrorDocument:
		return e.Err.Error()
	case InputErrorStaleTarget:
		return fmt.Sprintf("input target from revision %d is stale at revision %d", e.Captured, e.Current)
	case InputErrorModifiedDocument:
		return "input target requires a draft without earlier document writes"
	case InputErrorConsumedTarget:
		return "text input has already been consumed by this event"
	case InputErrorCompositionActive:
		return "finish composition before starting new input"
	case InputErrorInvalidAttributeInput:
		return fmt.Sprintf("invalid projected input for %s attribute %s", e.Node, e.Key)
	case InputErrorNoComposition:
		return "no composition is active"
	default:
		return "unknown input error"
	}
}

func (e *InputError) Unwrap() error { return e.Err }
