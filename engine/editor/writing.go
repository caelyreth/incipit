package editor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// The Writing Profile is the CommonMark-shaped syntax the writing policy
// reads and writes. A page keeps only what its parts are: a heading and its
// level, a quote, a list item, a code block and its info string, a mark's
// kind and level. The notation that shows them ("## ", "> ", "*", a fence)
// is projected from that and never stored. Typing notation where it reads
// as syntax turns it into structure; editing projected notation parses it
// back, and notation that no longer reads as syntax stays as the literal
// text it shows.
const (
	// HeadingNode is a heading block; its "level" attribute is 1 to 6.
	HeadingNode model.NodeTypeID = "heading"
	// QuoteNode is a block quote.
	QuoteNode model.NodeTypeID = "quote"
	// BulletNode is an item of a bullet list.
	BulletNode model.NodeTypeID = "bullet"
	// OrderedNode is an item of an ordered list, numbered by its place in
	// the run of items it stands in.
	OrderedNode model.NodeTypeID = "ordered"
	// CodeNode is a fenced code block; its text keeps its line breaks and
	// its "info" attribute names the language.
	CodeNode model.NodeTypeID = "code"
	// RuleNode is a thematic break.
	RuleNode model.NodeTypeID = "rule"

	// CodeMark is inline code.
	CodeMark model.MarkKind = "code"
	// StrikeMark is struck-through text.
	StrikeMark model.MarkKind = "strike"

	headingLevelAttribute = "level"
	codeInfoAttribute     = "info"

	// blockNotationKey names the notation a block opens with, and
	// fenceNotationKey the fence a code block closes with.
	blockNotationKey = "block"
	fenceNotationKey = "fence"

	headingsSetting    = "headings"
	blockquotesSetting = "blockquotes"
	listsSetting       = "lists"
	codeSetting        = "code"
	strikeSetting      = "strikethrough"
	rulesSetting       = "rules"
)

// writingSyntax is the profile's syntax as the writing policy's settings
// shape it.
type writingSyntax struct {
	allowStrong bool
	underscores bool
	headings    bool
	quotes      bool
	lists       bool
	code        bool
	strike      bool
	rules       bool
}

func syntaxFrom(setting func(id string, fallback bool) bool) writingSyntax {
	return writingSyntax{
		allowStrong: setting(allowStrongSetting, true),
		underscores: setting(underscoresSetting, false),
		headings:    setting(headingsSetting, true),
		quotes:      setting(blockquotesSetting, true),
		lists:       setting(listsSetting, true),
		code:        setting(codeSetting, true),
		strike:      setting(strikeSetting, true),
		rules:       setting(rulesSetting, true),
	}
}

func configuredSyntax(configuration *kernel.PreparedPolicyConfiguration) writingSyntax {
	return syntaxFrom(func(id string, fallback bool) bool {
		return markupSettingBool(configuration, id, fallback)
	})
}

func contextSyntax(context *kernel.RuleContext) writingSyntax {
	return syntaxFrom(func(id string, fallback bool) bool {
		return contextMarkupBool(context, id, fallback)
	})
}

func writingSettingDescriptors() []kernel.SettingDescriptor {
	return []kernel.SettingDescriptor{
		{ID: allowStrongSetting, Default: kernel.BoolSetting(true)},
		{ID: underscoresSetting, Default: kernel.BoolSetting(false)},
		{ID: headingsSetting, Default: kernel.BoolSetting(true)},
		{ID: blockquotesSetting, Default: kernel.BoolSetting(true)},
		{ID: listsSetting, Default: kernel.BoolSetting(true)},
		{ID: codeSetting, Default: kernel.BoolSetting(true)},
		{ID: strikeSetting, Default: kernel.BoolSetting(true)},
		{ID: rulesSetting, Default: kernel.BoolSetting(true)},
	}
}

// markerKind is the mark a typed marker opens, and the highest level the
// mark goes to.
func (w writingSyntax) markerKind(text string) (model.MarkKind, uint8, bool) {
	switch text {
	case "*", "_":
		if w.allowStrong {
			return EmphasisMark, 2, true
		}
		return EmphasisMark, 1, true
	case "`":
		return CodeMark, 1, w.code
	case "~":
		return StrikeMark, 1, w.strike
	}
	return "", 0, false
}

// inlineNotation is how a mark shows on either side of its text.
func (w writingSyntax) inlineNotation(mark model.InlineMark) string {
	switch mark.Kind() {
	case EmphasisMark:
		marker := "*"
		if w.underscores {
			marker = "_"
		}
		return strings.Repeat(marker, int(mark.Level()))
	case CodeMark:
		return "`"
	case StrikeMark:
		return "~"
	}
	return ""
}

