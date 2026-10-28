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

	legacyEmphasisAttribute = "emphasis"
	allowStrongSetting      = "allow-strong"
	underscoresSetting      = "underscores"
	markupFormatTrigger     = "markup:format"
)

// EditorPolicy is the stable identity of one concrete editor policy.
type EditorPolicy string

const (
	EditorPolicyText       EditorPolicy = "editor.text"
	EditorPolicyMarkup     EditorPolicy = "editor.markup"
	EditorPolicyPairs      EditorPolicy = "editor.pairs"
	EditorPolicyCJK        EditorPolicy = "editor.cjk"
	EditorPolicyTypography EditorPolicy = "editor.typography"
	EditorPolicyHistory    EditorPolicy = "editor.history"
	EditorPolicyNavigation EditorPolicy = "editor.navigation"
	EditorPolicyWords      EditorPolicy = "editor.words"
)

// AllEditorPolicies is the Rust assembly order.
var AllEditorPolicies = [...]EditorPolicy{
	EditorPolicyText,
	EditorPolicyMarkup,
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

// TextCommand is one semantic text-editing or navigation command.
type TextCommand uint8

const (
	TextLeft TextCommand = iota + 1
	TextRight
	TextSelectLeft
	TextSelectRight
	TextLineStart
	TextLineEnd
	TextSelectLineStart
	TextSelectLineEnd
	TextDocumentStart
	TextDocumentEnd
	TextSelectDocumentStart
	TextSelectDocumentEnd
	TextSelectAll
	TextBackspace
	TextDelete
	TextNewline
)

// AllTextCommands is the declaration order used by the Rust command macro.
var AllTextCommands = [...]TextCommand{
	TextLeft,
	TextRight,
	TextSelectLeft,
	TextSelectRight,
	TextLineStart,
	TextLineEnd,
	TextSelectLineStart,
	TextSelectLineEnd,
	TextDocumentStart,
	TextDocumentEnd,
	TextSelectDocumentStart,
	TextSelectDocumentEnd,
	TextSelectAll,
	TextBackspace,
	TextDelete,
	TextNewline,
}

// Description returns stable command metadata.
func (c TextCommand) Description() Command {
	meta := Command{Policy: TextPolicyID, BindingCtx: kernel.EditorBindingContext}
	switch c {
	case TextLeft:
		meta.ID, meta.Label, meta.Key = "text.left", "Move left", "left"
	case TextRight:
		meta.ID, meta.Label, meta.Key = "text.right", "Move right", "right"
	case TextSelectLeft:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-left", "Extend left", "left", kernel.ModifiersShift
	case TextSelectRight:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-right", "Extend right", "right", kernel.ModifiersShift
	case TextLineStart:
		meta.ID, meta.Label, meta.Key = "text.line-start", "Start of line", "home"
	case TextLineEnd:
		meta.ID, meta.Label, meta.Key = "text.line-end", "End of line", "end"
	case TextSelectLineStart:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-line-start", "Extend to line start", "home", kernel.ModifiersShift
	case TextSelectLineEnd:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-line-end", "Extend to line end", "end", kernel.ModifiersShift
	case TextDocumentStart:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.document-start", "Start of document", "home", kernel.ModifiersPrimary
	case TextDocumentEnd:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.document-end", "End of document", "end", kernel.ModifiersPrimary
	case TextSelectDocumentStart:
		meta.ID, meta.Label, meta.Key = "text.select-document-start", "Extend to document start", "home"
		meta.Modifiers = kernel.ModifiersPrimary | kernel.ModifiersShift
	case TextSelectDocumentEnd:
		meta.ID, meta.Label, meta.Key = "text.select-document-end", "Extend to document end", "end"
		meta.Modifiers = kernel.ModifiersPrimary | kernel.ModifiersShift
	case TextSelectAll:
		meta.ID, meta.Label, meta.Key, meta.Modifiers = "text.select-all", "Select all", "a", kernel.ModifiersPrimary
	case TextBackspace:
		meta.ID, meta.Label, meta.Key = "text.backspace", "Delete backward", "backspace"
	case TextDelete:
		meta.ID, meta.Label, meta.Key = "text.delete", "Delete forward", "delete"
	case TextNewline:
		meta.ID, meta.Label, meta.Key = "text.newline", "Insert newline", "enter"
	}
	return meta
}

// ID returns the stable command identity.
func (c TextCommand) ID() kernel.CommandID { return c.Description().ID }

// HistoryCommand is one semantic undo-stack command.
type HistoryCommand uint8

const (
	HistoryUndo HistoryCommand = iota + 1
	HistoryRedo
)

// AllHistoryCommands is the Rust declaration order.
var AllHistoryCommands = [...]HistoryCommand{HistoryUndo, HistoryRedo}

// Description returns stable command metadata.
func (c HistoryCommand) Description() Command {
	if c == HistoryRedo {
		return Command{
			ID:         "history.redo",
			Label:      "Redo",
			Key:        "z",
			Modifiers:  kernel.ModifiersPrimary | kernel.ModifiersShift,
			Policy:     HistoryPolicyID,
			BindingCtx: kernel.EditorBindingContext,
		}
	}
	return Command{
		ID:         "history.undo",
		Label:      "Undo",
		Key:        "z",
		Modifiers:  kernel.ModifiersPrimary,
		Policy:     HistoryPolicyID,
		BindingCtx: kernel.EditorBindingContext,
	}
}

// ID returns the stable command identity.
func (c HistoryCommand) ID() kernel.CommandID { return c.Description().ID }

// EditorPolicyDescriptors returns the complete Rust policy assembly in stable
// declaration order.
func EditorPolicyDescriptors() ([]kernel.PolicyDescriptor, error) {
	text, err := textPolicyDescriptor()
	if err != nil {
		return nil, err
	}
	markup, err := markupPolicyDescriptor()
	if err != nil {
		return nil, err
	}
	pairs, err := pairsPolicyDescriptor()
	if err != nil {
		return nil, err
	}
	cjk, err := observerPolicyDescriptor(
		EditorPolicyCJK,
		300,
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
	typography, err := observerPolicyDescriptor(
		EditorPolicyTypography,
		320,
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
	}, nil
}

// EditorPolicyCatalog links the complete editor declaration set.
func EditorPolicyCatalog() (kernel.PolicyCatalog, error) {
	descriptors, err := EditorPolicyDescriptors()
	if err != nil {
		return kernel.PolicyCatalog{}, err
	}
	return kernel.LinkPolicyCatalog(descriptors)
}

func textPolicyDescriptor() (kernel.PolicyDescriptor, error) {
	algorithm, err := singleLeafProgram()
	if err != nil {
		return kernel.PolicyDescriptor{}, err
	}
	commands := make([]kernel.CommandID, 0, len(AllTextCommands))
	bindings := make([]kernel.BindingSpec, 0, len(AllTextCommands))
	for _, command := range AllTextCommands {
		meta := command.Description()
		commands = append(commands, meta.ID)
		bindings = append(bindings, kernel.BindingSpec{Command: meta.ID, Target: meta.Binding()})
	}
	edit := kernel.RuleDescriptor{
		ID:        "edit",
		Phase:     kernel.HandleRulePhase(),
		Trigger:   kernel.NewExactTrigger(kernel.TextInputKey),
		Order:     100,
		Effects:   kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite),
		Algorithm: algorithm,
	}
	projected := edit
	projected.ID = "projected-edit"
	projected.Trigger = kernel.NewExactTrigger(kernel.ProjectedTextKey)
	return kernel.PolicyDescriptor{
		ID:               TextPolicyID,
		EnabledByDefault: true,
		Commands:         commands,
		Bindings:         bindings,
		Rules:            []kernel.RuleDescriptor{edit, projected},
	}, nil
}

func markupPolicyDescriptor() (kernel.PolicyDescriptor, error) {
	algorithm, err := singleLeafProgram()
	if err != nil {
		return kernel.PolicyDescriptor{}, err
	}
	commands := []kernel.CommandID{MarkupEmphasis.ID(), MarkupStrong.ID()}
	bindings := []kernel.BindingSpec{
		{Command: MarkupEmphasis.ID(), Target: MarkupEmphasis.Description().Binding()},
		{Command: MarkupStrong.ID(), Target: MarkupStrong.Description().Binding()},
	}
	edit := kernel.RuleDescriptor{
		ID:        "edit",
		Phase:     kernel.HandleRulePhase(),
		Trigger:   kernel.NewExactTrigger(kernel.AttributeInputKey),
		Order:     100,
		Effects:   kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite),
		Algorithm: algorithm,
	}
	marker := edit
	marker.ID = "type-marker"
	marker.Trigger = kernel.NewExactTrigger(kernel.TextInputKey)
	marker.Origins = []kernel.EventOrigin{kernel.EventOriginTyping}
	marker.Order = 10
	format := edit
	format.ID = "format"
	format.Trigger = kernel.NewExactTrigger(markupFormatTrigger)
	return kernel.PolicyDescriptor{
		ID:               MarkupPolicyID,
		EnabledByDefault: true,
		Settings: []kernel.SettingDescriptor{
			{ID: allowStrongSetting, Default: kernel.BoolSetting(true)},
			{ID: underscoresSetting, Default: kernel.BoolSetting(false)},
		},
		Commands: commands,
		Bindings: bindings,
		Rules:    []kernel.RuleDescriptor{edit, marker, format},
	}, nil
}

