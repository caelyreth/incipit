package shell

import (
	"slices"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/locales"
)

// What may be written next stands around the pill as idle ribbons
// (field-and-desk.md §4.5, pill.go), and nothing of the query's language
// is ever written in the pill for it. A key pressed opens in the pill:
// its values stand around it, each with how many sheets it would find,
// what is written narrows them, and the arrows, Tab, Return or a press
// take one, which becomes a ribbon. What is written, where the key takes
// it, stands among them as a value of its own: after those it starts,
// alone where none does, as words for a title or 7d for a date. A key
// that holds one value, as a date, a length or the order, takes the new
// one in place of the one it held; others, as tags, gather them. What
// was written before the key opened comes back as it closes. A key
// written by hand ($up…) marks the keys it may start, and taken, opens.
// What the field shows answers the query as it was last whole: without
// what is half written, or what narrows an open key's values.

// offer works out what may stand at caret in the words, reporting whether
// what stands in the field for it changed.
func (s *State) offer(caret int) bool {
	words := s.field.words
	changed := false
	if s.field.dismissed != "" && s.field.dismissed != words {
		s.field.dismissed = ""
		changed = true
	}
	// An open key's values are what its key would take written before
	// the words.
	prefix := s.openedPrefix()
	if prefix+words != s.field.completedFor || caret != s.field.caret {
		var names func(string) []string
		if s.Library != nil {
			names = s.Library.SearchNames
		}
		next := query.Complete(prefix+words, len(prefix)+caret, query.InNotebook, names)
		next.Span.From, next.Span.To = max(0, next.Span.From-len(prefix)), max(0, next.Span.To-len(prefix))
		if k := s.field.opened; k != nil {
			next.Items = withWritten(next.Items, k, words)
		}
		if !slices.Equal(next.Items, s.field.completion.Items) || next.Span != s.field.completion.Span {
			changed = true
		}
		s.field.completion, s.field.caret, s.field.completedFor = next, caret, prefix+words
	}
	if changed {
		// Nothing is chosen until the keys go to it, and the values come
		// again no more than idleMost at first.
		s.field.suggestion = -1
		s.field.allValues = false
	}
	return changed
}

// withWritten is items, the values offered for k, and what is written,
// where k takes it and none of them is it, after them.
func withWritten(items []query.Suggestion, k *query.Key, words string) []query.Suggestion {
	v := strings.TrimSpace(words)
	if v == "" || !k.Accepts(v) || slices.ContainsFunc(items, func(item query.Suggestion) bool {
		return query.Fold(rawValue(item)) == query.Fold(v)
	}) {
		return items
	}
	return append(items, query.Suggestion{Label: v, Text: v, Of: k.Name, Done: true, Written: true,
		Help: locales.TakeWhatIsWritten()})
}

// offered reports whether suggestions stand around the pill: while it is
// open, and they were not put away.
func (s *State) offered() bool {
	put := s.field.dismissed != "" && s.field.dismissed == s.field.words
	return s.field.open && len(s.field.completion.Items) > 0 && !put
}

// openedPrefix is what the open key would be written as, before its value.
func (s *State) openedPrefix() string {
	if k := s.field.opened; k != nil {
		return "$" + k.Name + ":"
	}
	return ""
}

// completing is the key whose values are offered: the open key, while the
// pill is open; nil while keys are.
func (s *State) completing() *query.Key {
	if s.field.opened != nil {
		if s.field.open {
			return s.field.opened
		}
		return nil
	}
	items := s.field.completion.Items
	if !s.offered() || items[0].Of == "" {
		return nil
	}
	return mustKey(items[0].Of)
}

// suggested is the suggestion Tab and Return take: the one the keys are
// on, or the first.
func (s *State) suggested() int {
	return max(0, min(s.field.suggestion, len(s.field.completion.Items)-1))
}

// stepSuggestion moves the keys along the suggestions by by, from none to
// the first going on, or to the last going back.
func (s *State) stepSuggestion(by int) {
	n := len(s.field.completion.Items)
	if s.field.suggestion < 0 {
		s.field.suggestion = map[bool]int{true: 0, false: n - 1}[by > 0]
		return
	}
	s.field.suggestion = (s.field.suggestion + by + n) % n
}

// wholeQuery is the query the field answers: its conditions and words,
// without a filter still being written, whether its suggestions stand or
// not, so the field never answers half a condition. All the field works
// out of its query, it works out of this one.
func (s *State) wholeQuery() string {
	words := s.field.held
	if s.field.opened == nil {
		words = s.unwritten()
	}
	return strings.TrimSpace(strings.Join(append(slices.Clone(s.field.conditions), words), " "))
}

// countedWith is the query the values of k are counted against: the
// query as it is whole, without k's own condition where k holds one value,
// as the value counted would take its place.
func (s *State) countedWith(k *query.Key) string {
	conds := slices.Clone(s.field.conditions)
	if k.OneValue() {
		conds = slices.DeleteFunc(conds, func(text string) bool { return conditionKey(text) == k.Name })
	}
	words := s.field.held
	if s.field.opened == nil {
		words = s.unwritten()
	}
	return strings.TrimSpace(strings.Join(append(conds, words), " "))
}

