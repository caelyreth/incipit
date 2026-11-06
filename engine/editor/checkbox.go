package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

const checkboxTrigger = "text:checkbox"

// openCheckbox reads the start of a bullet as a checkbox. The paired form
// completes as soon as a space or x is typed between the brackets; the
// complete notation also works when automatic pairing is disabled.
func (i *EditorInvoker) openCheckbox(document *model.DocumentDraft, node *model.Node, at model.UTF16Offset, text string) (kernel.Outcome, bool, error) {
	content := node.Text().String()
	length, checked := model.UTF16Offset(0), false
	switch {
	case at == 1 && strings.HasPrefix(content, "[]") && (text == " " || text == "x" || text == "X"):
		length, checked = 2, text != " "
	case at == 3 && text == " " && (strings.HasPrefix(content, "[ ]") || strings.HasPrefix(content, "[x]") || strings.HasPrefix(content, "[X]")):
		length, checked = 3, content[1] != ' '
	default:
		return kernel.OutcomePass, false, nil
	}
	shape := shapeOf(node)
	shape.kind, shape.checked = CheckboxNode, checked
	if err := shapeBlock(document, node.ID(), shape); err != nil {
		return 0, true, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if err := document.ReplaceTextMarked(node.ID(), 0, length, "", nil); err != nil {
		return 0, true, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	i.land(node.ID(), 0, document)
	return kernel.OutcomeHandled, true, nil
}

// SetCheckbox changes an existing item's state through the core text policy,
// independently of whether the writing policy's typing triggers are enabled.
func (s *Session) SetCheckbox(node model.NodeID, checked bool) (kernel.Outcome, error) {
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	return s.dispatchSelectionCommandWithVisual(checkboxTrigger,
		&checkboxInvoker{session: s, node: node, checked: checked},
		func(*model.Node) *model.MarkID { return activeMark(s.inline) }, true)
}

type checkboxInvoker struct {
	session *Session
	node    model.NodeID
	checked bool
}

func (i *checkboxInvoker) Invoke(rule *kernel.LinkedRule, leaf kernel.LeafID, context *kernel.RuleContext) (kernel.Outcome, error) {
	if rule == nil || rule.Key.Owner != TextPolicyID || rule.Key.ID != "checkbox" {
		return i.session.invoker.Invoke(rule, leaf, context)
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	node, ok := document.Node(i.node)
	if !ok || node.Kind() != CheckboxNode {
		return kernel.OutcomePass, nil
	}
	shape := shapeOf(node)
	if shape.checked == i.checked {
		return kernel.OutcomePass, nil
	}
	shape.checked = i.checked
	if err := shapeBlock(document, node.ID(), shape); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return kernel.OutcomeHandled, nil
}
