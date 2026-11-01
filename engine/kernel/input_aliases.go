package kernel

import "github.com/caelyreth/neverland/engine/input"

const TextInputKey = "text:input"
const AttributeInputKey = "attribute:input"

// SpanInputKey triggers the rules that replace text across nodes.
const SpanInputKey = "text:span"

type SpanInput = input.SpanInput

var NewSpanInput = input.NewSpanInput

type TextTarget = input.TextTarget
type TextInput = input.TextInput
type AttributeTarget = input.AttributeTarget
type AttributeInput = input.AttributeInput
type InputError = input.Error
type InputErrorKind = input.ErrorKind

const (
	InputErrorDocument              = input.ErrorDocument
	InputErrorStaleTarget           = input.ErrorStaleTarget
	InputErrorModifiedDocument      = input.ErrorModifiedDocument
	InputErrorConsumedTarget        = input.ErrorConsumedTarget
	InputErrorCompositionActive     = input.ErrorCompositionActive
	InputErrorNoComposition         = input.ErrorNoComposition
	InputErrorInvalidAttributeInput = input.ErrorInvalidAttributeInput
	InputErrorMultipleTextInputs    = input.ErrorMultipleTextInputs
)

var NewTextTarget = input.NewTextTarget
var TextTargetFromAnchor = input.TextTargetFromAnchor
var TextTargetFromAnchors = input.TextTargetFromAnchors
var TextTargetFromSelection = input.TextTargetFromSelection
var NewTextInput = input.NewTextInput
var NewAttributeTarget = input.NewAttributeTarget
var NewAttributeInput = input.NewAttributeInput

func ValidateTextTargetDraft(target TextTarget, draft *EngineDraft) error {
	return input.ValidateTextTargetDraft(target, draft.BaseRevision(), draft.Document())
}

func ValidateAttributeTargetDraft(target AttributeTarget, draft *EngineDraft) error {
	return input.ValidateAttributeTargetDraft(target, draft.BaseRevision(), draft.Document())
}
