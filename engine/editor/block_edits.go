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
		created, err := document.SplitText(node, at+1)
		if err != nil {
			return err
		}
		// Lines broken from a caption are paragraphs: the picture stays
		// with the first.
		if current, _ := document.Node(node); current.Kind() == ImageNode {
			if err := shapeBlock(document, created, blockShape{kind: ParagraphNode}); err != nil {
				return err
			}
		} else if current.Kind() == CheckboxNode {
			// A new item starts unchecked even when the split item is checked.
			shape := shapeOf(current)
			shape.checked = false
			if err := shapeBlock(document, created, shape); err != nil {
				return err
			}
		}
		if err := document.ReplaceTextMarked(node, at, at+1, "", nil); err != nil {
			return err
		}
	}
	return nil
}

// joinSpan replaces the text from start in first to end in last, a later
// block: the blocks between go, the first block keeps what precedes the
// start and the last what follows the end, the two join, and the
// replacement goes in where they meet, breaking into blocks at its line
// breaks. A span that starts on a rule, as a deletion back into it does,
// takes the rule away with it, as a rule holds no text.
func (i *EditorInvoker) joinSpan(
	document *model.DocumentDraft,
	first model.NodeID,
	start model.UTF16Offset,
	last model.NodeID,
	end model.UTF16Offset,
	text string,
) (kernel.Outcome, error) {
	if spansTable(document, first, last) {
		return i.tableSpan(document, first, start, last, end, text)
	}
	if head, ok := document.Node(first); ok && head.Kind() == RuleNode {
		if err := shapeBlock(document, first, blockShape{kind: ParagraphNode}); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
	}
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
	// What replaces a span starts no mark: the marks its ends were in had
	// their notation taken with the span.
	if err := document.ReplaceTextMarked(first, start, start, text, nil); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	i.proposal.land(Landing{
		Head:    start + model.UTF16Offset(model.StringLenUTF16(text)),
		Through: document.TextEditCount(),
	})
	// The joined block breaks at every line break it now holds, the ones
	// code it took in brought too, unless it keeps lines itself.
	if joined, _ := document.Node(first); !blockKeepsLines(joined) {
		if err := splitAtNewlines(document, first, 0, joined.Text().String()); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
	}
	return kernel.OutcomeHandled, nil
}

// invokePairSpan edits mates together across blocks, as one block's edit
// does: where the span cuts a pair, taking one side of it with the first
// block's tail or the last block's head, the side it leaves goes too. The
// mates go before the join; those in the first block stand before the
// span's start, which moves back past them.
func (i *EditorInvoker) invokePairSpan(context *kernel.RuleContext) (kernel.Outcome, error) {
	span, ok := context.Event().SpanInput()
	if !ok || !contextMarkupBool(context, "edit-mates", true) {
		return kernel.OutcomePass, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	first, start := span.From().Node(), span.From().RangeUTF16().Start
	last, end := span.To().Node(), span.To().RangeUTF16().Start
	head, okFirst := document.Node(first)
	tail, okLast := document.Node(last)
	if !okFirst || !okLast {
		return kernel.OutcomePass, nil
	}
	headText, tailText := head.Text().String(), tail.Text().String()
	headMates := pairMates(cutPairs(headText, model.NewUTF16Range(start, blockLength(head))),
		model.NewUTF16Range(start, blockLength(head)))
	tailMates := pairMates(cutPairs(tailText, model.NewUTF16Range(0, end)), model.NewUTF16Range(0, end))
	if len(headMates) == 0 && len(tailMates) == 0 {
		return kernel.OutcomePass, nil
	}
	remove := func(node model.NodeID, mates []model.UTF16Offset) error {
		for index := len(mates) - 1; index >= 0; index-- {
			if err := document.ReplaceTextMarked(node, mates[index], mates[index]+1, "", nil); err != nil {
				return err
			}
		}
		return nil
	}
	if err := remove(last, tailMates); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if err := remove(first, headMates); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return i.joinSpan(document, first, start-model.UTF16Offset(len(headMates)), last, end, span.Text())
}
