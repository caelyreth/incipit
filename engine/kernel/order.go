package kernel

import (
	"slices"
	"strings"
)

// ruleOrder ranks linked rules by the relations they declare: each rule
// comes after every rule it names in After, and every rule naming it in
// Before. Rules unrelated either way keep their identity order, which only
// rules that can never meet may rely on (checkAmbiguity).
type ruleOrder struct {
	rank  map[RuleKey]int
	reach map[RuleKey]map[RuleKey]bool
}

func orderRules(rules []*LinkedRule, declared map[RuleKey]RuleDescriptor) (ruleOrder, error) {
	present := make(map[RuleKey]bool, len(rules))
	for _, rule := range rules {
		present[rule.Key] = true
	}
	next := make(map[RuleKey][]RuleKey, len(rules))
	incoming := make(map[RuleKey]int, len(rules))
	edge := func(from, to RuleKey) {
		if !present[from] || !present[to] || from == to || slices.Contains(next[from], to) {
			return
		}
		next[from] = append(next[from], to)
		incoming[to]++
	}
	// A relation naming no policy names a rule of the declaring one.
	own := func(rule, other RuleKey) RuleKey {
		if other.Owner == "" {
			other.Owner = rule.Owner
		}
		return other
	}
	for _, rule := range rules {
		descriptor := declared[rule.Key]
		for _, first := range descriptor.After {
			edge(own(rule.Key, first), rule.Key)
		}
		for _, later := range descriptor.Before {
			edge(rule.Key, own(rule.Key, later))
		}
	}
	keys := make([]RuleKey, 0, len(rules))
	for _, rule := range rules {
		keys = append(keys, rule.Key)
	}
	slices.SortFunc(keys, compareRuleKeys)
	ready := make([]RuleKey, 0, len(keys))
	for _, key := range keys {
		if incoming[key] == 0 {
			ready = append(ready, key)
		}
	}
	order := ruleOrder{rank: make(map[RuleKey]int, len(keys)), reach: make(map[RuleKey]map[RuleKey]bool, len(keys))}
	for len(ready) > 0 {
		key := ready[0]
		ready = ready[1:]
		order.rank[key] = len(order.rank)
		for _, to := range next[key] {
			incoming[to]--
			if incoming[to] == 0 {
				at, _ := slices.BinarySearchFunc(ready, to, compareRuleKeys)
				ready = slices.Insert(ready, at, to)
			}
		}
	}
	if len(order.rank) != len(keys) {
		for _, key := range keys {
			if _, ranked := order.rank[key]; !ranked {
				return ruleOrder{}, &DispatchError{Kind: DispatchErrorCyclicOrder, Rule: key}
			}
		}
	}
	for _, key := range keys {
		seen := make(map[RuleKey]bool)
		pending := append([]RuleKey(nil), next[key]...)
		for len(pending) > 0 {
			at := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[at] {
				continue
			}
			seen[at] = true
			pending = append(pending, next[at]...)
		}
		order.reach[key] = seen
	}
	for _, rule := range rules {
		rule.rank = order.rank[rule.Key]
	}
	return order, nil
}

// related reports whether one of two rules is declared, directly or
// through others, to go first.
func (o ruleOrder) related(first, second RuleKey) bool {
	return o.reach[first][second] || o.reach[second][first]
}

func compareRuleKeys(a, b RuleKey) int {
	if c := strings.Compare(string(a.Owner), string(b.Owner)); c != 0 {
		return c
	}
	return strings.Compare(string(a.ID), string(b.ID))
}

// scoped orders the rules matching an event by where they answer: rules of
// the event's innermost scope first, outward to the page's. A rule whose
// scope the event does not stand in does not answer it.
func scoped(rules []*LinkedRule, scopes []ScopeKind) []*LinkedRule {
	plain := true
	for _, rule := range rules {
		if rule.Scope != PageScope {
			plain = false
			break
		}
	}
	if plain {
		return rules
	}
	ordered := make([]*LinkedRule, 0, len(rules))
	for _, scope := range append(slices.Clone(scopes), PageScope) {
		for _, rule := range rules {
			if rule.Scope == scope && !slices.Contains(ordered, rule) {
				ordered = append(ordered, rule)
			}
		}
	}
	return ordered
}
