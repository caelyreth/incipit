package editor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/planning"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

const (
	// TextPolicyID owns ordinary semantic text input.
	TextPolicyID kernel.PolicyID = "editor.text"
	// HistoryPolicyID owns undo and redo commands.
	HistoryPolicyID kernel.PolicyID = "editor.history"

	asterisksSetting    = "asterisks"
	underscoresSetting  = "underscores"
	markupFormatTrigger = "markup:format"
	nestTrigger         = "writing:nest"
)

// EditorPolicy is the stable identity of one concrete editor policy.
type EditorPolicy string

const (
	EditorPolicyText       EditorPolicy = "editor.text"
	EditorPolicyWriting    EditorPolicy = "editor.writing"
	EditorPolicyPairs      EditorPolicy = "editor.pairs"
	EditorPolicyCJK        EditorPolicy = "editor.cjk"
	EditorPolicyTypography EditorPolicy = "editor.typography"
	EditorPolicyHistory    EditorPolicy = "editor.history"
	EditorPolicyNavigation EditorPolicy = "editor.navigation"
	EditorPolicyWords      EditorPolicy = "editor.words"
)

// AllEditorPolicies is the policies' assembly order.
var AllEditorPolicies = [...]EditorPolicy{
	EditorPolicyText,
	EditorPolicyWriting,
	EditorPolicyPairs,
	EditorPolicyCJK,
	EditorPolicyTypography,
	EditorPolicyHistory,
	EditorPolicyNavigation,
	EditorPolicyWords,
}

// ID returns the kernel identity for this policy.
func (p EditorPolicy) ID() kernel.PolicyID {
	return kernel.PolicyID(p)
}

// String returns the stable configuration identity.
func (p EditorPolicy) String() string {
	return string(p)
}

// Setting returns a setting key owned by this policy.
func (p EditorPolicy) Setting(id string) kernel.SettingKey {
	return kernel.NewSettingKey(p.ID(), kernel.SettingID(id))
}

// EditorPolicyFromID resolves a stable policy identity.
func EditorPolicyFromID(id kernel.PolicyID) (EditorPolicy, bool) {
	for _, policy := range AllEditorPolicies {
		if policy.ID() == id {
			return policy, true
		}
	}
	return "", false
}

// textOp is one character, line or page step, or deletion, of the text
// responders (session_text_commands.go).
type textOp uint8

const (
	textLeft textOp = iota + 1
	textRight
	textSelectLeft
	textSelectRight
	textLineStart
	textLineEnd
	textSelectLineStart
	textSelectLineEnd
	textDocumentStart
	textDocumentEnd
	textSelectDocumentStart
	textSelectDocumentEnd
	textSelectAll
	textBackspace
	textDelete
	textNewline
	textDismissSelection
)

// historyOp is one step of the page's history.
type historyOp uint8

const (
	historyUndo historyOp = iota + 1
	historyRedo
)

// EditorPolicyDescriptors returns the complete policy assembly in stable
// declaration order.
func EditorPolicyDescriptors() ([]kernel.PolicyDescriptor, error) {
	text, err := textPolicyDescriptor()
	if err != nil {
		return nil, err
	}
	markup, err := writingPolicyDescriptor()
	if err != nil {
		return nil, err
	}
	pairs, err := pairsPolicyDescriptor()
	if err != nil {
		return nil, err
	}
	cjk, err := observerPolicyDescriptor(
		EditorPolicyCJK,
		nil,
		false,
		[]string{"punctuation", "spacing"},
		[]kernel.EventOrigin{
			kernel.EventOriginTyping,
			kernel.EventOriginPaste,
			kernel.EventOriginIME,
		},
	)
	if err != nil {
		return nil, err
	}
	// Typography refines what CJK refinement leaves.
	typography, err := observerPolicyDescriptor(
		EditorPolicyTypography,
		[]kernel.RuleKey{{Owner: CJKPolicyID, ID: "refine"}},
		true,
		[]string{"symbols", "punctuation", "quote-placement"},
		[]kernel.EventOrigin{kernel.EventOriginTyping},
	)
	if err != nil {
		return nil, err
	}
	return []kernel.PolicyDescriptor{
		text,
		markup,
		pairs,
		cjk,
		typography,
		historyPolicyDescriptor(),
		navigationPolicyDescriptor(),
		wordsPolicyDescriptor(),
		surfacePolicyDescriptor(),
	}, nil
}

