package editor

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// The Writing Profile is the CommonMark-shaped syntax the writing policy
// reads and writes. A page keeps only what its parts are: a heading and its
// level, a quote, a list item, a code block and its info string, a math
// block, a mark's kind and level. The notation that shows them ("## ", "> ", "*", a fence)
// is projected from that and never stored, not even when edited. Typing
// notation where it reads as syntax turns it into structure. A mark's
// notation is edited marker by marker, each a level the mark stores; a
// block's is one unit standing for the block's structure: Backspace at its
// text peels the structure a level (peel), and an edit that covers any of
// the notation replaces all of it, read back as a shape, or else leaves a
// paragraph of the block's own text (flatten).
const (
	// CodeMark is inline code.
	CodeMark model.MarkKind = "code"
	// StrikeMark is struck-through text.
	StrikeMark model.MarkKind = "strike"
	// MathMark is an inline formula; the text it covers is its TeX source.
	MathMark model.MarkKind = "math"

	headingLevelAttribute = "level"
	codeInfoAttribute     = "info"
	// depthAttribute nests an item or quote line within the ones before
	// it: 0 is the outermost.
	depthAttribute   = "depth"
	checkedAttribute = "checked"
	// maxDepth bounds nesting, as notation must stay legible.
	maxDepth = 8

	// blockNotationKey names the notation a block opens with, and
	// fenceNotationKey the fence a code block closes with.
	blockNotationKey = "block"
	fenceNotationKey = "fence"

	headingsSetting    = "headings"
	blockquotesSetting = "blockquotes"
	listsSetting       = "lists"
	codeSetting        = "code"
	mathSetting        = "math"
	strikeSetting      = "strikethrough"
	rulesSetting       = "rules"
)

// writingTriggers are the notation typing reads as structure, as the
// writing policy's settings allow it. The kinds themselves are built in:
// a page shows and edits every heading, list or mark whatever is allowed
// here; these decide only what typing a marker makes.
type writingTriggers struct {
	asterisks   bool
	underscores bool
	headings    bool
	quotes      bool
	lists       bool
	code        bool
	math        bool
	strike      bool
	rules       bool
	tables      bool
}

func triggersFrom(setting func(id string, fallback bool) bool) writingTriggers {
	return writingTriggers{
		asterisks:   setting(asterisksSetting, true),
		underscores: setting(underscoresSetting, true),
		headings:    setting(headingsSetting, true),
		quotes:      setting(blockquotesSetting, true),
		lists:       setting(listsSetting, true),
		code:        setting(codeSetting, true),
		math:        setting(mathSetting, true),
		strike:      setting(strikeSetting, true),
		rules:       setting(rulesSetting, true),
		tables:      setting(tablesSetting, true),
	}
}

func contextTriggers(context *kernel.RuleContext) writingTriggers {
	return triggersFrom(func(id string, fallback bool) bool {
		return contextMarkupBool(context, id, fallback)
	})
}

// writingSettingDescriptors are the triggers, in the order the Writing
// Profile card lists them.
func writingSettingDescriptors() []kernel.SettingDescriptor {
	ids := []string{
		asterisksSetting, underscoresSetting, headingsSetting, listsSetting,
		blockquotesSetting, codeSetting, mathSetting, strikeSetting, rulesSetting, tablesSetting,
	}
	descriptors := make([]kernel.SettingDescriptor, 0, len(ids))
	for _, id := range ids {
		descriptors = append(descriptors, kernel.SettingDescriptor{
			ID: kernel.SettingID(id), Default: kernel.BoolSetting(true),
		})
	}
	return descriptors
}

// markerKind is the mark a typed marker opens, when typing it is allowed to,
// and the highest level the mark goes to.
func (t writingTriggers) markerKind(text string) (model.MarkKind, uint8, bool) {
	switch text {
	case "*":
		return EmphasisMark, 2, t.asterisks
	case "_":
		return EmphasisMark, 2, t.underscores
	case "`":
		return CodeMark, 1, t.code
	case "$":
		return MathMark, 1, t.math
	case "~":
		return StrikeMark, 1, t.strike
	}
	return "", 0, false
}

// opens reports whether typing may open a block of kind.
func (t writingTriggers) opens(kind model.NodeTypeID) bool {
	desc := describe(kind)
	return desc.opens != nil && desc.opens(t)
}

// inlineNotation is how a mark shows on either side of its text.
func inlineNotation(mark model.InlineMark) string {
	switch mark.Kind() {
	case EmphasisMark:
		return strings.Repeat("*", int(mark.Level()))
	case CodeMark:
		return "`"
	case MathMark:
		return "$"
	case StrikeMark:
		return "~"
	}
	return ""
}