// inlineLevel reads edited notation of a mark back to its level; zero takes
// the mark away.
func (w writingSyntax) inlineLevel(kind model.MarkKind, text string) (uint8, bool) {
	if text == "" {
		return 0, true
	}
	if kind != EmphasisMark {
		probe := model.NewInlineMark(0, model.UTF16Range{}, kind, 1)
		return 1, text == w.inlineNotation(probe)
	}
	marker := w.inlineNotation(model.NewInlineMark(0, model.UTF16Range{}, EmphasisMark, 1))
	if strings.Trim(text, marker) != "" {
		return 0, false
	}
	if len(text) > 2 || len(text) == 2 && !w.allowStrong {
		return 0, false
	}
	return uint8(len(text)), true
}

// inlineMarkerRune reports whether r is notation of some inline mark.
func inlineMarkerRune(r rune) bool {
	return r == '*' || r == '_' || r == '`' || r == '~'
}

// blockShape is what a block is, apart from its text.
type blockShape struct {
	kind  model.NodeTypeID
	level int
	info  string
}

// shapeOf reads a block's shape from the page.
func shapeOf(node *model.Node) blockShape {
	shape := blockShape{kind: node.Kind()}
	switch shape.kind {
	case HeadingNode:
		shape.level = 1
		if value, ok := node.Attribute(headingLevelAttribute); ok {
			if level, ok := value.Integer(); ok {
				shape.level = int(max(1, min(6, level)))
			}
		}
	case CodeNode:
		if value, ok := node.Attribute(codeInfoAttribute); ok {
			shape.info, _ = value.Text()
		}
	}
	return shape
}

// opensLine reports whether a shape's notation is a prefix its text follows
// on the same line, rather than a line of its own.
func (s blockShape) opensLine() bool {
	switch s.kind {
	case HeadingNode, QuoteNode, BulletNode, OrderedNode:
		return true
	}
	return false
}

// blockNotation is the notation a block opens with, and the fence it closes
// with. ordinal is an ordered item's number.
func blockNotation(shape blockShape, ordinal int) (open, close string) {
	switch shape.kind {
	case HeadingNode:
		return strings.Repeat("#", shape.level) + " ", ""
	case QuoteNode:
		return "> ", ""
	case BulletNode:
		return "- ", ""
	case OrderedNode:
		return strconv.Itoa(ordinal) + ". ", ""
	case CodeNode:
		return "```" + shape.info + "\n", "\n```"
	case RuleNode:
		return "---", ""
	}
	return "", ""
}

// parseOpener reads notation at a block's start as the shape it opens: the
// inverse of blockNotation over what the profile allows.
func (w writingSyntax) parseOpener(text string) (blockShape, bool) {
	paragraph := blockShape{kind: ParagraphNode}
	switch {
	case text == "":
		return paragraph, true
	case w.headings && strings.HasSuffix(text, " ") &&
		len(text) <= 7 && strings.Trim(text[:len(text)-1], "#") == "" && len(text) > 1:
		return blockShape{kind: HeadingNode, level: len(text) - 1}, true
	case w.quotes && text == "> ":
		return blockShape{kind: QuoteNode}, true
	case w.lists && (text == "- " || text == "+ "):
		return blockShape{kind: BulletNode}, true
	case w.lists && strings.HasSuffix(text, ". ") && len(text) > 2 && len(text) <= 11 &&
		strings.Trim(text[:len(text)-2], "0123456789") == "":
		return blockShape{kind: OrderedNode}, true
	case w.code && strings.HasPrefix(text, "```") && strings.HasSuffix(text, "\n") &&
		!strings.ContainsAny(text[3:len(text)-1], "`\n"):
		return blockShape{kind: CodeNode, info: strings.TrimSpace(text[3 : len(text)-1])}, true
	case w.rules && text == "---":
		return blockShape{kind: RuleNode}, true
	}
	return blockShape{}, false
}

// shapeBlock makes node the shape: its kind, and only the attributes the
// shape has.
func shapeBlock(document *model.DocumentDraft, node model.NodeID, shape blockShape) error {
	if err := document.SetNodeKind(node, shape.kind); err != nil {
		return err
	}
	current, _ := document.Node(node)
	set := func(key string, keep bool, value model.AttributeValue) error {
		_, has := current.Attribute(key)
		switch {
		case keep:
			return document.SetAttribute(node, key, value)
		case has:
			return document.RemoveAttribute(node, key)
		}
		return nil
	}
	if err := set(headingLevelAttribute, shape.kind == HeadingNode, model.IntegerValue(int64(shape.level))); err != nil {
		return err
	}
	return set(codeInfoAttribute, shape.kind == CodeNode && shape.info != "", model.TextValue(shape.info))
}

