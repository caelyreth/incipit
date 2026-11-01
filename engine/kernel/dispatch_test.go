package kernel

import (
	"errors"
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/planning"
	"github.com/caelyreth/neverland/engine/model"
)

func TestPrecomputedEventPlansMatchLinearFallback(t *testing.T) {
	t.Parallel()
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "benchmark.planning",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{
				ID:        "exact",
				Phase:     HandleRulePhase(),
				Trigger:   NewExactTrigger("benchmark:exact"),
				Origins:   []EventOrigin{EventOriginTyping, EventOriginPaste},
				Order:     10,
				Algorithm: dispatchProgram(t, 1),
			},
			{
				ID:        "prefix",
				Phase:     HandleRulePhase(),
				Trigger:   NewPrefixTrigger("benchmark:"),
				Origins:   []EventOrigin{EventOriginTyping},
				Order:     20,
				Algorithm: dispatchProgram(t, 2),
			},
			{
				ID:        "after-exact",
				Phase:     AfterRulePhase(DocumentStateKey()),
				Trigger:   NewExactTrigger("benchmark:exact"),
				Origins:   []EventOrigin{EventOriginTyping},
				Order:     30,
				Effects:   NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
				Algorithm: dispatchProgram(t, 3),
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	program, err := LinkEventProgram(catalog)
	if err != nil {
		t.Fatal(err)
	}
	events := []Event{
		NewCommandEvent("benchmark:exact").WithOrigin(EventOriginTyping),
		NewCommandEvent("benchmark:exact").WithOrigin(EventOriginPaste),
		NewCommandEvent("benchmark:prefix-only").WithOrigin(EventOriginTyping),
	}
	for _, event := range events {
		gotHandlers, gotObservers := program.plan(event)
		wantHandlers, wantObservers := program.planLinear(
			event.Key(),
			normalizedEventOrigin(event),
		)
		if !slices.Equal(gotHandlers, wantHandlers) ||
			!slices.Equal(gotObservers, wantObservers) {
			t.Fatalf(
				"event %#v: handlers=%v/%v observers=%v/%v",
				event,
				gotHandlers,
				wantHandlers,
				gotObservers,
				wantObservers,
			)
		}
	}
}

func TestEventPlansMatchLinearAcrossTriggersAndOrigins(t *testing.T) {
	handlers := []*LinkedRule{
		{Key: RuleKey{ID: "exact"}, Trigger: NewExactTrigger("edit:exact"), Origins: []EventOrigin{EventOriginTyping, EventOriginPaste}},
		{Key: RuleKey{ID: "prefix"}, Trigger: NewPrefixTrigger("edit:")},
	}
	after := []*LinkedRule{
		{Key: RuleKey{ID: "wildcard"}, Trigger: NewStateWildcardTrigger()},
		{Key: RuleKey{ID: "exact-after"}, Trigger: NewExactTrigger("edit:exact"), Origins: []EventOrigin{EventOriginPaste}},
		{Key: RuleKey{ID: "prefix-after"}, Trigger: NewPrefixTrigger("edit:"), Origins: []EventOrigin{EventOriginTyping, EventOriginTyping}},
	}
	program := &EventProgram{handlers: handlers, after: after, eventPlans: linkEventPlans(handlers, after)}
	for _, key := range []string{"edit:exact", "edit:other", "unmatched"} {
		for origin := EventOrigin(0); origin <= EventOriginSemantic+1; origin++ {
			event := NewCommandEvent(key).WithOrigin(origin)
			gotHandlers, gotAfter := program.plan(event)
			var wantHandlers, wantAfter []*LinkedRule
			for _, rule := range handlers {
				if rule.Trigger.Matches(key) && rule.AllowsOrigin(normalizedEventOrigin(event)) {
					wantHandlers = append(wantHandlers, rule)
				}
			}
			for _, rule := range after {
				if rule.AllowsOrigin(normalizedEventOrigin(event)) &&
					(rule.Trigger.Kind() == TriggerStateWildcard || rule.Trigger.Matches(key)) {
					wantAfter = append(wantAfter, rule)
				}
			}
			if !slices.Equal(gotHandlers, wantHandlers) || !slices.Equal(gotAfter, wantAfter) {
				t.Fatalf("key=%q origin=%v: handlers %v/%v after %v/%v", key, origin, gotHandlers, wantHandlers, gotAfter, wantAfter)
			}
			linearHandlers, linearAfter := program.planLinear(key, normalizedEventOrigin(event))
			if !slices.Equal(linearHandlers, wantHandlers) || !slices.Equal(linearAfter, wantAfter) {
				t.Fatalf("linear key=%q origin=%v: handlers %v/%v after %v/%v", key, origin, linearHandlers, wantHandlers, linearAfter, wantAfter)
			}
		}
	}
}

func TestAfterRulesFollowTheirDeclaredOrder(t *testing.T) {
	t.Parallel()
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "ordered.after",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{
				ID:        "later",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewExactTrigger("ordered"),
				Order:     20,
				Effects:   NewEffectSet(nil, []ResourceID{"b"}),
				Algorithm: dispatchProgram(t, 2),
			},
			{
				ID:        "earlier",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewExactTrigger("ordered"),
				Order:     10,
				Effects:   NewEffectSet(nil, []ResourceID{"c"}),
				Algorithm: dispatchProgram(t, 1),
			},
		},
		Resources: []ResourceDescriptor{
			{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
			{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
			{ID: "c", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	program, err := LinkEventProgram(catalog)
	if err != nil {
		t.Fatal(err)
	}
	_, observers := program.plan(NewCommandEvent("ordered"))
	if len(observers) != 2 ||
		observers[0].Key.ID != "earlier" || observers[1].Key.ID != "later" {
		t.Fatalf("observers = %#v", observers)
	}
}

func TestExactAfterRulesRejectEqualOrderInterference(t *testing.T) {
	t.Parallel()
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "conflicting.after",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{
				ID:        "first",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewExactTrigger("ordered"),
				Order:     10,
				Effects:   NewEffectSet(nil, []ResourceID{"b"}),
				Algorithm: dispatchProgram(t, 1),
			},
			{
				ID:        "second",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewExactTrigger("ordered"),
				Order:     10,
				Effects:   NewEffectSet(nil, []ResourceID{"b"}),
				Algorithm: dispatchProgram(t, 2),
			},
		},
		Resources: []ResourceDescriptor{
			{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
			{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = LinkEventProgram(catalog)
	var dispatchErr *DispatchError
	if err == nil || !errors.As(err, &dispatchErr) ||
		dispatchErr.Kind != DispatchErrorAmbiguousOrder {
		t.Fatalf("error = %#v", err)
	}
}

func TestAfterRulesRejectCoSchedulableWildcardInterference(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		keyed Trigger
	}{
		{name: "exact", keyed: NewExactTrigger("ordered")},
		{name: "prefix", keyed: NewPrefixTrigger("order")},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
				ID:               "conflicting.mixed.after",
				EnabledByDefault: true,
				Rules: []RuleDescriptor{
					{
						ID:        "wildcard",
						Phase:     AfterRulePhase(ResourceStateKey("a")),
						Trigger:   NewStateWildcardTrigger(),
						Order:     10,
						Effects:   NewEffectSet(nil, []ResourceID{"b"}),
						Algorithm: dispatchProgram(t, 1),
					},
					{
						ID:        "keyed",
						Phase:     AfterRulePhase(ResourceStateKey("a")),
						Trigger:   test.keyed,
						Order:     10,
						Effects:   NewEffectSet(nil, []ResourceID{"b"}),
						Algorithm: dispatchProgram(t, 2),
					},
				},
				Resources: []ResourceDescriptor{
					{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
					{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = LinkEventProgram(catalog)
			var dispatchErr *DispatchError
			if err == nil || !errors.As(err, &dispatchErr) ||
				dispatchErr.Kind != DispatchErrorAmbiguousOrder {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestAfterRulesAllowEqualOrderWhenEffectsOrOriginsDoNotConflict(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		firstTrigger  Trigger
		secondTrigger Trigger
		firstWrite    ResourceID
		secondWrite   ResourceID
		firstOrigins  []EventOrigin
		secondOrigins []EventOrigin
	}{
		{
			name:          "non-interfering effects",
			firstTrigger:  NewStateWildcardTrigger(),
			secondTrigger: NewExactTrigger("ordered"),
			firstWrite:    "b",
			secondWrite:   "c",
		},
		{
			name:          "disjoint origins",
			firstTrigger:  NewExactTrigger("ordered"),
			secondTrigger: NewExactTrigger("ordered"),
			firstWrite:    "b",
			secondWrite:   "b",
			firstOrigins:  []EventOrigin{EventOriginTyping},
			secondOrigins: []EventOrigin{EventOriginPaste},
		},
		{
			name:          "disjoint keyed triggers",
			firstTrigger:  NewExactTrigger("alpha"),
			secondTrigger: NewPrefixTrigger("beta"),
			firstWrite:    "b",
			secondWrite:   "b",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
				ID:               "allowed.mixed.after",
				EnabledByDefault: true,
				Rules: []RuleDescriptor{
					{
						ID:        "first",
						Phase:     AfterRulePhase(ResourceStateKey("a")),
						Trigger:   test.firstTrigger,
						Origins:   test.firstOrigins,
						Order:     10,
						Effects:   NewEffectSet(nil, []ResourceID{test.firstWrite}),
						Algorithm: dispatchProgram(t, 1),
					},
					{
						ID:        "second",
						Phase:     AfterRulePhase(ResourceStateKey("a")),
						Trigger:   test.secondTrigger,
						Origins:   test.secondOrigins,
						Order:     10,
						Effects:   NewEffectSet(nil, []ResourceID{test.secondWrite}),
						Algorithm: dispatchProgram(t, 2),
					},
				},
				Resources: []ResourceDescriptor{
					{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
					{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
					{ID: "c", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LinkEventProgram(catalog); err != nil {
				t.Fatalf("non-conflicting after rules rejected: %v", err)
			}
		})
	}
}

func TestWildcardAndKeyedAfterRulesFollowDeclaredOrder(t *testing.T) {
	t.Parallel()
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "ordered.mixed.after",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{
				ID:        "wildcard",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewStateWildcardTrigger(),
				Order:     30,
				Effects:   NewEffectSet(nil, []ResourceID{"c"}),
				Algorithm: dispatchProgram(t, 3),
			},
			{
				ID:        "keyed",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewExactTrigger("ordered"),
				Order:     20,
				Effects:   NewEffectSet(nil, []ResourceID{"b"}),
				Algorithm: dispatchProgram(t, 2),
			},
		},
		Resources: []ResourceDescriptor{
			{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
			{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
			{ID: "c", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	program, err := LinkEventProgram(catalog)
	if err != nil {
		t.Fatal(err)
	}
	_, observers := program.plan(NewCommandEvent("ordered"))
	if len(observers) != 2 ||
		observers[0].Key.ID != "keyed" ||
		observers[1].Key.ID != "wildcard" {
		t.Fatalf("observers = %#v", observers)
	}
}

func TestInputScopeAllowsOnlyTheTargetGap(t *testing.T) {
	t.Parallel()
	document, err := model.NewDocument("document")
	if err != nil {
		t.Fatal(err)
	}
	root := document.Snapshot().Root()
	var blocks []model.NodeID
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		for range 3 {
			id, err := draft.CreateNode(root, len(blocks), "paragraph")
			if err != nil {
				return err
			}
			blocks = append(blocks, id)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	base := document.Snapshot()
	event := NewTextEvent(
		NewTextInput(
			NewTextTarget(blocks[0], model.EmptyUTF16Range(0), base.Revision()),
			"x",
		),
		EventOriginTyping,
	)
	scope, ok := makeInputScope(base, event)
	if !ok {
		t.Fatal("no input scope")
	}
	draft := document.BeginDraft()
	if !scope.allows(draft, blocks[1]) {
		t.Fatal("adjacent sibling rejected")
	}
	if scope.allows(draft, blocks[2]) {
		t.Fatal("distant sibling accepted")
	}
	inserted, err := draft.CreateNode(root, 1, "paragraph")
	if err != nil {
		t.Fatal(err)
	}
	if !scope.allows(draft, inserted) {
		t.Fatal("split block in the target gap rejected")
	}
	far, err := draft.CreateNode(root, 4, "paragraph")
	if err != nil {
		t.Fatal(err)
	}
	if scope.allows(draft, far) {
		t.Fatal("new block beyond the adjacent gap accepted")
	}
}

func dispatchProgram(t *testing.T, leaf uint32) *planning.ControlProgram {
	t.Helper()
	builder := &planning.ControlBuilder{}
	root := builder.Leaf(planning.NewLeafID(leaf))
	program, err := builder.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func dispatchCatalog(t *testing.T) PolicyCatalog {
	t.Helper()
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "test",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{
				ID:        "first",
				Phase:     HandleRulePhase(),
				Trigger:   NewExactTrigger("command"),
				Order:     10,
				Effects:   NewEffectSet(nil, []ResourceID{"a"}),
				Algorithm: dispatchProgram(t, 1),
			},
			{
				ID:        "fallback",
				Phase:     HandleRulePhase(),
				Trigger:   NewExactTrigger("command"),
				Order:     20,
				Effects:   NewEffectSet(nil, []ResourceID{"a"}),
				Algorithm: dispatchProgram(t, 2),
			},
			{
				ID:        "after",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewStateWildcardTrigger(),
				Order:     30,
				Effects:   NewEffectSet(nil, []ResourceID{"b"}),
				Algorithm: dispatchProgram(t, 3),
			},
		},
		Resources: []ResourceDescriptor{
			{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
			{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func dispatchEngine(t *testing.T) *Engine {
	t.Helper()
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(document, NewResourceStore([]ResourceDescriptor{
		{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
		{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
	}))
}

func TestDispatchStopsOnHandledAndRunsCausalObservers(t *testing.T) {
	t.Parallel()
	engine := dispatchEngine(t)
	program, err := LinkEventProgram(dispatchCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	calls := make([]LeafID, 0)
	outcome, err := program.DispatchEngine(
		NewCommandEvent("command"),
		&EngineDraft{
			document:     engine.Document().BeginDraft(),
			resources:    engine.Resources().BeginDraft(),
			baseRevision: engine.Revision(),
		},
		EngineInvokerFunc(func(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			calls = append(calls, leaf)
			if leaf == planning.NewLeafID(1) {
				if err := context.Resources().Set("a", BoolResource(true)); err != nil {
					return 0, err
				}
				return OutcomeHandled, nil
			}
			if leaf == planning.NewLeafID(3) {
				if err := context.Resources().Set("b", BoolResource(true)); err != nil {
					return 0, err
				}
				return OutcomeHandled, nil
			}
			return OutcomePass, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != OutcomeHandled || outcome.HandledRule == nil ||
		outcome.HandledRule.ID != "first" || len(calls) != 2 ||
		calls[0] != planning.NewLeafID(1) || calls[1] != planning.NewLeafID(3) {
		t.Fatalf("outcome=%#v calls=%#v", outcome, calls)
	}
}

func TestDispatchPassReturnsPassAndRollsBackCandidateWrites(t *testing.T) {
	t.Parallel()
	engine := dispatchEngine(t)
	program, err := LinkEventProgram(dispatchCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	draft := &EngineDraft{
		document:     engine.Document().BeginDraft(),
		resources:    engine.Resources().BeginDraft(),
		baseRevision: engine.Revision(),
	}
	outcome, err := program.DispatchEngine(
		NewCommandEvent("command"),
		draft,
		EngineInvokerFunc(func(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			if leaf == planning.NewLeafID(1) {
				if err := context.Resources().Set("a", BoolResource(true)); err != nil {
					return 0, err
				}
				return OutcomePass, nil
			}
			return OutcomeHandled, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != OutcomeHandled || outcome.HandledRule.ID != "fallback" {
		t.Fatalf("outcome=%#v", outcome)
	}
	value, err := draft.Resources().Read(NewEffectSet([]ResourceID{"a"}, nil)).Get("a")
	boolean, kindOK := value.Bool()
	if err != nil || !kindOK || boolean {
		t.Fatalf("passed candidate retained write: value=%#v kindOK=%v err=%v", value, kindOK, err)
	}
}

func TestDispatchInvokerErrorRollsBackEventEntryAndReturnsError(t *testing.T) {
	t.Parallel()
	engine := dispatchEngine(t)
	program, err := LinkEventProgram(dispatchCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	draft := &EngineDraft{
		document:     engine.Document().BeginDraft(),
		resources:    engine.Resources().BeginDraft(),
		baseRevision: engine.Revision(),
	}
	sentinel := errors.New("rejected")
	_, err = program.DispatchEngine(
		NewCommandEvent("command"),
		draft,
		EngineInvokerFunc(func(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			if err := context.Resources().Set("a", BoolResource(true)); err != nil {
				return 0, err
			}
			return 0, sentinel
		}),
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	if draft.Resources().WasWritten("a") {
		t.Fatal("event-entry rollback lost")
	}
}

func TestLinkRejectsAmbiguousRuleOrder(t *testing.T) {
	t.Parallel()
	descriptor := PolicyDescriptor{
		ID:               "bad",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{ID: "one", Trigger: NewExactTrigger("event"), Order: 1, Algorithm: dispatchProgram(t, 1)},
			{ID: "two", Trigger: NewExactTrigger("event"), Order: 1, Algorithm: dispatchProgram(t, 2)},
		},
	}
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{descriptor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkEventProgram(catalog); err == nil {
		t.Fatal("ambiguous order accepted")
	}
}

func TestHandlerAmbiguityKeepsTriggerOverlapSemantics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		first   Trigger
		second  Trigger
		wantErr bool
	}{
		{
			name:    "overlapping exact and prefix",
			first:   NewExactTrigger("command:run"),
			second:  NewPrefixTrigger("command:"),
			wantErr: true,
		},
		{
			name:   "disjoint exact and prefix",
			first:  NewExactTrigger("alpha"),
			second: NewPrefixTrigger("beta:"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
				ID:               "handlers",
				EnabledByDefault: true,
				Rules: []RuleDescriptor{
					{
						ID:        "first",
						Trigger:   test.first,
						Order:     1,
						Algorithm: dispatchProgram(t, 1),
					},
					{
						ID:        "second",
						Trigger:   test.second,
						Order:     1,
						Algorithm: dispatchProgram(t, 2),
					},
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = LinkEventProgram(catalog)
			if test.wantErr && err == nil {
				t.Fatal("overlapping handlers accepted")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("disjoint handlers rejected: %v", err)
			}
		})
	}
}

func TestLinkRejectsRulePlansWithoutExactlyOneLeaf(t *testing.T) {
	t.Parallel()
	builder := &planning.ControlBuilder{}
	first := builder.Leaf(planning.NewLeafID(0))
	second := builder.Leaf(planning.NewLeafID(1))
	multi, err := builder.Build(builder.Then(first, second))
	if err != nil {
		t.Fatal(err)
	}
	passBuilder := &planning.ControlBuilder{}
	pass, err := passBuilder.Build(passBuilder.Pass())
	if err != nil {
		t.Fatal(err)
	}
	for _, program := range []*planning.ControlProgram{multi, pass} {
		catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
			ID:               "bad",
			EnabledByDefault: true,
			Rules: []RuleDescriptor{{
				ID:        "rule",
				Trigger:   NewExactTrigger("event"),
				Order:     1,
				Algorithm: program,
			}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := LinkEventProgram(catalog); err == nil {
			t.Fatalf("rule with %d leaves accepted", len(program.Leaves()))
		}
	}
}