// EditorPolicyCatalog links the complete editor declaration set with the
// default keymap.
func EditorPolicyCatalog() (kernel.PolicyCatalog, error) {
	descriptors, err := EditorPolicyDescriptors()
	if err != nil {
		return kernel.PolicyCatalog{}, err
	}
	return kernel.LinkPolicyCatalog(descriptors, defaultKeymap[:]...)
}

// The rules commands run as.
var (
	shapeRule   = textRule("shape")
	formatRule  = writingRule("format")
	nestRule    = writingRule("nest")
	figureRule  = writingRule("figure")
	tableRule   = writingRule("table")
	arrangeRule = writingRule("arrange")
)

// commandInvoker runs a command's edit as its rule, within the dispatch the
// command makes; every other rule answers as the session's do.
type commandInvoker struct {
	session *Session
	rule    kernel.RuleKey
	run     func(*kernel.RuleContext) (kernel.Outcome, error)
}

func (s *Session) command(rule kernel.RuleKey, run func(*kernel.RuleContext) (kernel.Outcome, error)) *commandInvoker {
	return &commandInvoker{session: s, rule: rule, run: run}
}

func (i *commandInvoker) Invoke(rule *kernel.LinkedRule, leaf kernel.LeafID, context *kernel.RuleContext) (kernel.Outcome, error) {
	if rule == nil || rule.Key != i.rule {
		return i.session.invoker.Invoke(rule, leaf, context)
	}
	return i.run(context)
}

func textRule(id kernel.RuleID) kernel.RuleKey {
	return kernel.RuleKey{Owner: TextPolicyID, ID: id}
}

func writingRule(id kernel.RuleID) kernel.RuleKey {
	return kernel.RuleKey{Owner: WritingPolicyID, ID: id}
}

func textPolicyDescriptor() (kernel.PolicyDescriptor, error) {
	algorithm, err := singleLeafProgram()
	if err != nil {
		return kernel.PolicyDescriptor{}, err
	}
	commands := policyCommands(TextPolicyID)
	edit := kernel.RuleDescriptor{
		ID:        "edit",
		Phase:     kernel.HandleRulePhase(),
		Trigger:   kernel.NewExactTrigger(kernel.TextInputKey),
		Effects:   kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite),
		Algorithm: algorithm,
	}
	projected := edit
	projected.ID = "projected-edit"
	projected.Trigger = kernel.NewExactTrigger(kernel.ProjectedTextKey)
	span := edit
	span.ID = "span"
	span.Trigger = kernel.NewExactTrigger(kernel.SpanInputKey)
	// Every block and mark kind is built in: editing their projected
	// notation, and typing as each kind has it, are core editing.
	notation := edit
	notation.ID = "notation"
	notation.Trigger = kernel.NewExactTrigger(kernel.AttributeInputKey)
	block := edit
	block.ID = "block"
	block.Origins = []kernel.EventOrigin{kernel.EventOriginTyping}
	// Typing as a block's kind has it comes before writing it literally.
	block.Before = []kernel.RuleKey{textRule("edit")}
	shape := edit
	shape.ID = shapeRule.ID
	shape.Trigger = kernel.NewExactTrigger(shapeTrigger)
	return kernel.PolicyDescriptor{
		ID:               TextPolicyID,
		EnabledByDefault: true,
		Commands:         commands,
		Rules:            []kernel.RuleDescriptor{edit, projected, span, notation, block, shape},
	}, nil
}

