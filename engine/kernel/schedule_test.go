package kernel

import "testing"

func scheduleRule(
	t *testing.T,
	id RuleID,
	phase RulePhase,
	order uint32,
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
		Order:     order,
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
		"producer",
		HandleRulePhase(),
		20,
		NewEffectSet(nil, []ResourceID{"a"}),
	)
	observer := scheduleRule(
		t,
		"observer",
		AfterRulePhase(ResourceStateKey("a")),
		10,
		NewEffectSet([]ResourceID{"a"}, nil),
	)
	_, err := LinkObserverProgram([]*LinkedRule{producer, observer})
	var dispatchErr *DispatchError
	if err == nil || !asDispatchError(err, &dispatchErr) ||
		dispatchErr.Kind != DispatchErrorBackwardObservation {
		t.Fatalf("error = %#v", err)
	}
}

func TestObserverProgramAllowsDependenciesOnEarlierProducers(t *testing.T) {
	t.Parallel()
	producer := scheduleRule(
		t,
		"producer",
		HandleRulePhase(),
		10,
		NewEffectSet(nil, []ResourceID{"a"}),
	)
	observer := scheduleRule(
		t,
		"observer",
		AfterRulePhase(ResourceStateKey("a")),
		20,
		NewEffectSet([]ResourceID{"a"}, []ResourceID{"b"}),
	)
	program, err := LinkObserverProgram([]*LinkedRule{producer, observer})
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
		10,
		NewEffectSet([]ResourceID{"a"}, []ResourceID{"b"}),
	)
	second := scheduleRule(
		t,
		"second",
		AfterRulePhase(ResourceStateKey("b")),
		10,
		NewEffectSet([]ResourceID{"b"}, []ResourceID{"a"}),
	)
	_, err := LinkObserverProgram([]*LinkedRule{first, second})
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
		10,
		NewEffectSet([]ResourceID{"a"}, []ResourceID{"b"}),
	)
	second := scheduleRule(
		t,
		"second",
		AfterRulePhase(ResourceStateKey("b")),
		20,
		NewEffectSet([]ResourceID{"b"}, []ResourceID{"c"}),
	)
	program, err := LinkObserverProgram([]*LinkedRule{second, first})
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
