package shell

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
)

// The query's conditions are a set (field-and-desk.md §4.5): what the
// field shows, the ribbons, the group the field is in and whether a query
// is kept already are worked out from which conditions it holds, never
// from the order they came in. Every way in puts a condition the same
// way (put): written as the language writes it, an alias as the key it
// stands for and a tag never as #; once; a key that holds one value with
// the new value in place of the old; and all of them in one order, by
// key, then by value.

// conditionOrder is the order conditions stand in, by key: what the query
// is in first, then the keys as the open pill offers them; any other key
// after, as the language lists them.
var conditionOrder = append([]string{"settings", "trash", "in"}, slices.DeleteFunc(slices.Clone(filterKeys), func(name string) bool {
	return name == "in"
})...)

// keyRank is where conditions of the key name stand among the others.
func keyRank(name string) int {
	if i := slices.Index(conditionOrder, name); i >= 0 {
		return i
	}
	return len(conditionOrder) + slices.IndexFunc(query.Keys, func(k query.Key) bool { return k.Name == name })
}

// written is c as the conditions hold it: an alias written as its key, a
// tag never as #.
func written(c query.Condition) query.Condition {
	if c.Key.Alias != "" {
		c.Key = mustKey(c.Key.Alias)
	}
	c.Hash = false
	return c
}

// sameCondition reports whether a and b say the same: of one key, turned
// alike, of one value.
func sameCondition(a, b query.Condition) bool {
	a, b = written(a), written(b)
	return a.Key.Name == b.Key.Name && a.Not == b.Not && query.Fold(a.Value) == query.Fold(b.Value)
}

// put is conds with c in it: in place of the key's condition where the key
// holds one value, not again where it is there, all in order. It reports
// whether conds changed.
func put(conds []string, c query.Condition) ([]string, bool) {
	c = written(c)
	var old []string
	for _, text := range conds {
		had, ok := condition(text)
		if ok && sameCondition(had, c) {
			return conds, false
		}
		if ok && c.Key.OneValue() && written(had).Key.Name == c.Key.Name {
			continue
		}
		old = append(old, text)
	}
	return ordered(append(old, c.String())), true
}

// ordered is conds in order: by key, then by value.
func ordered(conds []string) []string {
	slices.SortStableFunc(conds, func(a, b string) int {
		ca, okA := condition(a)
		cb, okB := condition(b)
		if !okA || !okB {
			return 0
		}
		if d := keyRank(ca.Key.Name) - keyRank(cb.Key.Name); d != 0 {
			return d
		}
		return strings.Compare(query.Fold(ca.Value), query.Fold(cb.Value))
	})
	return conds
}

// sameQuery reports whether a and b are the one query: the same
// conditions, in whatever order, and the same words.
func sameQuery(a, b string) bool {
	return canonicalQuery(a) == canonicalQuery(b)
}

// canonicalQuery is text as the field holds it: its conditions put in
// order, then its words.
func canonicalQuery(text string) string {
	conds, words := splitQuery(text)
	var out []string
	for _, cond := range conds {
		if c, ok := condition(cond); ok {
			out, _ = put(out, c)
		}
	}
	return strings.TrimSpace(strings.Join(append(out, strings.TrimSpace(words)), " "))
}
