package kernel

import (
	"fmt"
	"slices"
	"sort"

	"github.com/caelyreth/neverland/engine/kernel/planning"
	"github.com/caelyreth/neverland/engine/model"
)

type RuleKey struct {
	Owner PolicyID
	ID    RuleID
}

func (k RuleKey) String() string {
	return fmt.Sprintf("%s/%s", k.Owner, k.ID)
}

type LinkedRule struct {
	Key       RuleKey
	Trigger   Trigger
	Origins   []EventOrigin
	Scope     ScopeKind
	rank      int
	Effects   EffectSet
	Access    EffectSet
	Condition *CompiledCondition
	Phase     RulePhase
	Prepared  *planning.PreparedControl
}

func (r *LinkedRule) AllowsOrigin(origin EventOrigin) bool {
	return OriginMatches(r.Origins, origin)
}

type EventProgram struct {
	handlers      []*LinkedRule
	after         []*LinkedRule
	observers     []*LinkedRule
	schedule      *ObserverProgram
	eventPlans    map[string]*eventPlan
	configuration *PreparedPolicyConfiguration
}

type eventPlan struct {
	handlers  [EventOriginSemantic + 1][]*LinkedRule
	observers [EventOriginSemantic + 1][]*LinkedRule
}

func LinkEventProgram(catalog PolicyCatalog) (*EventProgram, error) {
	rules := catalog.Rules()
	handlers := make([]*LinkedRule, 0, len(rules))
	after := make([]*LinkedRule, 0, len(rules))
	all := make([]*LinkedRule, 0, len(rules))
	declared := make(map[RuleKey]RuleDescriptor, len(rules))
	for _, entry := range rules {
		rule, err := linkRule(entry.Policy, entry.Rule)
		if err != nil {
			return nil, err
		}
		all = append(all, rule)
		declared[rule.Key] = entry.Rule
	}
	order, err := orderRules(all, declared)
	if err != nil {
		return nil, err
	}
	for _, rule := range all {
		if rule.Trigger.Kind() == TriggerStateWildcard {
			if rule.Phase.Kind != RulePhaseKindAfter ||
				len(rule.Phase.ChangeKeys()) == 0 ||
				len(rule.Origins) != 0 {
				return nil, &DispatchError{
					Kind: DispatchErrorInvalidStateObserver,
					Rule: rule.Key,
				}
			}
			after = append(after, rule)
		} else if rule.Phase.Kind == RulePhaseKindAfter {
			after = append(after, rule)
		} else {
			handlers = append(handlers, rule)
		}
	}
	sortRules(handlers)
	sortRules(after)
	for index, first := range handlers {
		for _, second := range handlers[index+1:] {
			if first.Scope == second.Scope && !order.related(first.Key, second.Key) &&
				first.Trigger.Overlaps(second.Trigger) &&
				OriginsOverlap(first.Origins, second.Origins) {
				return nil, &DispatchError{
					Kind:   DispatchErrorAmbiguousOrder,
					First:  first.Key,
					Second: second.Key,
				}
			}
		}
	}
	schedule, err := linkObserverProgram(all, order)
	if err != nil {
		return nil, err
	}
	return &EventProgram{
		handlers:   handlers,
		after:      after,
		observers:  schedule.Rules(),
		schedule:   schedule,
		eventPlans: linkEventPlans(handlers, after),
	}, nil
}

func LinkConfiguredEventProgram(
	catalog PolicyCatalog,
	configuration *PreparedPolicyConfiguration,
) (*EventProgram, error) {
	if configuration == nil {
		return LinkEventProgram(catalog)
	}
	filtered := make([]PolicyDescriptor, 0)
	for _, descriptor := range catalog.Policies() {
		if configuration.IsEnabled(descriptor.ID) {
			filtered = append(filtered, descriptor)
		}
	}
	linked, err := LinkPolicyCatalog(filtered)
	if err != nil {
		return nil, err
	}
	// Preserve dependency-owned declarations only for enabled policies; linked
	// rule validation has already happened in the complete catalog.
	program, err := LinkEventProgram(linked)
	if err != nil {
		return nil, err
	}
	program.configuration = configuration
	return program, nil
}

