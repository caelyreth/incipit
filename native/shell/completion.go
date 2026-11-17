package shell

import (
	"slices"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/stage"
)

// What may be written next stands around the pill as idle ribbons
// (field-and-desk.md §4.5, pill.go): while the caret is in a key or a
// value, what may stand there, the keys that start it or the values of
// its key, taken with Tab, Return or a press, the arrows going along
// them; a value taken becomes a ribbon. Otherwise, while the pill is
// open, each key the query can be narrowed by, pressed to be written
// here. What the field shows answers the query as it was last whole,
// without what is half written.

// offer works out what may stand at caret in the words, reporting whether
// what stands in the field for it changed.
func (s *State) offer(caret int) bool {
	words := s.field.words
	changed := false
	if s.field.dismissed != "" && s.field.dismissed != words {
		s.field.dismissed = ""
		changed = true
	}
	if words != s.field.completedFor || caret != s.field.caret {
		var names func(string) []string
		if s.Library != nil {
			names = s.Library.SearchNames
		}
		next := query.Complete(words, caret, query.InNotebook, names)
		if !slices.Equal(next.Items, s.field.completion.Items) || next.Span != s.field.completion.Span {
			changed = true
		}
		s.field.completion, s.field.caret, s.field.completedFor = next, caret, words
	}
	if changed {
		s.field.offers++
		s.field.suggestion = 0
	}
	return changed
}

// offered reports whether suggestions stand around the pill: while it is
// open, and they were not put away.
func (s *State) offered() bool {
	return s.field.open && len(s.field.completion.Items) > 0 && s.field.dismissed != s.field.words
}

// completing is the key whose values are suggested, nil while keys are.
func (s *State) completing() *query.Key {
	items := s.field.completion.Items
	if !s.offered() || items[0].Of == "" {
		return nil
	}
	return mustKey(items[0].Of)
}

// suggested is the suggestion the keys are on, which Tab and Return take.
func (s *State) suggested() int {
	return max(0, min(s.field.suggestion, len(s.field.completion.Items)-1))
}

// wholeQuery is the query the field answers: its conditions and words,
// without the filter being written where suggestions stand for it.
func (s *State) wholeQuery() string {
	words := s.field.words
	if s.offered() {
		words = s.unwritten()
	}
	return strings.TrimSpace(strings.Join(append(slices.Clone(s.field.conditions), words), " "))
}

// unwritten is the words without the filter the caret is in.
func (s *State) unwritten() string {
	words, caret := s.field.words, s.field.caret
	query.Walk(query.Parse(words).Root, func(n query.Node) {
		if f, ok := n.(*query.Filter); ok && f.From < caret && caret <= f.To && f.To <= len(words) {
			words = words[:f.From] + words[f.To:]
		}
	})
	return words
}

// takeSuggestion writes the suggestion at i in place of what it is for; a
// value, which finishes its filter, then becomes a ribbon.
func (s *State) takeSuggestion(i string) {
	at, err := strconv.Atoi(i)
	items := s.field.completion.Items
	if err != nil || at < 0 || at >= len(items) {
		return
	}
	item, span := items[at], s.field.completion.Span
	words := s.field.words
	if span.To > len(words) || span.From > span.To {
		return
	}
	rest := words[span.To:]
	words = words[:span.From] + item.Text
	if item.Done && !strings.HasPrefix(rest, " ") {
		words += " "
	}
	s.field.focus = true
	s.setWords(words + rest)
}

// takeKey writes the key named name in the pill, in place of a filter
// being written, for its value to be written next, or takes it at once
// where it stands alone.
func (s *State) takeKey(name string) {
	k := mustKey(name)
	if k == nil {
		return
	}
	s.field.chosen = ""
	s.field.focus = true
	if k.Takes == query.TakesNothing {
		s.addCondition((&query.Condition{Key: k}).String())
		return
	}
	// In place of a filter being written, as another key is chosen.
	words := strings.TrimRight(s.unwritten(), " ")
	if words != "" {
		words += " "
	}
	s.setWords(words + "$" + k.Name + ":")
}

// addCondition puts cond in the query, unless it is there already.
func (s *State) addCondition(cond string) {
	if slices.Contains(s.field.conditions, cond) {
		return
	}
	s.field.conditions = append(s.field.conditions, cond)
	s.field.next = change{depth: stage.Down}
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
	s.field.conditions[i] = cond.String()
	s.field.chosen = ""
}

// rewrite takes the condition at i back into the pill, after what is
// written, the caret in its value for it to be written again.
func (s *State) rewrite(i int) {
	text := s.field.conditions[i]
	s.field.conditions = slices.Delete(s.field.conditions, i, i+1)
	words := strings.TrimRight(s.field.words, " ")
	if words != "" {
		words += " "
	}
	s.field.chosen = ""
	s.field.focus = true
	s.setWords(words + text)
}
