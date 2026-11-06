package kernel

import (
	"errors"
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
	// A relation naming no policy names a rule of the declaring one.
	own := func(rule RuleKey, others []RuleKey) []RuleKey {
		named := make([]RuleKey, len(others))
		for i, other := range others {
			if other.Owner == "" {
				other.Owner = rule.Owner
			}
			named[i] = other
		}
		return named
	}
	keys := make([]RuleKey, 0, len(rules))
	for _, rule := range rules {
		keys = append(keys, rule.Key)
	}
	ranked, cyclic, err := Order(keys, compareRuleKeys,
		func(key RuleKey) []RuleKey { return own(key, declared[key].After) },
		func(key RuleKey) []RuleKey { return own(key, declared[key].Before) })
	if err != nil {
		return ruleOrder{}, &DispatchError{Kind: DispatchErrorCyclicOrder, Rule: cyclic}
	}
	for _, rule := range rules {
		rule.rank = ranked.Rank[rule.Key]
	}
	return ruleOrder{rank: ranked.Rank, reach: ranked.Reach}, nil
}

// ErrCyclicOrder reports relations that put a key, through others, after
// itself.
var ErrCyclicOrder = errors.New("kernel: cyclic order")

// Ranking is an order of keys: each key's place, and the keys declared,
// directly or through others, to come after it.
type Ranking[K comparable] struct {
	Rank  map[K]int
	Reach map[K]map[K]bool
}

// Related reports whether one of two keys is declared, directly or through
// others, to go first.
func (r Ranking[K]) Related(first, second K) bool {
	return r.Reach[first][second] || r.Reach[second][first]
}

// Order ranks keys by the relations they declare: each key comes after
// every key its after names, and before every key its before names.
// Relations naming a key not among keys hold nothing. Keys unrelated either
// way keep compare's order. A cycle is ErrCyclicOrder, with a key on it.
func Order[K comparable](keys []K, compare func(a, b K) int, after, before func(K) []K) (Ranking[K], K, error) {
	present := make(map[K]bool, len(keys))
	for _, key := range keys {
		present[key] = true
	}
	next := make(map[K][]K, len(keys))
	incoming := make(map[K]int, len(keys))
	edge := func(from, to K) {
		if !present[from] || !present[to] || from == to || slices.Contains(next[from], to) {
			return
		}
		next[from] = append(next[from], to)
		incoming[to]++
	}
	for _, key := range keys {
		for _, first := range after(key) {
			edge(first, key)
		}
		for _, later := range before(key) {
			edge(key, later)
		}
	}
	sorted := slices.Clone(keys)
	slices.SortFunc(sorted, compare)
	ready := make([]K, 0, len(sorted))
	for _, key := range sorted {
		if incoming[key] == 0 {
			ready = append(ready, key)
		}
	}
	order := Ranking[K]{Rank: make(map[K]int, len(sorted)), Reach: make(map[K]map[K]bool, len(sorted))}
	for len(ready) > 0 {
		key := ready[0]
		ready = ready[1:]
		order.Rank[key] = len(order.Rank)
		for _, to := range next[key] {
			incoming[to]--
			if incoming[to] == 0 {
				at, _ := slices.BinarySearchFunc(ready, to, compare)
				ready = slices.Insert(ready, at, to)
			}
		}
	}
	if len(order.Rank) != len(sorted) {
		for _, key := range sorted {
			if _, ranked := order.Rank[key]; !ranked {
				return Ranking[K]{}, key, ErrCyclicOrder
			}
		}
	}
	for _, key := range sorted {
		seen := make(map[K]bool)
		pending := slices.Clone(next[key])
		for len(pending) > 0 {
			at := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[at] {
				continue
			}
			seen[at] = true
			pending = append(pending, next[at]...)
		}
		order.Reach[key] = seen
	}
	var none K
	return order, none, nil
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
