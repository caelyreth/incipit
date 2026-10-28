package kernel

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/model"
)

const AttributeInputKey = "attribute:input"

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

func (t AttributeTarget) Validate(snapshot EngineSnapshot) error {
	return t.validate(snapshot.Revision(), snapshot.Document())
}

func (t AttributeTarget) ValidateDraft(draft *EngineDraft) error {
	if draft.Document().WasWritten() {
		return &InputError{Kind: InputErrorModifiedDocument}
	}
	return t.validate(draft.BaseRevision(), draft.Document().BaseSnapshot())
}

func (t AttributeTarget) validate(revision uint64, document model.DocumentSnapshot) error {
	if t.revision != revision {
		return &InputError{
			Kind:     InputErrorStaleTarget,
			Captured: t.revision,
			Current:  revision,
		}
	}
	if t.key == "" {
		return &InputError{
			Kind: InputErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorEmptyAttributeKey},
		}
	}
	if _, ok := document.Node(t.node); !ok {
		return &InputError{
			Kind: InputErrorDocument,
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
func (i AttributeInput) String() string          { return fmt.Sprintf("%s:%s", i.target.node, i.target.key) }