// inlineLevel reads edited notation of a mark back to its level; zero takes
// the mark away. Emphasis reads either marker, as typing does.
func inlineLevel(kind model.MarkKind, text string) (uint8, bool) {
	if text == "" {
		return 0, true
	}
	if kind != EmphasisMark {
		return 1, text == inlineNotation(model.NewInlineMark(0, model.UTF16Range{}, kind, 1))
	}
	if strings.Trim(text, text[:1]) != "" || text[0] != '*' && text[0] != '_' || len(text) > 2 {
		return 0, false
	}
	return uint8(len(text)), true
}

// inlineMarkerRune reports whether r is notation of some inline mark.
func inlineMarkerRune(r rune) bool {
	return r == '*' || r == '_' || r == '`' || r == '~' || r == '$'
}

// blockShape is what a block is, apart from its text.
type blockShape struct {
	kind    model.NodeTypeID
	level   int
	info    string
	depth   int
	checked bool
}

// shapeOf reads a block's shape from the page.
func shapeOf(node *model.Node) blockShape {
	shape := blockShape{kind: node.Kind()}
	if read := shape.desc().read; read != nil {
		read(node, &shape)
	}
	if shape.nests() {
		if value, ok := node.Attribute(depthAttribute); ok {
			if depth, ok := value.Integer(); ok {
				shape.depth = int(max(0, min(maxDepth, depth)))
			}
		}
	}
	return shape
}

// nests reports whether a shape stands in a group with its neighbours,
// nested by depth: items in a list, lines in a quote.
func (s blockShape) nests() bool { return s.desc().Group != "" }

// groupOf is the group a block stands in: the Writing Profile's use of the
// page's block groups (blocks.go). Bullet and numbered items are two kinds
// of list, which nest in one another; quote lines are a quote.
func groupOf(node *model.Node) (blockGroup, bool) {
	shape := shapeOf(node)
	if family := shape.desc().Group; family != "" {
		return blockGroup{family: family, kind: string(shape.kind), depth: shape.depth}, true
	}
	return blockGroup{}, false
}

// opensLine reports whether a shape's notation is a prefix its text follows
// on the same line, rather than a line of its own.
func (s blockShape) opensLine() bool { return s.desc().Prefix }