func pairsPolicyDescriptor() (kernel.PolicyDescriptor, error) {
	algorithm, err := singleLeafProgram()
	if err != nil {
		return kernel.PolicyDescriptor{}, err
	}
	declarations := PairRuleDeclarations()
	rules := make([]kernel.RuleDescriptor, 0, len(declarations))
	for _, ruleID := range []string{"open", "overtype", "mate-edit", "close-quote"} {
		var declaration RuleDeclaration
		for _, candidate := range declarations {
			if candidate.ID == kernel.RuleID(ruleID) {
				declaration = candidate
				break
			}
		}
		effects := kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite)
		rule := kernel.RuleDescriptor{
			ID:        kernel.RuleID(declaration.ID),
			Phase:     kernel.HandleRulePhase(),
			Trigger:   kernel.NewExactTrigger(TextInputKey),
			Origins:   append([]kernel.EventOrigin(nil), declaration.Origins...),
			Order:     uint32(declaration.Order),
			Effects:   effects,
			Algorithm: algorithm,
		}
		rules = append(rules, rule)
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
	order uint32,
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
			Trigger:   kernel.NewExactTrigger(TextInputKey),
			Origins:   append([]kernel.EventOrigin(nil), origins...),
			Order:     order,
			Effects:   kernel.NewEffectSet(nil, nil).WithDocument(kernel.DocumentAccessWrite),
			Algorithm: algorithm,
		}},
	}, nil
}