// unwritten is the words without the filter the caret is in, unless it
// is a key that stands alone, named whole: nothing more is written of it.
func (s *State) unwritten() string {
	words, caret := s.field.words, s.field.caret
	query.Walk(query.Parse(words).Root, func(n query.Node) {
		f, ok := n.(*query.Filter)
		if !ok || f.Key != nil && f.Key.Bare() && !f.HasValue {
			return
		}
		if f.From < caret && caret <= f.To && f.To <= len(words) {
			words = words[:f.From] + words[f.To:]
		}
	})
	return words
}

// takeSuggestion takes the suggestion at i: a key opens (takeKey); a
// value is a condition, put in the query or, taken already, taken away
// again (toggleValue).
func (s *State) takeSuggestion(i string) {
	at, err := strconv.Atoi(i)
	items := s.field.completion.Items
	if err != nil || at < 0 || at >= len(items) {
		return
	}
	if k := mustKey(items[at].Of); k != nil {
		s.toggleValue(k, rawValue(items[at]))
		return
	}
	s.takeKey(keyOf(items[at]))
}

// takeOpened takes what the open key holds: the value the keys are on, or
// what is written, as the first value that starts it or, where none does,
// as it is.
func (s *State) takeOpened() {
	k, words := s.field.opened, strings.TrimSpace(s.field.words)
	switch {
	case s.offered() && (s.field.suggestion >= 0 || words != ""):
		s.takeSuggestion(strconv.Itoa(s.suggested()))
	case words != "":
		s.toggleValue(k, words)
	}
}

// rawValue is a value's suggestion's value as meant.
func rawValue(item query.Suggestion) string {
	return strings.TrimPrefix(item.Label, "#")
}

// takenAt is where the query has k's condition of value v, or -1.
func (s *State) takenAt(k *query.Key, v string) int {
	for i, text := range s.field.conditions {
		if c, ok := condition(text); ok && c.Key.Name == k.Name && !c.Not && query.Fold(c.Value) == query.Fold(v) {
			return i
		}
	}
	return -1
}

// toggleValue puts k's condition of v in the query (put), or takes it away
// where it is there. A key that takes names, as the tags, stays open for
// more of them to be taken or let go; any other closes.
func (s *State) toggleValue(k *query.Key, v string) {
	if i := s.takenAt(k, v); i >= 0 {
		s.field.conditions = slices.Delete(s.field.conditions, i, i+1)
		s.field.next = change{depth: stageUp}
	} else {
		s.field.conditions, _ = put(s.field.conditions, query.Condition{Key: k, Value: v})
		s.field.next = change{depth: stageDown}
	}
	s.field.chosen = ""
	s.field.focus = true
	if k.Takes == query.TakesName && s.field.opened == k {
		s.setWords("")
		return
	}
	s.closeKey()
}

// takeKey opens the key named name in the pill, in place of a key open or
// being written, for its value to be chosen; pressed open, it closes; one
// that stands alone is taken at once.
func (s *State) takeKey(name string) {
	k := mustKey(name)
	if k == nil {
		return
	}
	s.field.chosen = ""
	s.field.focus = true
	switch {
	case k.Bare():
		s.addCondition((&query.Condition{Key: k}).String())
	case s.field.opened == k:
		s.closeKey()
	default:
		if s.field.opened == nil {
			s.field.held = strings.TrimSpace(s.unwritten())
		}
		s.field.opened = k
		s.field.acting = false
		s.setWords("")
	}
}

// settleQuery lets go of what was under way in the pill as it closes: an
// open key closes, what was put away comes back, and nothing stays chosen
// or shown in full, so the pill opens again as it first did.
func (s *State) settleQuery() {
	s.closeKey()
	s.field.dismissed = ""
	s.field.suggestion = -1
	s.field.allValues = false
	s.field.allKeys = false
}

// closeKey closes the open key, what was written before it coming back.
func (s *State) closeKey() {
	if s.field.opened == nil {
		return
	}
	held := s.field.held
	s.field.opened, s.field.held = nil, ""
	s.setWords(held)
}

// addCondition puts cond in the query, unless it is there already.
func (s *State) addCondition(cond string) {
	c, ok := condition(cond)
	if !ok {
		return
	}
	var changed bool
	if s.field.conditions, changed = put(s.field.conditions, c); !changed {
		return
	}
	s.field.next = change{depth: stageDown}
	s.field.chosen = ""
	s.field.focus = true
}

// turn turns the condition at i to its opposite, where it has one.
func (s *State) turn(i int) {
	cond, ok := condition(s.field.conditions[i])
	if !ok || cond.Key.Setting {
		return
	}
	cond.Not = !cond.Not
	s.field.conditions = slices.Delete(s.field.conditions, i, i+1)
	s.field.conditions, _ = put(s.field.conditions, cond)
	s.field.chosen = ""
}

// rewrite opens the key of the condition at i, for its value to be
// changed: where the key offers its values, among them, the condition's
// ticked; where its value is written, as words for a title, the condition
// taken back into the pill to be written again.
func (s *State) rewrite(i int) {
	cond, ok := condition(s.field.conditions[i])
	if !ok || cond.Key.Takes == query.TakesNothing {
		return
	}
	k := mustKey(cond.Key.Name)
	free := cond.Key.Takes == query.TakesText || cond.Key.Takes == query.TakesNumber
	if free {
		s.field.conditions = slices.Delete(s.field.conditions, i, i+1)
	}
	if s.field.opened != k {
		s.takeKey(k.Name)
	}
	if free {
		s.setWords(cond.Value)
	}
}
