package input

import "github.com/caelyreth/neverland/engine/model"

func ValidateTextTargetDraft(target TextTarget, baseRevision uint64, draft *model.DocumentDraft) error {
	if draft.WasWritten() {
		return &Error{Kind: ErrorModifiedDocument}
	}
	return target.Validate(baseRevision, draft.BaseSnapshot())
}

func ValidateAttributeTargetDraft(target AttributeTarget, baseRevision uint64, draft *model.DocumentDraft) error {
	if draft.WasWritten() {
		return &Error{Kind: ErrorModifiedDocument}
	}
	return target.Validate(baseRevision, draft.BaseSnapshot())
}