// parseOpener reads notation at a block's start as the shape it opens: the
// inverse of blockNotation.
func parseOpener(text string) (blockShape, bool) {
	paragraph := blockShape{kind: ParagraphNode}
	// Nested items are indented two spaces a level, nested quote lines
	// quoted again.
	if indent := len(text) - len(strings.TrimLeft(text, " ")); indent > 0 {
		shape, ok := parseOpener(text[indent:])
		if !ok || indent/2 > maxDepth || shape.desc().Group != "list" {
			return blockShape{}, false
		}
		shape.depth = indent / 2
		return shape, true
	}
	if quotes := strings.Count(text, "> "); quotes > 1 && text == strings.Repeat("> ", quotes) {
		if quotes-1 > maxDepth {
			return blockShape{}, false
		}
		return blockShape{kind: QuoteNode, depth: quotes - 1}, true
	}
	switch {
	case text == "":
		return paragraph, true
	case strings.HasSuffix(text, " ") &&
		len(text) <= 7 && strings.Trim(text[:len(text)-1], "#") == "" && len(text) > 1:
		return blockShape{kind: HeadingNode, level: len(text) - 1}, true
	case text == "> ":
		return blockShape{kind: QuoteNode}, true
	case (text == "- " || text == "+ "):
		return blockShape{kind: BulletNode}, true
	case text == "- [ ] " || text == "+ [ ] ":
		return blockShape{kind: CheckboxNode}, true
	case text == "- [x] " || text == "- [X] " || text == "+ [x] " || text == "+ [X] ":
		return blockShape{kind: CheckboxNode, checked: true}, true
	case strings.HasSuffix(text, ". ") && len(text) > 2 && len(text) <= 11 &&
		strings.Trim(text[:len(text)-2], "0123456789") == "":
		return blockShape{kind: OrderedNode}, true
	case strings.HasPrefix(text, "```") && strings.HasSuffix(text, "\n") &&
		!strings.ContainsAny(text[3:len(text)-1], "`\n"):
		return blockShape{kind: CodeNode, info: strings.TrimSpace(text[3 : len(text)-1])}, true
	case text == "$$\n":
		return blockShape{kind: MathNode}, true
	case text == "---":
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
	if err := set(headingLevelAttribute, shape.desc().Leveled, model.IntegerValue(int64(shape.level))); err != nil {
		return err
	}
	if err := set(depthAttribute, shape.nests() && shape.depth > 0, model.IntegerValue(int64(shape.depth))); err != nil {
		return err
	}
	if err := set(codeInfoAttribute, shape.desc().Info && shape.info != "", model.TextValue(shape.info)); err != nil {
		return err
	}
	if err := set(checkedAttribute, shape.desc().Checks && shape.checked, model.BoolValue(true)); err != nil {
		return err
	}
	// A block that is no longer an image no longer names its picture; an
	// image's own are written as it is placed (writeImageShape).
	if !shape.desc().Atomic {
		for _, key := range imageAttributes {
			if err := set(key, false, model.AttributeValue{}); err != nil {
				return err
			}
		}
	}
	if !shape.desc().Cell {
		for _, key := range cellAttributes {
			if err := set(key, false, model.AttributeValue{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// flatten makes a block a paragraph of its own text, as when its notation
// is taken away: the notation goes with the structure it stood for, and is
// never left behind as text. Lines it kept, as code does, become blocks.
func flatten(document *model.DocumentDraft, node model.NodeID) error {
	if err := shapeBlock(document, node, blockShape{kind: ParagraphNode}); err != nil {
		return err
	}
	current, _ := document.Node(node)
	return splitAtNewlines(document, node, 0, current.Text().String())
}

// peel is the shape a block steps back to as its notation is taken from
// the outside in: a nested item or quote line steps out a level, anything
// else becomes a paragraph.
func (s blockShape) peel() blockShape {
	if s.nests() && s.depth > 0 {
		s.depth--
		return s
	}
	return blockShape{kind: ParagraphNode}
}

// blockKeepsLines reports whether a block's text holds line breaks of its
// own, as code does, rather than breaking into blocks at them.
func blockKeepsLines(node *model.Node) bool {
	return kindOf(node).KeepsLines
}

// literalBlock reports whether a block's text is all taken as written:
// code's and a formula's source.
func literalBlock(node *model.Node) bool {
	return kindOf(node).Literal
}

// literalAt reports whether text typed at a block, inside the active mark,
// is taken as written: code and a formula's source are, so no syntax,
// pairing or refinement reads into them.
func literalAt(node *model.Node, active *model.MarkID) bool {
	if literalBlock(node) {
		return true
	}
	_, ok := literalMark(node, active)
	return ok
}

// literalMark is the kind of the active mark when what it covers is taken
// as written, as code and an inline formula are.
func literalMark(node *model.Node, active *model.MarkID) (model.MarkKind, bool) {
	if node == nil || active == nil {
		return "", false
	}
	for _, mark := range node.Marks() {
		if mark.ID() == *active {
			kind := mark.Kind()
			return kind, kind == CodeMark || kind == MathMark
		}
	}
	return "", false
}

// ordinals numbers each ordered item by its place among the ordered items
// at its depth since its list, or the item it nests under, began. A bullet
// at that depth starts the count again.
func ordinals(blocks []*model.Node) []int {
	numbers := make([]int, len(blocks))
	counts := make([]int, maxDepth+1)
	for index, block := range blocks {
		group, ok := groupOf(block)
		if !ok || group.family != "list" {
			clear(counts)
			continue
		}
		clear(counts[group.depth+1:])
		if !kindOf(block).Numbered {
			counts[group.depth] = 0
			continue
		}
		counts[group.depth]++
		numbers[index] = counts[group.depth]
	}
	return numbers
}

// collapsedTyping is the block a collapsed text input types into, with the
// offset and the text.
func collapsedTyping(context *kernel.RuleContext) (*model.DocumentDraft, *model.Node, model.UTF16Offset, string, bool, error) {
	input, ok := context.Event().TextInput()
	if !ok || !input.Target().RangeUTF16().Empty() {
		return nil, nil, 0, "", false, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return nil, nil, 0, "", false, err
	}
	node, ok := document.Node(input.Target().Node())
	return document, node, input.Target().RangeUTF16().Start, input.Text(), ok, nil
}

// invokeBlockTrigger reads notation typed at a paragraph's start as the
// block it opens, as far as the writing policy allows: "## " a heading,
// "```" and Return a code block, "---" a rule.
func (i *EditorInvoker) invokeBlockTrigger(context *kernel.RuleContext) (kernel.Outcome, error) {
	document, node, at, text, ok, err := collapsedTyping(context)
	if err != nil || !ok {
		return kernel.OutcomePass, err
	}
	if kindOf(node).Checkable && contextTriggers(context).lists {
		if outcome, handled, err := i.openCheckbox(document, node, at, text); handled || err != nil {
			return outcome, err
		}
	}
	if !kindOf(node).Plain ||
		text != " " && text != "\n" && text != "-" {
		return kernel.OutcomePass, err
	}
	content := node.Text().String()
	split := int(model.UTF16OffsetToByteClamped(content, at))
	if text == "\n" && split == len(content) && contextTriggers(context).tables {
		if outcome, opened, err := i.openTable(document, node); opened || err != nil {
			return outcome, err
		}
	}
	opened, ok := parseOpener(content[:split] + text)
	if !ok || !contextTriggers(context).opens(opened.kind) ||
		!opened.opensLine() && split != len(content) {
		return kernel.OutcomePass, nil
	}
	if err := shapeBlock(document, node.ID(), opened); err == nil {
		err = document.ReplaceTextMarked(node.ID(), 0, at, "", nil)
	}
	if err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	i.land(node.ID(), 0, document)
	if opened.desc().Divider {
		return i.paragraphAfter(document, node.ID())
	}
	return kernel.OutcomeHandled, nil
}

// invokeBlockKind types into a block as its kind has it, whatever typing
// may open: a break where the kind decides what follows it, and text onto
// a rule, which has none of its own.
func (i *EditorInvoker) invokeBlockKind(context *kernel.RuleContext) (kernel.Outcome, error) {
	document, node, at, text, ok, err := collapsedTyping(context)
	if err != nil || !ok {
		return kernel.OutcomePass, err
	}
	shape := shapeOf(node)
	fail := func(err error) (kernel.Outcome, error) {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	switch {
	case shape.desc().Divider:
		if text == "\n" {
			return i.paragraphAfter(document, node.ID())
		}
		// Text typed on a rule makes the line a paragraph of that text.
		if err := flatten(document, node.ID()); err != nil {
			return fail(err)
		}
		if err := document.ReplaceTextMarked(node.ID(), 0, 0, text, i.active); err != nil {
			return fail(err)
		}
		i.land(node.ID(), model.UTF16Offset(model.StringLenUTF16(text)), document)
		return kernel.OutcomeHandled, nil
	case text != "\n":
		return kernel.OutcomePass, nil
	case shape.desc().Cell:
		return i.cellBreak(document, node.ID())
	case shape.desc().BreakLeaves:
		// A heading or a caption ends at a break: what follows is a
		// paragraph, and a break before its text leaves a paragraph above
		// it.
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
		i.land(carried, 0, document)
		return kernel.OutcomeHandled, nil
	case literalBlock(node):
		content := node.Text().String()
		if at != blockLength(node) || !strings.HasSuffix(content, "\n") {
			if err := document.ReplaceTextMarked(node.ID(), at, at, "\n", nil); err != nil {
				return fail(err)
			}
			i.land(node.ID(), at+1, document)
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
	i.land(created, 0, document)
	return kernel.OutcomeHandled, nil
}

// land proposes the caret at offset in node once the edit is written,
// carried only through what is written after this.
func (i *EditorInvoker) land(node model.NodeID, offset model.UTF16Offset, document *model.DocumentDraft) {
	i.proposal.land(Landing{Node: node, Head: offset, Through: document.TextEditCount()})
}

// invokeBlockNotation reads an edit of a block's projected notation back
// into its shape, or makes the block a paragraph when it no longer reads
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
	reshaped, ok := parseOpener(open)
	if _, fence := blockNotation(reshaped, 1); ok && !reshaped.desc().Plain && fence == close {
		if err := shapeBlock(document, node.ID(), reshaped); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
		return kernel.OutcomeHandled, nil
	}
	if err := flatten(document, node.ID()); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return kernel.OutcomeHandled, nil
}

// invokeSpanSyntax replaces a span as its ends give it.
func (i *EditorInvoker) invokeSpanSyntax(context *kernel.RuleContext) (kernel.Outcome, error) {
	span, ok := context.Event().SpanInput()
	if !ok {
		return kernel.OutcomePass, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	return i.joinSpan(document, span.From().Node(), span.From().RangeUTF16().Start,
		span.To().Node(), span.To().RangeUTF16().Start, span.Text())
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

// PageTitle is the text of a page's first heading, whatever its level, on
// one line; a page without a heading has none.
func PageTitle(document model.DocumentSnapshot) string {
	for _, id := range blockOrder(document) {
		if block, ok := document.Node(id); ok && kindOf(block).Leveled {
			return strings.Join(strings.Fields(block.Text().String()), " ")
		}
	}
	return ""
}