func linkRule(owner PolicyID, descriptor RuleDescriptor) (*LinkedRule, error) {
	if err := descriptor.Trigger.Validate(); err != nil {
		return nil, &DispatchError{
			Kind:    DispatchErrorInvalidTrigger,
			Trigger: descriptor.Trigger.String(),
		}
	}
	var condition *CompiledCondition
	if descriptor.When != nil {
		compiled := CompileCondition(*descriptor.When)
		condition = &compiled
	}
	effects := descriptor.Effects.WithStateReads(descriptor.Phase.ChangeKeys())
	if condition != nil {
		effects = effects.WithReads(condition.ResourceReads())
	}
	var preparedControl planning.PreparedControl
	if descriptor.Algorithm == nil {
		builder := planning.ControlBuilder{}
		leaf := builder.Leaf(planning.NewLeafID(0))
		program, err := builder.Build(leaf)
		if err != nil {
			return nil, err
		}
		preparedControl = program.Prepare(planning.DefaultControlPlanBudget)
	} else {
		leaves := descriptor.Algorithm.Leaves()
		if len(leaves) != 1 {
			return nil, &DispatchError{
				Kind:   DispatchErrorInvalidRulePlan,
				Rule:   RuleKey{Owner: owner, ID: descriptor.ID},
				Leaves: leaves,
			}
		}
		preparedControl = descriptor.Algorithm.Prepare(planning.DefaultControlPlanBudget)
	}
	return &LinkedRule{
		Key:       RuleKey{Owner: owner, ID: descriptor.ID},
		Trigger:   descriptor.Trigger,
		Origins:   append([]EventOrigin(nil), descriptor.Origins...),
		Scope:     descriptor.Scope,
		Effects:   effects,
		Access:    descriptor.Effects.Clone(),
		Condition: condition,
		Phase:     descriptor.Phase.clone(),
		Prepared:  &preparedControl,
	}, nil
}

func sortRules(rules []*LinkedRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].rank < rules[j].rank
	})
}

type DispatchErrorKind uint8

const (
	DispatchErrorInvalidTrigger DispatchErrorKind = iota + 1
	DispatchErrorAmbiguousOrder
	DispatchErrorInvalidStateObserver
	DispatchErrorBackwardObservation
	DispatchErrorInvalidRulePlan
	DispatchErrorCyclicOrder
)

type DispatchError struct {
	Kind    DispatchErrorKind
	Trigger string
	Rule    RuleKey
	First   RuleKey
	Second  RuleKey
	Leaves  []LeafID
}

func (e *DispatchError) Error() string {
	switch e.Kind {
	case DispatchErrorInvalidTrigger:
		return "invalid event trigger " + e.Trigger
	case DispatchErrorAmbiguousOrder:
		return fmt.Sprintf("ambiguous order between %s and %s", e.First, e.Second)
	case DispatchErrorInvalidStateObserver:
		return fmt.Sprintf("state observer %s requires an after phase, change dependencies and no origin restriction", e.Rule)
	case DispatchErrorBackwardObservation:
		return fmt.Sprintf("backward or cyclic observation: %s -> %s", e.First, e.Second)
	case DispatchErrorCyclicOrder:
		return fmt.Sprintf("rule %s is declared both before and after another", e.Rule)
	case DispatchErrorInvalidRulePlan:
		return fmt.Sprintf("rule %s must own exactly one leaf, found %d", e.Rule, len(e.Leaves))
	default:
		return "unknown dispatch error"
	}
}

type DispatchOutcome struct {
	Status      Outcome
	HandledRule *RuleKey
	Candidates  int
	Observers   []RuleKey
}

