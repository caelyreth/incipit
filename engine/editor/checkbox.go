package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// openCheckbox reads the start of a checkable block as the kind of its
// group holding a box: the paired form completes as soon as a state is
// typed between the box's open and close; the complete box followed by a
// space also works when automatic pairing is disabled.
func (i *EditorInvoker) openCheckbox(document *model.DocumentDraft, node *model.Node, at model.UTF16Offset, text string) (kernel.Outcome, bool, error) {
	boxed, ok := boxedKind(kindOf(node).Group)
	if !ok {
		return kernel.OutcomePass, false, nil
	}
	box, content := boxed.Box, node.Text().String()
	r, size := utf8.DecodeRuneInString(text)
	length, checked, absorb := model.UTF16Offset(0), false, ""
	paired := string(box.Open) + string(box.Close)
	switch set, reads := box.reads(r); {
	case at == 1 && size == len(text) && reads && strings.HasPrefix(content, paired):
		// The paired closer and the space after it are the box's: the
		// writer's own are typed into it.
		length, checked, absorb = 2, set, string(box.Close)+" "
	case text == " ":
		set, n, ok := box.parse(content)
		if !ok || at != model.UTF16Offset(model.StringLenUTF16(content[:n])) {
			return kernel.OutcomePass, false, nil
		}
		length, checked = at, set
	default:
		return kernel.OutcomePass, false, nil
	}
	shape := shapeOf(node)
	shape.kind, shape.checked = boxed.Kind, checked
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
