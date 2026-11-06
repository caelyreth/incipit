package kernel

import "testing"

func scheduleRule(
	t *testing.T,
	id RuleID,
	phase RulePhase,
	effects EffectSet,
	origins ...EventOrigin,
) *LinkedRule {
	t.Helper()
	program := runtimeLeaf(t, 1)
	trigger := NewStateWildcardTrigger()
	if phase.Kind == RulePhaseKindHandle {
		trigger = NewExactTrigger("cmd:test")
	}
	rule, err := linkRule("policy", RuleDescriptor{
		ID:        id,
		Phase:     phase,
		Trigger:   trigger,
		Origins:   origins,
		Effects:   effects,
		Algorithm: program,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

func TestObserverProgramRejectsBackwardWritingDependencies(t *testing.T) {
	t.Parallel()
	producer := scheduleRule(
		t,
		"b",
		HandleRulePhase(),
		NewEffectSet(nil, []ResourceID{"a"}),
	)
	observer := scheduleRule(
		t,
		"a",
		AfterRulePhase(ResourceStateKey("a")),
		NewEffectSet([]ResourceID{"a"}, nil),
	)
	_, err := scheduleProgram([]*LinkedRule{producer, observer}, map[RuleID][]RuleID{"b": {"a"}})
	var dispatchErr *DispatchError
	if err == nil || !asDispatchError(err, &dispatchErr) ||
		dispatchErr.Kind != DispatchErrorBackwardObservation {
		t.Fatalf("error = %#v", err)
	}
}

func TestExactAfterRuleIsNotAStaticObserver(t *testing.T) {
	t.Parallel()
	program := runtimeLeaf(t, 1)
	rule, err := linkRule("policy", RuleDescriptor{
		ID:        "typing-refine",
		Phase:     AfterRulePhase(DocumentStateKey()),
		Trigger:   NewExactTrigger(TextInputKey),
		Origins:   []EventOrigin{EventOriginTyping},
		Effects:   NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
		Algorithm: program,
	})
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := scheduleProgram([]*LinkedRule{rule}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(schedule.Rules()) != 0 {
		t.Fatalf("exact after rule entered static observers: %#v", schedule.Rules())
	}
}

func TestObserverProgramAllowsDependenciesOnEarlierProducers(t *testing.T) {
	t.Parallel()
	producer := scheduleRule(
		t,
		"producer",
		HandleRulePhase(),
		NewEffectSet(nil, []ResourceID{"a"}),
	)
	observer := scheduleRule(
		t,
		"observer",
		AfterRulePhase(ResourceStateKey("a")),
		NewEffectSet([]ResourceID{"a"}, []ResourceID{"b"}),
	)
	program, err := scheduleProgram([]*LinkedRule{producer, observer}, map[RuleID][]RuleID{"observer": {"producer"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Rules()) != 1 || program.Rules()[0].Key.ID != "observer" {
		t.Fatalf("rules = %#v", program.Rules())
	}
}

func TestObserverProgramRejectsEqualOrderInterference(t *testing.T) {
	t.Parallel()
	first := scheduleRule(
		t,
		"first",
		AfterRulePhase(ResourceStateKey("a")),
		NewEffectSet([]ResourceID{"a"}, []ResourceID{"b"}),
	)
	second := scheduleRule(
		t,
		"second",
		AfterRulePhase(ResourceStateKey("b")),
		NewEffectSet([]ResourceID{"b"}, []ResourceID{"a"}),
	)
	_, err := scheduleProgram([]*LinkedRule{first, second}, nil)
	var dispatchErr *DispatchError
	if err == nil || !asDispatchError(err, &dispatchErr) ||
		dispatchErr.Kind != DispatchErrorBackwardObservation {
		t.Fatalf("error = %#v", err)
	}
}

func TestObserverProgramReachabilityIsTransitiveAndDeterministic(t *testing.T) {
	t.Parallel()
	first := scheduleRule(
		t,
		"first",
		AfterRulePhase(ResourceStateKey("a")),
		NewEffectSet([]ResourceID{"a"}, []ResourceID{"b"}),
	)
	second := scheduleRule(
		t,
		"second",
		AfterRulePhase(ResourceStateKey("b")),
		NewEffectSet([]ResourceID{"b"}, []ResourceID{"c"}),
	)
	program, err := scheduleProgram([]*LinkedRule{second, first}, map[RuleID][]RuleID{"second": {"first"}})
	if err != nil {
		t.Fatal(err)
	}
	reachable := program.Reachable(
		[]StateKey{ResourceStateKey("a")},
		EventOriginTyping,
		nil,
	)
	if len(reachable) != 2 ||
		reachable[0].Key.ID != "first" ||
		reachable[1].Key.ID != "second" {
		t.Fatalf("reachable = %#v", reachable)
	}
	if filtered := program.Reachable(
		[]StateKey{ResourceStateKey("a")},
		EventOriginTyping,
		func(rule *LinkedRule) bool { return rule.Key.ID != "first" },
	); len(filtered) != 0 {
		t.Fatalf("blocked path = %#v", filtered)
	}
}

func asDispatchError(err error, target **DispatchError) bool {
	value, ok := err.(*DispatchError)
	if ok {
		*target = value
	}
	return ok
}

// scheduleProgram links rules of one policy as after names each to follow.
func scheduleProgram(rules []*LinkedRule, after map[RuleID][]RuleID) (*ObserverProgram, error) {
	declared := make(map[RuleKey]RuleDescriptor, len(rules))
	for _, rule := range rules {
		descriptor := RuleDescriptor{ID: rule.Key.ID}
		for _, first := range after[rule.Key.ID] {
			descriptor.After = append(descriptor.After, RuleKey{Owner: rule.Key.Owner, ID: first})
		}
		declared[rule.Key] = descriptor
	}
	order, err := orderRules(rules, declared)
	if err != nil {
		return nil, err
	}
	return linkObserverProgram(rules, order)
}