type BatchDispatchOutcome struct {
	Outcome DispatchOutcome
	Handled []bool
}

type PlanAnalysis struct {
	Handlers      int
	Observers     int
	DynamicGuards int
}

func (p *EventProgram) AnalyzeEvent(event Event) PlanAnalysis {
	handlers, observers := p.plan(event)
	dynamic := 0
	for _, rule := range handlers {
		if rule.Condition != nil {
			dynamic++
		}
	}
	for _, rule := range observers {
		if rule.Condition != nil {
			dynamic++
		}
	}
	return PlanAnalysis{Handlers: len(handlers), Observers: len(observers), DynamicGuards: dynamic}
}

// Handlers are the rules that may answer event, in the order they are
// asked.
func (p *EventProgram) Handlers(event Event) []RuleKey {
	handlers, _ := p.plan(event)
	keys := make([]RuleKey, 0, len(handlers))
	for _, rule := range scoped(handlers, event.Scopes()) {
		keys = append(keys, rule.Key)
	}
	return keys
}

func (p *EventProgram) plan(event Event) ([]*LinkedRule, []*LinkedRule) {
	origin := normalizedEventOrigin(event)
	if plan := p.eventPlans[event.Key()]; plan != nil &&
		origin >= EventOriginTyping && origin <= EventOriginSemantic {
		return plan.handlers[origin], plan.observers[origin]
	}
	return p.planLinear(event.Key(), origin)
}

func (p *EventProgram) planLinear(key string, origin EventOrigin) ([]*LinkedRule, []*LinkedRule) {
	handlers := make([]*LinkedRule, 0, len(p.handlers))
	for _, rule := range p.handlers {
		if handlerMatches(rule, key, origin) {
			handlers = append(handlers, rule)
		}
	}
	observers := make([]*LinkedRule, 0, len(p.after))
	for _, observer := range p.after {
		if afterMatches(observer, key, origin) {
			observers = append(observers, observer)
		}
	}
	return handlers, observers
}

func handlerMatches(rule *LinkedRule, key string, origin EventOrigin) bool {
	return rule.Trigger.Matches(key) && rule.AllowsOrigin(origin)
}

func afterMatches(rule *LinkedRule, key string, origin EventOrigin) bool {
	return rule.AllowsOrigin(origin) &&
		(rule.Trigger.Kind() == TriggerStateWildcard || rule.Trigger.Matches(key))
}

func linkEventPlans(handlers, observers []*LinkedRule) map[string]*eventPlan {
	keys := make(map[string]struct{})
	for _, rule := range handlers {
		if rule.Trigger.Kind() == TriggerExact {
			keys[rule.Trigger.Pattern()] = struct{}{}
		}
	}
	for _, observer := range observers {
		if observer.Trigger.Kind() == TriggerExact {
			keys[observer.Trigger.Pattern()] = struct{}{}
		}
	}
	plans := make(map[string]*eventPlan, len(keys))
	for key := range keys {
		plan := &eventPlan{}
		for origin := EventOriginTyping; origin <= EventOriginSemantic; origin++ {
			for _, rule := range handlers {
				if handlerMatches(rule, key, origin) {
					plan.handlers[origin] = append(plan.handlers[origin], rule)
				}
			}
			for _, observer := range observers {
				if afterMatches(observer, key, origin) {
					plan.observers[origin] = append(plan.observers[origin], observer)
				}
			}
		}
		plans[key] = plan
	}
	return plans
}

func normalizedEventOrigin(event Event) EventOrigin {
	origin, ok := event.Origin()
	if !ok {
		return EventOriginTyping
	}
	return origin
}

// EngineInvoker invokes one handler leaf with checked state access.
type EngineInvoker interface {
	Invoke(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error)
}

type EngineInvokerFunc func(*LinkedRule, LeafID, *RuleContext) (Outcome, error)