// writingPolicyDescriptor declares what the writing policy adds over the
// built-in kinds: typing markers as structure, and the formatting commands.
func writingPolicyDescriptor() (kernel.PolicyDescriptor, error) {
	algorithm, err := singleLeafProgram()
	if err != nil {
		return kernel.PolicyDescriptor{}, err
	}
	commands := policyCommands(WritingPolicyID)
	marker := kernel.RuleDescriptor{
		ID:        "type-marker",
		Phase:     kernel.HandleRulePhase(),
		Trigger:   kernel.NewExactTrigger(kernel.TextInputKey),
		Origins:   []kernel.EventOrigin{kernel.EventOriginTyping},
		After:     []kernel.RuleKey{writingRule("block-syntax")},
		Before:    []kernel.RuleKey{textRule("edit")},
		Effects:   kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite),
		Algorithm: algorithm,
	}
	// Block triggers read typing before marks do.
	block := marker
	block.ID = "block-syntax"
	block.After = []kernel.RuleKey{textRule("block")}
	format := marker
	format.ID = formatRule.ID
	format.Trigger = kernel.NewExactTrigger(markupFormatTrigger)
	format.Origins = nil
	format.After, format.Before = nil, nil
	nest := format
	nest.ID = nestRule.ID
	nest.Trigger = kernel.NewExactTrigger(nestTrigger)
	figure := format
	figure.ID = figureRule.ID
	figure.Trigger = kernel.NewExactTrigger(figureTrigger)
	table := format
	table.ID = tableRule.ID
	table.Trigger = kernel.NewExactTrigger(tableTrigger)
	arrange := format
	arrange.ID = arrangeRule.ID
	arrange.Trigger = kernel.NewExactTrigger(arrangeTrigger)
	return kernel.PolicyDescriptor{
		ID:               WritingPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Settings:         writingSettingDescriptors(),
		Commands:         commands,
		Rules:            []kernel.RuleDescriptor{marker, format, block, nest, figure, table, arrange},
	}, nil
}

func pairsPolicyDescriptor() (kernel.PolicyDescriptor, error) {
	algorithm, err := singleLeafProgram()
	if err != nil {
		return kernel.PolicyDescriptor{}, err
	}
	effects := kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite)
	typing := []kernel.EventOrigin{kernel.EventOriginTyping}
	rule := func(id kernel.RuleID, origins []kernel.EventOrigin, after ...kernel.RuleID) kernel.RuleDescriptor {
		descriptor := kernel.RuleDescriptor{
			ID:        id,
			Phase:     kernel.HandleRulePhase(),
			Trigger:   kernel.NewExactTrigger(kernel.TextInputKey),
			Origins:   origins,
			After:     []kernel.RuleKey{textRule("block"), writingRule("type-marker")},
			Before:    []kernel.RuleKey{textRule("edit")},
			Effects:   effects,
			Algorithm: algorithm,
		}
		for _, first := range after {
			descriptor.After = append(descriptor.After, kernel.RuleKey{Owner: PairsPolicyID, ID: first})
		}
		return descriptor
	}
	// Typed markers are read first; a closer typed over its own steps
	// over it, an edit of one mate takes the other, a quote closes by its
	// context, and only then does an opener bring its closer.
	overtype := rule("overtype", typing)
	mate := rule("mate-edit", []kernel.EventOrigin{kernel.EventOriginTyping, kernel.EventOriginSemantic}, "overtype")
	// Mates are edited together across blocks too: a span that cuts a
	// pair takes the mate it leaves, before core editing joins it.
	span := mate
	span.ID = "mate-span"
	span.Trigger = kernel.NewExactTrigger(kernel.SpanInputKey)
	span.After = nil
	span.Before = []kernel.RuleKey{textRule("span")}
	rules := []kernel.RuleDescriptor{
		rule("open", typing, "close-quote"),
		overtype,
		mate,
		span,
		rule("close-quote", typing, "mate-edit"),
	}
	return kernel.PolicyDescriptor{
		ID:               PairsPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Settings: []kernel.SettingDescriptor{
			{ID: "quotes", Default: kernel.BoolSetting(true)},
			{ID: "overtype", Default: kernel.BoolSetting(true)},
			{ID: "edit-mates", Default: kernel.BoolSetting(true)},
			{ID: "contextual-quotes", Default: kernel.BoolSetting(true)},
		},
		Rules: rules,
	}, nil
}

