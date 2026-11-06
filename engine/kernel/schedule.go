package kernel

import "sort"

// ObserverProgram is the validated after-phase schedule. Watchers map each
// observable state key to the observer indices that depend on it.
type ObserverProgram struct {
	rules    []*LinkedRule
	watchers map[StateKey][]int
}

// linkObserverProgram validates after-phase rules against every rule in the
// linked catalog. A state observer cannot depend on state written by a rule
// that is not declared to go before it. After rules neither declared first
// cannot interfere when they can run for the same event and origin.
func linkObserverProgram(rules []*LinkedRule, order ruleOrder) (*ObserverProgram, error) {
	all := append([]*LinkedRule(nil), rules...)
	sortRules(all)
	after := make([]*LinkedRule, 0)
	observers := make([]*LinkedRule, 0)
	for _, rule := range all {
		if rule.Phase.Kind == RulePhaseKindAfter {
			after = append(after, rule)
		}
		if rule.Trigger.Kind() == TriggerStateWildcard {
			observers = append(observers, rule)
		}
	}
	for _, observer := range all {
		if observer.Trigger.Kind() != TriggerStateWildcard {
			continue
		}
		for _, producer := range all {
			if producer.Key == observer.Key || order.reach[producer.Key][observer.Key] {
				continue
			}
			for _, key := range observer.Phase.ChangeKeys() {
				if producer.Effects.CanWriteState(key) {
					return nil, &DispatchError{
						Kind:   DispatchErrorBackwardObservation,
						First:  producer.Key,
						Second: observer.Key,
					}
				}
			}
		}
	}
	if err := validateAfterRuleOrder(after, order); err != nil {
		return nil, err
	}
	watchers := make(map[StateKey][]int)
	for index, observer := range observers {
		for _, key := range observer.Phase.ChangeKeys() {
			watchers[key] = append(watchers[key], index)
		}
	}
	return &ObserverProgram{rules: observers, watchers: watchers}, nil
}

// afterTriggersCoSchedule reports whether both trigger forms can select an
// after rule for one event. A state wildcard observes every event; keyed
// triggers still use their normal overlap semantics.
func afterTriggersCoSchedule(first, second Trigger) bool {
	if first.Kind() == TriggerStateWildcard || second.Kind() == TriggerStateWildcard {
		return true
	}
	return first.Overlaps(second)
}

func validateAfterRuleOrder(rules []*LinkedRule, order ruleOrder) error {
	for index, first := range rules {
		for _, second := range rules[index+1:] {
			if !order.related(first.Key, second.Key) &&
				afterTriggersCoSchedule(first.Trigger, second.Trigger) &&
				OriginsOverlap(first.Origins, second.Origins) &&
				first.Effects.Interferes(second.Effects) {
				return &DispatchError{
					Kind:   DispatchErrorAmbiguousOrder,
					First:  first.Key,
					Second: second.Key,
				}
			}
		}
	}
	return nil
}

// Rules returns immutable observer declarations in deterministic order.
func (p *ObserverProgram) Rules() []*LinkedRule {
	if p == nil {
		return nil
	}
	return append([]*LinkedRule(nil), p.rules...)
}

// Reachable returns observers reachable from event writes after applying
// origin, trigger, and condition-free declaration filters.
func (p *ObserverProgram) Reachable(
	possibleWrites []StateKey,
	origin EventOrigin,
	matches func(*LinkedRule) bool,
) []*LinkedRule {
	if p == nil {
		return nil
	}
	reachable := make(map[StateKey]struct{}, len(possibleWrites))
	pending := make([]StateKey, 0, len(possibleWrites))
	for _, key := range possibleWrites {
		if _, seen := reachable[key]; seen {
			continue
		}
		reachable[key] = struct{}{}
		pending = append(pending, key)
	}
	selected := make(map[int]struct{})
	for len(pending) > 0 {
		key := pending[0]
		pending = pending[1:]
		for _, index := range p.watchers[key] {
			if _, seen := selected[index]; seen {
				continue
			}
			observer := p.rules[index]
			if !observer.AllowsOrigin(origin) || (matches != nil && !matches(observer)) {
				continue
			}
			selected[index] = struct{}{}
			for _, output := range observer.Effects.StateWrites() {
				if _, seen := reachable[output]; seen {
					continue
				}
				reachable[output] = struct{}{}
				pending = append(pending, output)
			}
		}
	}
	indices := make([]int, 0, len(selected))
	for index := range selected {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	result := make([]*LinkedRule, 0, len(indices))
	for _, index := range indices {
		result = append(result, p.rules[index])
	}
	return result
}