func (f EngineInvokerFunc) Invoke(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
	return f(rule, leaf, context)
}

func (p *EventProgram) DispatchEngine(
	event Event,
	draft *EngineDraft,
	invoker EngineInvoker,
) (DispatchOutcome, error) {
	if event.HasReservedCommandKey() {
		return DispatchOutcome{}, &EngineError{Kind: EngineErrorInvalidEventKey, Key: event.Key()}
	}
	if err := validateEventInput(event, draft); err != nil {
		return DispatchOutcome{}, err
	}
	checkpoint := draft.Checkpoint()
	outcome, err := p.executeEngine(event, draft, invoker)
	if err != nil {
		draft.Rollback(checkpoint)
		return DispatchOutcome{}, err
	}
	return outcome, nil
}

// DispatchEngineBatch validates every event against the pre-write draft,
// checkpoints once, and executes the events in order. Any error rolls the whole
// batch back, including writes from events that had already handled.
func (p *EventProgram) DispatchEngineBatch(
	events []Event,
	draft *EngineDraft,
	invoker EngineInvoker,
) (BatchDispatchOutcome, error) {
	primaryText := 0
	for _, event := range events {
		if event.HasReservedCommandKey() {
			return BatchDispatchOutcome{}, &EngineError{
				Kind: EngineErrorInvalidEventKey,
				Key:  event.Key(),
			}
		}
		if err := validateEventInput(event, draft); err != nil {
			return BatchDispatchOutcome{}, err
		}
		if eventWritesPrimaryText(event) {
			primaryText++
		}
	}
	if primaryText > 1 {
		return BatchDispatchOutcome{}, &EngineError{
			Kind: EngineErrorInput,
			Err:  &InputError{Kind: InputErrorMultipleTextInputs},
		}
	}
	checkpoint := draft.Checkpoint()
	aggregate := DispatchOutcome{Status: OutcomePass}
	handled := make([]bool, 0, len(events))
	for _, event := range events {
		outcome, err := p.executeEngine(event, draft, invoker)
		if err != nil {
			draft.Rollback(checkpoint)
			return BatchDispatchOutcome{}, err
		}
		handled = append(handled, outcome.Status == OutcomeHandled)
		aggregate.Candidates += outcome.Candidates
		aggregate.Observers = append(aggregate.Observers, outcome.Observers...)
		if outcome.Status == OutcomeHandled {
			aggregate.Status = OutcomeHandled
			if aggregate.HandledRule == nil {
				aggregate.HandledRule = outcome.HandledRule
			}
		}
	}
	return BatchDispatchOutcome{Outcome: aggregate, Handled: handled}, nil
}

// eventWritesPrimaryText reports whether an event owns the one text
// replacement an atomic input batch may make. Attribute-only projected edits
// and commands may accompany that one text intent.
func eventWritesPrimaryText(event Event) bool {
	switch event.Kind() {
	case EventText, EventSpan:
		return true
	case EventProjected:
		projected, ok := event.ProjectedEdit()
		return ok && projected.ApplyText()
	default:
		return false
	}
}