func observerPolicyDescriptor(
	policy EditorPolicy,
	after []kernel.RuleKey,
	enabled bool,
	settings []string,
	origins []kernel.EventOrigin,
) (kernel.PolicyDescriptor, error) {
	algorithm, err := singleLeafProgram()
	if err != nil {
		return kernel.PolicyDescriptor{}, err
	}
	declared := make([]kernel.SettingDescriptor, 0, len(settings))
	for _, id := range settings {
		declared = append(declared, kernel.SettingDescriptor{
			ID:      kernel.SettingID(id),
			Default: kernel.BoolSetting(true),
		})
	}
	return kernel.PolicyDescriptor{
		ID:               policy.ID(),
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: enabled,
		Settings:         declared,
		Rules: []kernel.RuleDescriptor{{
			ID:        "refine",
			Phase:     kernel.AfterRulePhase(kernel.DocumentStateKey()),
			Trigger:   kernel.NewExactTrigger(kernel.TextInputKey),
			Origins:   append([]kernel.EventOrigin(nil), origins...),
			After:     after,
			Effects:   kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite),
			Algorithm: algorithm,
		}},
	}, nil
}

func historyPolicyDescriptor() kernel.PolicyDescriptor {
	commands := policyCommands(HistoryPolicyID)
	return kernel.PolicyDescriptor{
		ID:               HistoryPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Commands:         commands,
	}
}

func navigationPolicyDescriptor() kernel.PolicyDescriptor {
	return kernel.PolicyDescriptor{
		ID:               NavigationPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Settings: []kernel.SettingDescriptor{
			{ID: "pairs", Default: kernel.BoolSetting(true)},
			{ID: "emphasis", Default: kernel.BoolSetting(true)},
			{ID: "return-to-edit", Default: kernel.BoolSetting(true)},
		},
		Commands: policyCommands(NavigationPolicyID),
	}
}

func wordsPolicyDescriptor() kernel.PolicyDescriptor {
	commands := policyCommands(WordsPolicyID)
	return kernel.PolicyDescriptor{
		ID:               WordsPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Commands:         commands,
	}
}

func singleLeafProgram() (*planning.ControlProgram, error) {
	builder := &planning.ControlBuilder{}
	root := builder.Leaf(planning.NewLeafID(0))
	return builder.Build(root)
}

// WritingPolicy owns semantic inline emphasis projection.
type WritingPolicy struct{}

// Project builds a source-validated projection from one engine snapshot.
func (WritingPolicy) Project(
	snapshot kernel.EngineSnapshot,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	return ProjectSnapshot(snapshot, generation, configuration)
}

// ProjectDocument builds a projection from a bare document snapshot.
func (WritingPolicy) ProjectDocument(
	document model.DocumentSnapshot,
	revision uint64,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	return ProjectPage(document, revision, generation, configuration)
}

// MarkupProjectionError identifies projection construction failures.
type MarkupProjectionError struct {
	Kind projection.ErrorKind
	Err  error
}

func (e *MarkupProjectionError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("markup projection error %d", e.Kind)
}

func (e *MarkupProjectionError) Unwrap() error { return e.Err }

// MarkupProject builds the projected surface of the whole document.
func ProjectSnapshot(
	snapshot kernel.EngineSnapshot,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	return ProjectPage(snapshot.Document(), snapshot.Revision(), generation, configuration)
}

// ProjectPage builds and source-validates the projection of a bare
// semantic document snapshot: its blocks in order, each with its notation,
// one line break of generated text between one block and the next.
func ProjectPage(
	document model.DocumentSnapshot,
	revision uint64,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	if err := ValidatePage(document); err != nil {
		return nil, &MarkupProjectionError{Kind: projection.ErrorSource, Err: err}
	}
	blocks := DocumentBlocks(document)
	if len(blocks) == 0 {
		return nil, &MarkupProjectionError{
			Kind: projection.ErrorSource,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: document.Root()},
		}
	}
	builder := projection.NewBuilder(
		projection.ProjectionID(fmt.Sprintf("document/%d", document.Root())),
		projection.NewVersion(revision, generation),
	)
	builder.Grow(4 * len(blocks))
	numbers := ordinals(blocks)
	for index, node := range blocks {
		if index > 0 {
			if err := builder.Generated("\n"); err != nil {
				return nil, wrapProjectionError(err)
			}
		}
		if err := projectBlock(builder, node, numbers[index]); err != nil {
			return nil, err
		}
	}
	result, err := builder.Build([]projection.Dependency{{Document: true}})
	if err != nil {
		return nil, wrapProjectionError(err)
	}
	if err := result.ValidateSource(revision, document); err != nil {
		return nil, wrapProjectionError(err)
	}
	return result, nil
}