// unfold turns a block whose notation no longer reads as syntax into
// paragraphs of what it shows: the notation becomes literal text around the
// block's own, which breaks into blocks at its line breaks.
func unfold(document *model.DocumentDraft, node model.NodeID, open, close string) error {
	if err := shapeBlock(document, node, blockShape{kind: ParagraphNode}); err != nil {
		return err
	}
	if err := document.ReplaceTextMarked(node, 0, 0, open, nil); err != nil {
		return err
	}
	current, _ := document.Node(node)
	end := blockLength(current)
	if err := document.ReplaceTextMarked(node, end, end, close, nil); err != nil {
		return err
	}
	current, _ = document.Node(node)
	return splitAtNewlines(document, node, 0, current.Text().String())
}

// blockKeepsLines reports whether a block's text holds line breaks of its
// own, as code does, rather than breaking into blocks at them.
func blockKeepsLines(node *model.Node) bool {
	return node != nil && node.Kind() == CodeNode
}

// literalAt reports whether text typed at a block, inside the active mark,
// is taken as written: code is, so no syntax, pairing or refinement reads
// into it.
func literalAt(node *model.Node, active *model.MarkID) bool {
	if node == nil {
		return false
	}
	if node.Kind() == CodeNode {
		return true
	}
	if active != nil {
		for _, mark := range node.Marks() {
			if mark.ID() == *active {
				return mark.Kind() == CodeMark
			}
		}
	}
	return false
}

// ordinals numbers each ordered item by its place in the run of ordered
// items it stands in.
func ordinals(blocks []*model.Node) []int {
	numbers := make([]int, len(blocks))
	run := 0
	for index, block := range blocks {
		if block.Kind() != OrderedNode {
			run = 0
			continue
		}
		run++
		numbers[index] = run
	}
	return numbers
}