func historyPolicyDescriptor() kernel.PolicyDescriptor {
	commands := make([]kernel.CommandID, 0, len(AllHistoryCommands))
	bindings := make([]kernel.BindingSpec, 0, len(AllHistoryCommands))
	for _, command := range AllHistoryCommands {
		meta := command.Description()
		commands = append(commands, meta.ID)
		bindings = append(bindings, kernel.BindingSpec{Command: meta.ID, Target: meta.Binding()})
	}
	return kernel.PolicyDescriptor{
		ID:               HistoryPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Commands:         commands,
		Bindings:         bindings,
	}
}

func navigationPolicyDescriptor() kernel.PolicyDescriptor {
	meta := TabCommand()
	return kernel.PolicyDescriptor{
		ID:               NavigationPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Settings: []kernel.SettingDescriptor{
			{ID: "pairs", Default: kernel.BoolSetting(true)},
			{ID: "emphasis", Default: kernel.BoolSetting(true)},
			{ID: "return-to-edit", Default: kernel.BoolSetting(true)},
		},
		Commands: []kernel.CommandID{meta.ID},
		Bindings: []kernel.BindingSpec{{Command: meta.ID, Target: meta.Binding()}},
	}
}

func wordsPolicyDescriptor() kernel.PolicyDescriptor {
	commands := make([]kernel.CommandID, 0, len(AllWordCommands))
	bindings := make([]kernel.BindingSpec, 0, len(AllWordCommands))
	for _, command := range AllWordCommands {
		meta := command.Description()
		commands = append(commands, meta.ID)
		bindings = append(bindings, kernel.BindingSpec{Command: meta.ID, Target: meta.Binding()})
	}
	return kernel.PolicyDescriptor{
		ID:               WordsPolicyID,
		Requires:         []kernel.PolicyID{TextPolicyID},
		EnabledByDefault: true,
		Commands:         commands,
		Bindings:         bindings,
	}
}