// projectBlock projects one block: the notation it opens with, its text
// with the notation of its marks, and the fence it closes with.
func projectBlock(
	builder *projection.Builder,
	node *model.Node,
	ordinal int,
) error {
	open, close := blockNotationParts(shapeOf(node), ordinal)
	edge := func(key string, parts notation) error {
		if len(parts) == 0 {
			return wrapProjectionError(builder.Generated(""))
		}
		return wrapProjectionError(builder.Attribute(node.ID(), key, parts...))
	}
	if err := edge(blockNotationKey, open); err != nil {
		return err
	}
	if err := projectInlineBody(builder, node); err != nil {
		return wrapProjectionError(err)
	}
	return edge(fenceNotationKey, close)
}

type inlineProjectionEvent struct {
	position model.UTF16Offset
	kind     uint8
	end      model.UTF16Offset
	start    model.UTF16Offset
	mark     model.InlineMark
}

func projectInlineBody(
	builder *projection.Builder,
	node *model.Node,
) error {
	length := model.UTF16Offset(0)
	if node.TextPresent() {
		length = model.UTF16Offset(node.Text().UTF16Len())
	}
	if node.MarkCount() == 0 {
		return builder.Editable(node, model.NewUTF16Range(0, length))
	}
	events := make([]inlineProjectionEvent, 0, node.MarkCount()*2)
	for _, mark := range node.Marks() {
		range16 := mark.Range()
		if range16.Empty() {
			events = append(events, inlineProjectionEvent{
				position: range16.Start,
				kind:     2,
				mark:     mark,
			})
			continue
		}
		events = append(events, inlineProjectionEvent{
			position: range16.Start,
			kind:     1,
			end:      range16.End,
			mark:     mark,
		})
		events = append(events, inlineProjectionEvent{
			position: range16.End,
			kind:     0,
			start:    range16.Start,
			mark:     mark,
		})
	}
	sort.Slice(events, func(i, j int) bool {
		left, right := events[i], events[j]
		if left.position != right.position {
			return left.position < right.position
		}
		if left.kind != right.kind {
			return left.kind < right.kind
		}
		switch left.kind {
		case 0:
			if left.start != right.start {
				return left.start > right.start
			}
		case 1:
			if left.end != right.end {
				return left.end > right.end
			}
		}
		return left.mark.ID() < right.mark.ID()
	})
	cursor := model.UTF16Offset(0)
	for _, event := range events {
		if cursor < event.position {
			if err := builder.Editable(node, model.NewUTF16Range(cursor, event.position)); err != nil {
				return err
			}
		}
		cursor = event.position
		// An end sorts before a start at one place, kind 0; an empty mark,
		// kind 2, opens and closes there.
		if err := appendInlineMarker(builder, node, event.mark, event.kind != 0); err != nil {
			return err
		}
		if event.kind == 2 {
			if err := builder.Editable(node, model.NewUTF16Range(cursor, cursor)); err != nil {
				return err
			}
			if err := appendInlineMarker(builder, node, event.mark, false); err != nil {
				return err
			}
		}
	}
	if cursor < length {
		return builder.Editable(node, model.NewUTF16Range(cursor, length))
	}
	return nil
}

// appendInlineMarker appends the notation opening a mark, or closing it.
func appendInlineMarker(
	builder *projection.Builder,
	node *model.Node,
	mark model.InlineMark,
	opening bool,
) error {
	return builder.Attribute(node.ID(), MarkerKeyForMark(mark.ID()), projection.Part{Text: inlineNotation(mark), Role: markRole(mark, opening)})
}

func markupSettingBool(
	configuration *kernel.PreparedPolicyConfiguration,
	id string,
	fallback bool,
) bool {
	if configuration == nil {
		return fallback
	}
	value, ok := configuration.Setting(NewWritingSettingKey(id))
	if !ok {
		return fallback
	}
	boolean, ok := value.Bool()
	if !ok {
		return fallback
	}
	return boolean
}

// NewWritingSettingKey returns a setting key owned by editor.markup.
func NewWritingSettingKey(id string) kernel.SettingKey {
	return EditorPolicyWriting.Setting(id)
}

