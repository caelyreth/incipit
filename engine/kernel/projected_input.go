package kernel

import "github.com/caelyreth/neverland/engine/kernel/projection"

// ProjectedEditInput is one projected replacement resolved into a canonical
// text intent and ordered attribute intents.
type ProjectedEditInput struct {
	input       TextInput
	attributes  []AttributeInput
	applyText   bool
	interactive bool
	caret       *projection.Caret
}

func NewProjectedEditInput(
	input TextInput,
	attributes []AttributeInput,
	applyText bool,
	interactive bool,
	caret *projection.Caret,
) ProjectedEditInput {
	var caretCopy *projection.Caret
	if caret != nil {
		value := *caret
		caretCopy = &value
	}
	return ProjectedEditInput{
		input:       input,
		attributes:  append([]AttributeInput(nil), attributes...),
		applyText:   applyText,
		interactive: interactive,
		caret:       caretCopy,
	}
}

func NewProjectedTextInput(input TextInput, interactive bool) ProjectedEditInput {
	return NewProjectedEditInput(input, nil, true, interactive, nil)
}

func (p ProjectedEditInput) Input() TextInput { return p.input }

func (p ProjectedEditInput) Attributes() []AttributeInput {
	return append([]AttributeInput(nil), p.attributes...)
}

func (p ProjectedEditInput) ApplyText() bool   { return p.applyText }
func (p ProjectedEditInput) Interactive() bool { return p.interactive }

func (p ProjectedEditInput) Caret() *projection.Caret {
	if p.caret == nil {
		return nil
	}
	value := *p.caret
	return &value
}

func (p ProjectedEditInput) Clone() ProjectedEditInput {
	return NewProjectedEditInput(p.input, p.attributes, p.applyText, p.interactive, p.caret)
}

func (p ProjectedEditInput) ValidateDraft(draft *EngineDraft) error {
	if err := ValidateTextTargetDraft(p.input.Target(), draft); err != nil {
		return err
	}
	for _, input := range p.attributes {
		if err := ValidateAttributeTargetDraft(input.Target(), draft); err != nil {
			return err
		}
	}
	return nil
}