func singleLeafProgram() (*planning.ControlProgram, error) {
	builder := &planning.ControlBuilder{}
	root := builder.Leaf(planning.NewLeafID(0))
	return builder.Build(root)
}

// MarkupPolicy owns semantic inline emphasis projection.
type MarkupPolicy struct{}

// Project builds a source-validated projection from one engine snapshot.
func (MarkupPolicy) Project(
	snapshot kernel.EngineSnapshot,
	nodeID model.NodeID,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	return MarkupProject(snapshot, nodeID, generation, configuration)
}

// ProjectDocument builds a projection from a bare document snapshot.
func (MarkupPolicy) ProjectDocument(
	document model.DocumentSnapshot,
	revision uint64,
	nodeID model.NodeID,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	return MarkupProjectDocument(document, revision, nodeID, generation, configuration)
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

// MarkupProject builds the projected surface for one semantic node.
func MarkupProject(
	snapshot kernel.EngineSnapshot,
	nodeID model.NodeID,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	return MarkupProjectDocument(snapshot.Document(), snapshot.Revision(), nodeID, generation, configuration)
}

// MarkupProjectDocument builds and source-validates a projection from a bare
// semantic document snapshot.
func MarkupProjectDocument(
	document model.DocumentSnapshot,
	revision uint64,
	nodeID model.NodeID,
	generation uint64,
	configuration *kernel.PreparedPolicyConfiguration,
) (*projection.Projection, error) {
	node, ok := document.Node(nodeID)
	if !ok {
		return nil, &MarkupProjectionError{
			Kind: projection.ErrorSource,
			Err:  &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: nodeID},
		}
	}
	enabled := markupEnabled(configuration)
	count, err := legacyEmphasisCount(node, enabled)
	if err != nil {
		return nil, err
	}
	marker := "*"
	if markupSettingBool(configuration, underscoresSetting, false) {
		marker = "_"
	}
	notation := strings.Repeat(marker, count)
	builder := projection.NewBuilder(
		projection.ProjectionID(fmt.Sprintf("markup/%d", nodeID)),
		projection.NewVersion(revision, generation),
	)
	if count == 0 {
		if err := builder.Generated(""); err != nil {
			return nil, wrapProjectionError(err)
		}
	} else if err := builder.Attribute(nodeID, legacyEmphasisAttribute, notation); err != nil {
		return nil, wrapProjectionError(err)
	}
	if err := projectInlineBody(builder, node, marker, enabled); err != nil {
		return nil, wrapProjectionError(err)
	}
	if count == 0 {
		if err := builder.Generated(""); err != nil {
			return nil, wrapProjectionError(err)
		}
	} else if err := builder.Attribute(nodeID, legacyEmphasisAttribute, notation); err != nil {
		return nil, wrapProjectionError(err)
	}
	result, err := builder.Build(nil)
	if err != nil {
		return nil, wrapProjectionError(err)
	}
	if err := result.ValidateSource(revision, document); err != nil {
		return nil, wrapProjectionError(err)
	}
	return result, nil
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
	marker string,
	enabled bool,
) error {
	length := model.UTF16Offset(0)
	if node.TextPresent() {
		length = model.UTF16Offset(node.Text().UTF16Len())
	}
	if !enabled || node.MarkCount() == 0 {
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
		if err := appendInlineMarker(builder, node, event.mark, marker); err != nil {
			return err
		}
		if event.kind == 2 {
			if err := builder.Editable(node, model.NewUTF16Range(cursor, cursor)); err != nil {
				return err
			}
			if err := appendInlineMarker(builder, node, event.mark, marker); err != nil {
				return err
			}
		}
	}
	if cursor < length {
		return builder.Editable(node, model.NewUTF16Range(cursor, length))
	}
	return nil
}

func appendInlineMarker(
	builder *projection.Builder,
	node *model.Node,
	mark model.InlineMark,
	marker string,
) error {
	return builder.Attribute(
		node.ID(),
		MarkerKeyForMark(mark.ID()),
		strings.Repeat(marker, int(mark.Level())),
	)
}

func legacyEmphasisCount(node *model.Node, enabled bool) (int, error) {
	value, ok := node.Attribute(legacyEmphasisAttribute)
	if !ok {
		return 0, nil
	}
	count, ok := value.Integer()
	if !ok || count < 0 || count > 2 {
		return 0, &MarkupProjectionError{
			Kind: projection.ErrorSource,
			Err:  &model.DocumentError{Kind: model.DocumentErrorInvalidTextRange, Node: node.ID()},
		}
	}
	if !enabled {
		return 0, nil
	}
	return int(count), nil
}

