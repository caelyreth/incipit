package shell

import "slices"

// The field's places are a history, as a browser's pages are: going into
// a group, the trash or the settings goes to a new place, and Back and
// Forward (⌘[ and ⌘], the Go menu) walk them, each place coming back with
// what was written in it and where it was scrolled (scroll.go). A place
// is the query's conditions; the words written in it are its own, kept
// as they change, so a search in progress is never lost by looking
// elsewhere.

// visit is a place in the history: its conditions and its words.
type visit struct {
	conds []string
	words string
}

// placesHeld is how many places the history keeps.
const placesHeld = 64

// notePlace keeps the history up with the query: a change of conditions
// is a new place, after the one it left, with nothing forward of it;
// words written are the place's own.
func (s *State) notePlace() {
	h := &s.field.history
	if s.commanding() {
		return
	}
	if h.at >= 0 && h.at < len(h.visits) && slices.Equal(h.visits[h.at].conds, s.field.conditions) {
		h.visits[h.at].words = s.field.words
		return
	}
	h.visits = append(h.visits[:min(h.at+1, len(h.visits))], visit{conds: slices.Clone(s.field.conditions), words: s.field.words})
	if n := len(h.visits); n > placesHeld {
		h.visits = slices.Delete(h.visits, 0, n-placesHeld)
	}
	h.at = len(h.visits) - 1
}

// canGo reports whether the place history goes by step: -1 back, 1
// forward.
func (s *State) canGo(step int) bool {
	h := &s.field.history
	at := h.at + step
	return s.Library != nil && at >= 0 && at < len(h.visits)
}

// goPlace goes along the place history by step, to the place as it was
// left: its conditions, its words.
func (s *State) goPlace(step int) {
	if !s.canGo(step) {
		return
	}
	h := &s.field.history
	h.at += step
	v := h.visits[h.at]
	next := change{depth: stageUp}
	if step > 0 {
		next = change{depth: stageDown}
	}
	s.ShowField()
	s.field.picked, s.field.chosen, s.field.dismissed = nil, "", ""
	if s.field.opened != nil {
		s.closeKey()
	}
	s.field.conditions = slices.Clone(v.conds)
	s.setWords(v.words)
	s.field.rerank = true
	s.field.next = next
}

// leave goes back from a place set out to as a side trip, the trash or
// the settings, to where the field was before it; with nothing before
// it, gone reports false.
func (s *State) leave(kind string) (gone bool) {
	h := &s.field.history
	if h.at <= 0 || h.at >= len(h.visits) {
		return false
	}
	if slices.ContainsFunc(h.visits[h.at-1].conds, func(c string) bool { return conditionKey(c) == kind }) {
		return false
	}
	s.goPlace(-1)
	return true
}