func (p *EventProgram) executeEngine(
	event Event,
	draft *EngineDraft,
	invoker EngineInvoker,
) (DispatchOutcome, error) {
	handlers, observers := p.plan(event)
	handlers = scoped(handlers, event.Scopes())
	candidates := handlers
	for _, rule := range handlers {
		if rule.Condition == nil {
			continue
		}
		candidates = make([]*LinkedRule, 0, len(handlers))
		for _, candidate := range handlers {
			matches, err := evaluateRuleCondition(candidate, event, draft.Resources())
			if err != nil {
				return DispatchOutcome{}, err
			}
			if matches {
				candidates = append(candidates, candidate)
			}
		}
		break
	}
	handled := OutcomePass
	for _, rule := range candidates {
		ruleCheckpoint := draft.Checkpoint()
		status, err := invokeRule(rule, event, draft, invoker, p.configuration)
		if err != nil {
			return DispatchOutcome{}, err
		}
		if status == OutcomePass {
			draft.Rollback(ruleCheckpoint)
			continue
		}
		handled = OutcomeHandled
		handledKey := rule.Key
		handledObservers := make([]RuleKey, 0)
		for _, observer := range observers {
			if !observerChangesSurvived(observer, draft) {
				continue
			}
			matches, err := evaluateRuleCondition(observer, event, draft.Resources())
			if err != nil {
				return DispatchOutcome{}, err
			}
			if !matches {
				continue
			}
			status, err := invokeRule(observer, event, draft, invoker, p.configuration)
			if err != nil {
				return DispatchOutcome{}, err
			}
			if status == OutcomeHandled {
				handledObservers = append(handledObservers, observer.Key)
			}
		}
		return DispatchOutcome{
			Status:      handled,
			HandledRule: &handledKey,
			Candidates:  len(candidates),
			Observers:   handledObservers,
		}, nil
	}
	return DispatchOutcome{
		Status:     handled,
		Candidates: len(candidates),
	}, nil
}

func invokeRule(
	rule *LinkedRule,
	event Event,
	draft *EngineDraft,
	invoker EngineInvoker,
	configuration *PreparedPolicyConfiguration,
) (Outcome, error) {
	checkpoint := draft.Checkpoint()
	changeStart := draft.Document().ChangeCount()
	status, err := planning.TryExecutePrepared(
		rule.Prepared,
		draft,
		&event,
		func(leaf LeafID, event *Event, draft *EngineDraft) (Outcome, error) {
			context := NewRuleContext(
				draft,
				rule,
				*event,
				configuration,
			)
			return invoker.Invoke(rule, leaf, context)
		},
	)
	if err != nil {
		return 0, err
	}
	if status == OutcomePass {
		draft.Rollback(checkpoint)
		return status, nil
	}
	if scope, ok := makeInputScope(draft.Document().BaseSnapshot(), event); ok {
		for _, change := range draft.Document().ChangesSince(changeStart) {
			if !scope.allows(draft.Document(), change.Node) {
				return 0, &EngineError{
					Kind:    EngineErrorOutOfScopeDocumentWrite,
					Target:  scope.target,
					Changed: change.Node,
				}
			}
		}
	}
	return status, nil
}

// inputScope is the document region one event may write. For ordinary
// input it is the target block, its container, and the gap to its original
// neighbors; new blocks from splits may occupy that gap. A span owns every
// block between its endpoints instead.
type inputScope struct {
	target      model.NodeID
	parent      model.NodeID
	left, right model.NodeID
	allowed     map[model.NodeID]struct{}
}

func makeInputScope(base model.DocumentSnapshot, event Event) (inputScope, bool) {
	if span, ok := event.SpanInput(); ok {
		first, last := span.From().Node(), span.To().Node()
		parent, ok := baseNodeParent(base, first)
		if !ok {
			return inputScope{}, false
		}
		container, ok := base.Node(parent)
		if !ok {
			return inputScope{}, false
		}
		allowed := map[model.NodeID]struct{}{first: {}, last: {}, parent: {}}
		from, to := -1, -1
		for index, child := range container.Children() {
			switch child {
			case first:
				from = index
			case last:
				to = index
			}
		}
		if from < 0 || to < 0 || to < from {
			return inputScope{}, false
		}
		for _, child := range container.Children()[from : to+1] {
			allowed[child] = struct{}{}
		}
		scope := inputScope{target: first, parent: parent, left: first, allowed: allowed}
		if to+1 < len(container.Children()) {
			scope.right = container.Children()[to+1]
		}
		return scope, true
	}
	target, ok := eventTargetNode(event)
	if !ok {
		return inputScope{}, false
	}
	parent, ok := baseNodeParent(base, target)
	if !ok {
		return inputScope{}, false
	}
	scope := inputScope{
		target: target, parent: parent,
		allowed: map[model.NodeID]struct{}{target: {}, parent: {}},
	}
	container, ok := base.Node(parent)
	if !ok {
		return scope, true
	}
	for index, child := range container.Children() {
		if child != target {
			continue
		}
		if index > 0 {
			scope.left = container.Children()[index-1]
			scope.allowed[scope.left] = struct{}{}
		}
		if index+1 < len(container.Children()) {
			scope.right = container.Children()[index+1]
			scope.allowed[scope.right] = struct{}{}
		}
		break
	}
	return scope, true
}