func wrapProjectionError(err error) error {
	if err == nil {
		return nil
	}
	if current, ok := err.(*MarkupProjectionError); ok {
		return current
	}
	return &MarkupProjectionError{Kind: projection.ErrorSource, Err: err}
}

// EditorInvoker executes text replace and markup rules against one engine
// draft. Active and last-marker fields are session-owned facts.
type EditorInvoker struct {
	active *model.MarkID
	// proposal is what the rules of the current dispatch propose besides
	// their edits.
	proposal Proposal
	// corrections are the document edits refinement observers made in
	// the current dispatch, published as feedback once it succeeds.
	corrections CorrectionLog
}

// ActiveMark returns the current active inline mark.
func (i *EditorInvoker) ActiveMark() (model.MarkID, bool) {
	id := derefMark(i.active)
	return id, i.active != nil
}

// ActivateMark replaces the active inline mark.
func (i *EditorInvoker) ActivateMark(id *model.MarkID) {
	i.active = cloneMarkPointer(id)
}

// Invoke implements kernel.EngineInvoker for the implemented editor rules.
func (i *EditorInvoker) Invoke(
	rule *kernel.LinkedRule,
	_ kernel.LeafID,
	context *kernel.RuleContext,
) (kernel.Outcome, error) {
	if rule == nil {
		return kernel.OutcomePass, nil
	}
	if rule.Phase.Kind == kernel.RulePhaseKindHandle {
		i.proposal.reset()
	}
	switch rule.Key.Owner {
	case TextPolicyID:
		switch rule.Key.ID {
		case "span":
			return i.invokeSpanSyntax(context)
		case "notation":
			return i.invokeAttribute(context)
		case "block":
			return i.invokeBlockKind(context)
		}
		return i.invokeText(context)
	case WritingPolicyID:
		switch rule.Key.ID {
		case "block-syntax":
			return i.invokeBlockTrigger(context)
		case "type-marker":
			return i.invokeMarker(context)
		}
	case PairsPolicyID:
		if rule.Key.ID == "mate-span" {
			return i.invokePairSpan(context)
		}
		return i.invokePair(context, rule.Key.ID)
	case CJKPolicyID, TypographyPolicyID:
		return i.invokeCorrection(context, rule.Key.Owner)
	}
	return kernel.OutcomePass, nil
}

func (i *EditorInvoker) invokeText(context *kernel.RuleContext) (kernel.Outcome, error) {
	if projected, ok := context.Event().ProjectedEdit(); ok && !projected.ApplyText() {
		return kernel.OutcomePass, nil
	}
	input, _ := context.Event().TextInput()
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	start := input.Target().RangeUTF16().Start
	beforeEdits := document.TextEditCount()
	// Projected edits batch attribute intents before this text intent, with
	// every target validated against the pre-write draft. Consume through
	// the compound path: ApplyTextInput would
	// revalidate against the draft the attribute intents already wrote.
	_, _, handled, err := context.ReplaceTextInputMarked(i.active)
	if err != nil {
		return 0, err
	}
	if !handled {
		return kernel.OutcomePass, nil
	}
	i.proposal.land(Landing{
		Head:    start + model.UTF16Offset(model.StringLenUTF16(input.Text())),
		Through: min(beforeEdits+1, document.TextEditCount()),
	})
	if node, _ := document.Node(input.Target().Node()); blockKeepsLines(node) {
		return kernel.OutcomeHandled, nil
	}
	if err := splitAtNewlines(document, input.Target().Node(), start, input.Text()); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return kernel.OutcomeHandled, nil
}

