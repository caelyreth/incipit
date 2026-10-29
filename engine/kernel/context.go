package kernel

import (
	"github.com/caelyreth/neverland/engine/model"
)

// RuleContext is guarded access to the engine draft for one linked rule.
type RuleContext struct {
	draft             *EngineDraft
	resources         ResourceWrite
	effects           EffectSet
	owner             PolicyID
	configuration     *PreparedPolicyConfiguration
	event             Event
	textInputConsumed bool
}

func NewRuleContext(
	draft *EngineDraft,
	rule *LinkedRule,
	event Event,
	configuration *PreparedPolicyConfiguration,
) *RuleContext {
	return &RuleContext{
		draft:         draft,
		resources:     draft.Resources().write(rule.Effects),
		effects:       rule.Effects,
		owner:         rule.Key.Owner,
		configuration: configuration,
		event:         event,
	}
}

func (c *RuleContext) Event() Event { return c.event }

func (c *RuleContext) Owner() PolicyID { return c.owner }

func (c *RuleContext) Changed(key StateKey) (bool, error) {
	if key.Document {
		if _, err := c.Document(); err != nil {
			return false, err
		}
		return c.draft.Document().WasWritten(), nil
	}
	return c.resources.Changed(key.Resource)
}

func (c *RuleContext) Document() (*model.DocumentDraft, error) {
	if c.effects.Document() == DocumentAccessNone {
		return nil, &EngineError{Kind: EngineErrorUndeclaredDocumentRead}
	}
	return c.draft.Document(), nil
}

func (c *RuleContext) DocumentMut() (*model.DocumentDraft, error) {
	if c.effects.Document() != DocumentAccessWrite {
		return nil, &EngineError{Kind: EngineErrorUndeclaredDocumentWrite}
	}
	return c.draft.Document(), nil
}

func (c *RuleContext) Resources() *ResourceWrite { return &c.resources }

func (c *RuleContext) Setting(id SettingID) (SettingValue, bool) {
	if c.configuration == nil {
		return SettingValue{}, false
	}
	return c.configuration.Setting(NewSettingKey(c.owner, id))
}

// ApplyTextInput stores event text literally using declared write authority.
func (c *RuleContext) ApplyTextInput() (Outcome, error) {
	input, ok := c.event.TextInput()
	if !ok {
		return OutcomePass, nil
	}
	if c.effects.Document() != DocumentAccessWrite {
		return 0, &EngineError{Kind: EngineErrorUndeclaredDocumentWrite}
	}
	if c.textInputConsumed || c.draft.textInputConsumed {
		return 0, &EngineError{
			Kind: EngineErrorInput,
			Err:  &InputError{Kind: InputErrorConsumedTarget},
		}
	}
	if err := ValidateTextTargetDraft(input.Target(), c.draft); err != nil {
		return 0, &EngineError{Kind: EngineErrorInput, Err: err}
	}
	c.textInputConsumed = true
	c.draft.textInputConsumed = true
	target := input.Target()
	if err := c.draft.Document().ReplaceTextUTF16(
		target.Node(),
		target.RangeUTF16().Start,
		target.RangeUTF16().End,
		input.Text(),
	); err != nil {
		return 0, &EngineError{Kind: EngineErrorDocument, Err: err}
	}
	return OutcomeHandled, nil
}

func (c *RuleContext) ReplaceTextInputMarked(active *model.MarkID) (model.NodeID, int, bool, error) {
	input, ok := c.event.TextInput()
	if !ok {
		return 0, 0, false, nil
	}
	if c.effects.Document() != DocumentAccessWrite {
		return 0, 0, false, &EngineError{Kind: EngineErrorUndeclaredDocumentWrite}
	}
	if c.textInputConsumed || c.draft.textInputConsumed {
		return 0, 0, false, &EngineError{
			Kind: EngineErrorInput,
			Err:  &InputError{Kind: InputErrorConsumedTarget},
		}
	}
	c.textInputConsumed = true
	c.draft.textInputConsumed = true
	target := input.Target()
	if err := c.draft.Document().ReplaceTextMarked(
		target.Node(),
		target.RangeUTF16().Start,
		target.RangeUTF16().End,
		input.Text(),
		active,
	); err != nil {
		return 0, 0, false, &EngineError{Kind: EngineErrorDocument, Err: err}
	}
	head := int(target.RangeUTF16().Start) + model.StringLenUTF16(input.Text())
	return target.Node(), head, true, nil
}