// invokeBlockSyntax reads text typed into a block as block syntax: notation
// typed at a paragraph's start that opens a block, a break typed where a
// block's kind decides what follows it, and text typed onto a rule, which
// has none of its own.
func (i *EditorInvoker) invokeBlockSyntax(context *kernel.RuleContext) (kernel.Outcome, error) {
	input, ok := context.Event().TextInput()
	if !ok || !input.Target().RangeUTF16().Empty() {
		return kernel.OutcomePass, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	node, ok := document.Node(input.Target().Node())
	if !ok {
		return kernel.OutcomePass, nil
	}
	syntax := contextSyntax(context)
	at := input.Target().RangeUTF16().Start
	text := input.Text()
	shape := shapeOf(node)
	fail := func(err error) (kernel.Outcome, error) {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	switch {
	case shape.kind == ParagraphNode && (text == " " || text == "\n" || text == "-"):
		content := node.Text().String()
		split := int(model.UTF16OffsetToByteClamped(content, at))
		opened, ok := syntax.parseOpener(content[:split] + text)
		if !ok || opened.kind == ParagraphNode ||
			!opened.opensLine() && split != len(content) {
			return kernel.OutcomePass, nil
		}
		if err := shapeBlock(document, node.ID(), opened); err != nil {
			return fail(err)
		}
		if err := document.ReplaceTextMarked(node.ID(), 0, at, "", nil); err != nil {
			return fail(err)
		}
		i.placeHead(node.ID(), 0, document)
		if opened.kind == RuleNode {
			return i.paragraphAfter(document, node.ID())
		}
		return kernel.OutcomeHandled, nil
	case shape.kind == RuleNode:
		if text == "\n" {
			return i.paragraphAfter(document, node.ID())
		}
		open, _ := blockNotation(shape, 0)
		if err := unfold(document, node.ID(), open, ""); err != nil {
			return fail(err)
		}
		end := model.UTF16Offset(len(open))
		if err := document.ReplaceTextMarked(node.ID(), end, end, text, i.active); err != nil {
			return fail(err)
		}
		i.placeHead(node.ID(), end+model.UTF16Offset(model.StringLenUTF16(text)), document)
		return kernel.OutcomeHandled, nil
	case text != "\n":
		return kernel.OutcomePass, nil
	case shape.kind == HeadingNode:
		// A heading ends at a break: what follows is a paragraph, and a
		// break before its text leaves a paragraph above it.
		created, err := document.SplitText(node.ID(), at)
		if err != nil {
			return fail(err)
		}
		plain, carried := created, created
		if at == 0 && blockLength(node) > 0 {
			plain = node.ID()
		}
		if err := shapeBlock(document, plain, blockShape{kind: ParagraphNode}); err != nil {
			return fail(err)
		}
		i.placeHead(carried, 0, document)
		return kernel.OutcomeHandled, nil
	case shape.opensLine() && blockLength(node) == 0:
		// A break in an empty item or quote line ends the list or quote.
		if err := shapeBlock(document, node.ID(), blockShape{kind: ParagraphNode}); err != nil {
			return fail(err)
		}
		i.placeHead(node.ID(), 0, document)
		return kernel.OutcomeHandled, nil
	case shape.kind == CodeNode:
		content := node.Text().String()
		if at != blockLength(node) || !strings.HasSuffix(content, "\n") {
			if err := document.ReplaceTextMarked(node.ID(), at, at, "\n", nil); err != nil {
				return fail(err)
			}
			i.placeHead(node.ID(), at+1, document)
			return kernel.OutcomeHandled, nil
		}
		// A break on an empty last line leaves the code.
		if err := document.ReplaceTextMarked(node.ID(), at-1, at, "", nil); err != nil {
			return fail(err)
		}
		return i.paragraphAfter(document, node.ID())
	}
	return kernel.OutcomePass, nil
}

// paragraphAfter opens an empty paragraph after node and puts the caret in
// it.
func (i *EditorInvoker) paragraphAfter(document *model.DocumentDraft, node model.NodeID) (kernel.Outcome, error) {
	current, _ := document.Node(node)
	parent, _ := current.Parent()
	container, _ := document.Node(parent)
	index := 0
	for position, child := range container.Children() {
		if child == node {
			index = position + 1
		}
	}
	created, err := document.CreateNode(parent, index, ParagraphNode)
	if err == nil {
		err = document.SetTextValue(created, "")
	}
	if err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	i.placeHead(created, 0, document)
	return kernel.OutcomeHandled, nil
}

// placeHead puts the caret at offset in node once the edit is written,
// carried only through what is written after this.
func (i *EditorInvoker) placeHead(node model.NodeID, offset model.UTF16Offset, document *model.DocumentDraft) {
	i.correctionEffectNode = node
	i.correctionEffectHead = offset
	i.hasCorrectionEffect = true
	i.correctionEffectThrough = document.TextEditCount()
}

// invokeBlockNotation reads an edit of a block's projected notation back
// into its shape, or unfolds the block when the notation no longer reads
// as one.
func (i *EditorInvoker) invokeBlockNotation(
	context *kernel.RuleContext,
	attribute kernel.AttributeInput,
) (kernel.Outcome, error) {
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	node, ok := document.Node(attribute.Target().Node())
	if !ok {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: attribute.Target().Node()},
		}
	}
	shape := shapeOf(node)
	open, close := blockNotation(shape, 1)
	edited := attribute.Text()
	if attribute.Target().Key() == fenceNotationKey {
		close = edited
	} else {
		open = edited
	}
	reshaped, ok := contextSyntax(context).parseOpener(open)
	if _, fence := blockNotation(reshaped, 1); ok && reshaped.kind != ParagraphNode && fence == close {
		if err := shapeBlock(document, node.ID(), reshaped); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
		return kernel.OutcomeHandled, nil
	}
	if err := unfold(document, node.ID(), open, close); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return kernel.OutcomeHandled, nil
}

// invokeSpanSyntax keeps a rule from taking text in: a span that starts on
// a rule, as a deletion back into it does, takes the rule away with it.
func (i *EditorInvoker) invokeSpanSyntax(context *kernel.RuleContext) (kernel.Outcome, error) {
	span, ok := context.Event().SpanInput()
	if !ok {
		return kernel.OutcomePass, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	if first, ok := document.Node(span.From().Node()); ok && first.Kind() == RuleNode {
		if err := shapeBlock(document, first.ID(), blockShape{kind: ParagraphNode}); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
	}
	return i.invokeSpan(context)
}

// markerKeyPrefix begins the notation key of every inline mark.
const markerKeyPrefix = "mark/"

// MarkerKeyForMark returns the generated attribute identity for one mark.
func MarkerKeyForMark(id model.MarkID) string {
	return fmt.Sprintf("%s%d", markerKeyPrefix, id.Value())
}

// MarkerKeyIsInline reports whether an attribute key is an inline mark's
// notation rather than a block's.
func MarkerKeyIsInline(key string) bool {
	return strings.HasPrefix(key, markerKeyPrefix)
}

// MarkerKeyIdentifies reports whether a key names one concrete inline mark.
func MarkerKeyIdentifies(key string, id model.MarkID) bool {
	return key == MarkerKeyForMark(id)
}