func (i *EditorInvoker) invokeMarker(context *kernel.RuleContext) (kernel.Outcome, error) {
	input, ok := context.Event().TextInput()
	if !ok {
		return kernel.OutcomePass, nil
	}
	origin, ok := context.Event().Origin()
	if !ok {
		origin = kernel.EventOriginTyping
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	node, ok := document.Node(input.Target().Node())
	if !ok {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: input.Target().Node()},
		}
	}
	action, err := PlanMarkerTypes(
		node,
		input.Target().RangeUTF16(),
		input.Text(),
		i.active,
		contextTriggers(context),
		origin,
	)
	if err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if !action.Handled {
		return kernel.OutcomePass, nil
	}
	if !action.NoOp {
		if err := action.Apply(document); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
	}
	// The marker's head reads the block as it was before it wrote; one
	// that exits a mark lands past its closer, the mark no longer active,
	// and the rest of the closer the writer types is the one overtyped.
	i.proposal.land(Landing{Head: action.Head})
	if action.Kind == MarkerExit && action.Level > 1 {
		i.proposal.absorbs(strings.Repeat(input.Text(), int(action.Level)-1))
	}
	switch {
	case action.Exited, action.Kind == MarkerEnclose, action.Kind == MarkerFormulaBlock:
		i.active = nil
	case action.Kind == MarkerUpgrade && action.Level == 0:
		i.active = nil
	default:
		updated, ok := document.Node(action.Node)
		if !ok {
			return 0, &kernel.EngineError{
				Kind: kernel.EngineErrorDocument,
				Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: action.Node},
			}
		}
		if id, found := MarkerEffect(updated, action.Range); found {
			i.active = &id
		} else if action.Kind == MarkerUpgrade && action.HasMark {
			id := action.Mark
			i.active = &id
		} else {
			i.active = nil
		}
	}
	return kernel.OutcomeHandled, nil
}

func (i *EditorInvoker) invokeAttribute(context *kernel.RuleContext) (kernel.Outcome, error) {
	input, ok := context.Event().AttributeInput()
	if !ok {
		return kernel.OutcomePass, nil
	}
	key := input.Target().Key()
	if key == blockNotationKey || key == fenceNotationKey {
		return i.invokeBlockNotation(context, input)
	}
	if !MarkerKeyIsInline(key) {
		return kernel.OutcomePass, nil
	}
	invalid := &kernel.EngineError{
		Kind: kernel.EngineErrorInput,
		Err: &kernel.InputError{
			Kind: kernel.InputErrorInvalidAttributeInput,
			Node: input.Target().Node(),
			Key:  key,
		},
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	node, ok := document.Node(input.Target().Node())
	if !ok {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: input.Target().Node()},
		}
	}
	var mark *model.InlineMark
	for _, candidate := range node.Marks() {
		if MarkerKeyIdentifies(key, candidate.ID()) {
			value := candidate
			mark = &value
			break
		}
	}
	if mark == nil {
		return 0, invalid
	}
	level, ok := inlineLevel(mark.Kind(), input.Text())
	if !ok {
		return 0, invalid
	}
	if err := document.SetMarkLevel(node.ID(), mark.ID(), level); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if level == 0 {
		i.active = nil
	} else {
		id := mark.ID()
		i.active = &id
	}
	return kernel.OutcomeHandled, nil
}

func (i *EditorInvoker) invokePair(
	context *kernel.RuleContext,
	ruleID kernel.RuleID,
) (kernel.Outcome, error) {
	input, ok := context.Event().TextInput()
	if !ok {
		return kernel.OutcomePass, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	node, ok := document.Node(input.Target().Node())
	if !ok {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorDocument,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: input.Target().Node()},
		}
	}
	// A formula's source pairs its brackets, a code span's or block's text
	// nothing: both are taken as written.
	scope, inFormula := formulaScope(node, i.active)
	if literalAt(node, i.active) && !inFormula {
		return kernel.OutcomePass, nil
	}
	origin, _ := context.Event().Origin()
	rule := map[kernel.RuleID]PairRule{
		"open":        PairOpen,
		"overtype":    PairOvertype,
		"mate-edit":   PairMateEdit,
		"close-quote": PairCloseQuote,
	}[ruleID]
	if rule == 0 {
		return kernel.OutcomePass, nil
	}
	settings := PairSettings{
		Quotes:           contextMarkupBool(context, "quotes", true),
		Overtype:         contextMarkupBool(context, "overtype", true),
		EditMates:        contextMarkupBool(context, "edit-mates", true),
		ContextualQuotes: contextMarkupBool(context, "contextual-quotes", true),
	}
	pairInput := PairInput{
		Range:       input.Target().RangeUTF16(),
		Replacement: input.Text(),
		Origin:      origin,
	}
	if !PairMayApply(rule, pairInput, settings) {
		return kernel.OutcomePass, nil
	}
	text := ""
	if node.TextPresent() {
		text = node.Text().String()
	}
	var plan PairPlan
	if inFormula {
		plan, err = planFormulaPair(rule, text, scope, pairInput, settings)
	} else {
		plan, err = PlanPair(rule, text, pairInput, settings)
	}
	if err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	if !plan.Handled {
		return kernel.OutcomePass, nil
	}
	for _, edit := range plan.Edits.Edits() {
		if err := document.ReplaceTextMarked(
			node.ID(),
			edit.Range.Start,
			edit.Range.End,
			edit.Replacement,
			i.active,
		); err != nil {
			return 0, err
		}
	}
	i.proposal.land(Landing{Head: plan.Head, Through: document.TextEditCount()})
	return kernel.OutcomeHandled, nil
}

