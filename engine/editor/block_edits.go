package editor

import (
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// splitAtNewlines breaks node where text inserted at start holds line
// breaks: each break ends a block and the text after it starts the next. The
// line breaks themselves go; the blocks are their boundary now.
func splitAtNewlines(
	document *model.DocumentDraft,
	node model.NodeID,
	start model.UTF16Offset,
	text string,
) error {
	if !strings.ContainsRune(text, '\n') {
		return nil
	}
	units := utf16.Encode([]rune(text))
	for index := len(units) - 1; index >= 0; index-- {
		if units[index] != '\n' {
			continue
		}
		at := start + model.UTF16Offset(index)
		if _, err := document.SplitText(node, at+1); err != nil {
			return err
		}
		if err := document.ReplaceTextMarked(node, at, at+1, "", nil); err != nil {
			return err
		}
	}
	return nil
}

// invokeSpan replaces the text between two positions in different blocks:
// the blocks between go, the first block keeps what precedes the start and
// the last what follows the end, the two join, and the replacement goes in
// where they meet, breaking into blocks at its line breaks.
func (i *EditorInvoker) invokeSpan(context *kernel.RuleContext) (kernel.Outcome, error) {
	span, ok := context.Event().SpanInput()
	if !ok {
		return kernel.OutcomePass, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	first, last := span.From().Node(), span.To().Node()
	start, end := span.From().RangeUTF16().Start, span.To().RangeUTF16().Start
	root, ok := document.Node(document.Root())
	if !ok {
		return kernel.OutcomePass, nil
	}
	order := root.Children()
	from, to := slices.Index(order, first), slices.Index(order, last)
	if from < 0 || to <= from {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: last},
		}
	}
	for _, between := range order[from+1 : to] {
		if err := document.RemoveNode(between); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
	}
	head, _ := document.Node(first)
	if err := document.ReplaceTextMarked(first, start, blockLength(head), "", nil); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if err := document.ReplaceTextMarked(last, 0, end, "", nil); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if err := document.JoinText(first, last); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if err := document.ReplaceTextMarked(first, start, start, span.Text(), i.active); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	i.correctionEffectHead = start + model.UTF16Offset(model.StringLenUTF16(span.Text()))
	i.hasCorrectionEffect = true
	i.correctionEffectThrough = document.TextEditCount()
	if err := splitAtNewlines(document, first, start, span.Text()); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return kernel.OutcomeHandled, nil
}
