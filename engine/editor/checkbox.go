package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// openCheckbox reads the start of a bullet as a checkbox. The paired form
// completes as soon as a space or x is typed between the brackets; the
// complete notation also works when automatic pairing is disabled.
func (i *EditorInvoker) openCheckbox(document *model.DocumentDraft, node *model.Node, at model.UTF16Offset, text string) (kernel.Outcome, bool, error) {
	content := node.Text().String()
	length, checked, absorb := model.UTF16Offset(0), false, ""
	switch {
	case at == 1 && strings.HasPrefix(content, "[]") && (text == " " || text == "x" || text == "X"):
		// The paired closer and the space after it are the box's: the
		// writer's own are typed into it.
		length, checked, absorb = 2, text != " ", "] "
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
	i.proposal.absorbs(absorb)
	return kernel.OutcomeHandled, true, nil
}