func markupEnabled(configuration *kernel.PreparedPolicyConfiguration) bool {
	return configuration == nil || configuration.IsEnabled(MarkupPolicyID)
}

func markupSettingBool(
	configuration *kernel.PreparedPolicyConfiguration,
	id string,
	fallback bool,
) bool {
	if configuration == nil {
		return fallback
	}
	value, ok := configuration.Setting(NewMarkupSettingKey(id))
	if !ok {
		return fallback
	}
	boolean, ok := value.Bool()
	if !ok {
		return fallback
	}
	return boolean
}

// NewMarkupSettingKey returns a setting key owned by editor.markup.
func NewMarkupSettingKey(id string) kernel.SettingKey {
	return EditorPolicyMarkup.Setting(id)
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
	active    *model.MarkID
	lastMark  MarkerAction
	hasMarker bool
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

// LastMarker returns the last handled marker action.
func (i *EditorInvoker) LastMarker() (MarkerAction, bool) {
	return i.lastMark, i.hasMarker
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
	switch rule.Key.Owner {
	case TextPolicyID:
		return i.invokeText(context)
	case MarkupPolicyID:
		switch rule.Key.ID {
		case "type-marker":
			return i.invokeMarker(context)
		case "edit":
			return i.invokeAttribute(context)
		}
	}
	return kernel.OutcomePass, nil
}

func (i *EditorInvoker) invokeText(context *kernel.RuleContext) (kernel.Outcome, error) {
	if projected, ok := context.Event().ProjectedEdit(); ok && !projected.ApplyText() {
		return kernel.OutcomePass, nil
	}
	if i.active == nil {
		return context.ApplyTextInput()
	}
	_, _, handled, err := context.ReplaceTextInputMarked(i.active)
	if err != nil {
		return 0, err
	}
	if !handled {
		return kernel.OutcomePass, nil
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
	settings := MarkerSettings{
		Enabled:     true,
		AllowStrong: contextMarkupBool(context, allowStrongSetting, true),
	}
	action, err := PlanMarkerTypes(
		node,
		input.Target().RangeUTF16(),
		input.Text(),
		i.active,
		settings,
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
	i.lastMark = action
	i.hasMarker = true
	switch {
	case action.Exited:
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
	if key != legacyEmphasisAttribute && !MarkerKeyIsInline(key) {
		return kernel.OutcomePass, nil
	}
	marker := "*"
	if contextMarkupBool(context, underscoresSetting, false) {
		marker = "_"
	}
	count := len(input.Text())
	valid := count <= 2
	for _, r := range input.Text() {
		valid = valid && string(r) == marker
	}
	if count == 2 && !contextMarkupBool(context, allowStrongSetting, true) {
		valid = false
	}
	if !valid {
		return 0, &kernel.EngineError{
			Kind: kernel.EngineErrorInput,
			Err: &kernel.InputError{
				Kind: kernel.InputErrorInvalidAttributeInput,
				Node: input.Target().Node(),
				Key:  key,
			},
		}
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
	if MarkerKeyIsInline(key) {
		var mark *model.InlineMark
		for _, candidate := range node.Marks() {
			if MarkerKeyIdentifies(key, candidate.ID()) {
				value := candidate
				mark = &value
				break
			}
		}
		if mark == nil {
			return 0, &kernel.EngineError{
				Kind: kernel.EngineErrorInput,
				Err: &kernel.InputError{
					Kind: kernel.InputErrorInvalidAttributeInput,
					Node: input.Target().Node(),
					Key:  key,
				},
			}
		}
		if err := document.SetMarkLevel(node.ID(), mark.ID(), uint8(count)); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
		if count == 0 {
			i.active = nil
		} else {
			id := mark.ID()
			i.active = &id
		}
		return kernel.OutcomeHandled, nil
	}
	if count == 0 {
		if err := document.RemoveAttribute(node.ID(), legacyEmphasisAttribute); err != nil {
			return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
		}
	} else if err := document.SetAttribute(
		node.ID(),
		legacyEmphasisAttribute,
		model.IntegerValue(int64(count)),
	); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return kernel.OutcomeHandled, nil
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