func (i *EditorInvoker) invokeCorrection(
	context *kernel.RuleContext,
	policy kernel.PolicyID,
) (kernel.Outcome, error) {
	input, hasInput := context.Event().TextInput()
	// Text that broke into blocks left the block it was typed in; the
	// block it ended in is refined as it is typed on.
	if !hasInput || strings.ContainsRune(input.Text(), '\n') {
		return kernel.OutcomePass, nil
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	nodeID := input.Target().Node()
	node, ok := document.Node(nodeID)
	if !ok || !node.TextPresent() || literalAt(node, i.active) ||
		i.landsElsewhere(nodeID) {
		return kernel.OutcomePass, nil
	}
	var batches []CorrectionBatch
	if policy == TypographyPolicyID {
		settings := TypographySettings{
			Symbols:        contextMarkupBool(context, "symbols", true),
			Punctuation:    contextMarkupBool(context, "punctuation", true),
			QuotePlacement: contextMarkupBool(context, "quote-placement", true),
		}
		if !TypographyTrigger(input.Text()) {
			return kernel.OutcomePass, nil
		}
		text := node.Text().String()
		plan, ok := PlanTypography(text, i.correctionHead(input, document.TextEditCount()), settings)
		if !ok {
			return kernel.OutcomePass, nil
		}
		if !plan.Empty() {
			batches = append(batches, plan)
		}
	} else {
		settings := CJKSettings{
			Punctuation: contextMarkupBool(context, "punctuation", true),
			Spacing:     contextMarkupBool(context, "spacing", true),
		}
		plans, err := planCJKTree(node.Text(), input.Target().RangeUTF16(), settings)
		if err != nil {
			return 0, err
		}
		for _, plan := range plans {
			batches = append(batches, plan.Corrections)
		}
	}
	if len(batches) == 0 {
		return kernel.OutcomePass, nil
	}
	for _, batch := range batches {
		for _, edit := range batch.Edits() {
			// A refinement inside the active mark stays inside it, as the
			// typing it refines did.
			start := document.TextEditCount()
			if err := document.ReplaceTextMarked(
				nodeID,
				edit.Range.Start,
				edit.Range.End,
				edit.Replacement,
				i.active,
			); err != nil {
				return 0, err
			}
			for index := start; index < document.TextEditCount(); index++ {
				i.corrections.Record(index)
			}
		}
	}
	return kernel.OutcomeHandled, nil
}

// landsElsewhere reports whether the caret was proposed to land in a block
// other than node, which refining node would not follow.
func (i *EditorInvoker) landsElsewhere(node model.NodeID) bool {
	landing, ok := i.proposal.Landing()
	return ok && landing.Node != 0 && landing.Node != node
}

// correctionHead is where the caret stands in the text a refinement reads:
// the proposed landing when it reads that text, else after the input.
func (i *EditorInvoker) correctionHead(input kernel.TextInput, edits int) model.UTF16Offset {
	if landing, ok := i.proposal.Landing(); ok && landing.current(edits) {
		return landing.Head
	}
	return input.Target().RangeUTF16().Start +
		model.UTF16Offset(model.StringLenUTF16(input.Text()))
}

func contextMarkupBool(
	context *kernel.RuleContext,
	id string,
	fallback bool,
) bool {
	if context == nil {
		return fallback
	}
	value, ok := context.Setting(kernel.SettingID(id))
	if !ok {
		return fallback
	}
	boolean, ok := value.Bool()
	if !ok {
		return fallback
	}
	return boolean
}

func cloneMarkPointer(id *model.MarkID) *model.MarkID {
	if id == nil {
		return nil
	}
	value := *id
	return &value
}