func (s inputScope) allows(document *model.DocumentDraft, changed model.NodeID) bool {
	if _, ok := s.allowed[changed]; ok {
		return true
	}
	if _, existed := document.BaseSnapshot().Node(changed); existed {
		return false
	}
	parent, ok := nodeParent(document, changed)
	if !ok || parent != s.parent {
		return false
	}
	container, ok := document.Node(parent)
	if !ok {
		return false
	}
	index := slices.Index(container.Children(), changed)
	if index < 0 {
		return false
	}
	left, right := -1, len(container.Children())
	if s.left != 0 {
		left = slices.Index(container.Children(), s.left)
	}
	if s.right != 0 {
		right = slices.Index(container.Children(), s.right)
	}
	return left < index && index < right
}

func baseNodeParent(base model.DocumentSnapshot, id model.NodeID) (model.NodeID, bool) {
	node, ok := base.Node(id)
	if !ok {
		return 0, false
	}
	return node.Parent()
}

// nodeParent is a node's parent in the draft, or in the snapshot the draft
// began from for a node it removed.
func nodeParent(document *model.DocumentDraft, id model.NodeID) (model.NodeID, bool) {
	if node, ok := document.Node(id); ok {
		return node.Parent()
	}
	if node, ok := document.BaseSnapshot().Node(id); ok {
		return node.Parent()
	}
	return 0, false
}

func observerChangesSurvived(observer *LinkedRule, draft *EngineDraft) bool {
	changes := observer.Phase.changeKeys()
	if len(changes) == 0 {
		return true
	}
	for _, key := range changes {
		if draft.WasWritten(key) {
			return true
		}
	}
	return false
}

func eventTargetNode(event Event) (model.NodeID, bool) {
	if projected, ok := event.ProjectedEdit(); ok {
		return projected.Input().Target().Node(), true
	}
	if input, ok := event.TextInput(); ok {
		return input.Target().Node(), true
	}
	if input, ok := event.AttributeInput(); ok {
		return input.Target().Node(), true
	}
	if input, ok := event.SpanInput(); ok {
		return input.From().Node(), true
	}
	return 0, false
}

func validateEventInput(event Event, draft *EngineDraft) error {
	if projected, ok := event.ProjectedEdit(); ok {
		return projected.ValidateDraft(draft)
	}
	if text, ok := event.TextInput(); ok {
		return ValidateTextTargetDraft(text.Target(), draft)
	}
	if attribute, ok := event.AttributeInput(); ok {
		return ValidateAttributeTargetDraft(attribute.Target(), draft)
	}
	if span, ok := event.SpanInput(); ok {
		if err := ValidateTextTargetDraft(span.From(), draft); err != nil {
			return err
		}
		return ValidateTextTargetDraft(span.To(), draft)
	}
	return nil
}

func evaluateRuleCondition(
	rule *LinkedRule,
	event Event,
	resources *ResourceDraft,
) (bool, error) {
	if rule.Condition == nil {
		return true, nil
	}
	origin, ok := event.Origin()
	if !ok {
		origin = EventOriginTyping
	}
	specialized := rule.Condition.Specialize(event.Key(), origin)
	return specialized.Evaluate(event, resources.Read(rule.Effects))
}
