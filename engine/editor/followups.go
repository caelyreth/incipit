package editor

import "github.com/caelyreth/neverland/engine/model"

// A follow-up is what one intent leaves for the next to go on with: a
// deletion armed for its second press, text a reshape already typed, the
// selection a run of Select All grew from, the place Leave just left. It
// holds for the next intent only, at the selection and revision it was
// left at: any other intent, a move, or an edit ends it.
type followUp struct {
	at       model.UTF16Range
	revision uint64

	// arm is a deletion the same press in the same direction completes.
	arm *armedDeletion
	// absorb is text typing takes from, glyph for glyph, as typed.
	absorb string
	// widen is the selection a run of Select All grew from.
	widen *model.TextSelection
	// inside is where the caret stood in the place it just left.
	inside *model.UTF16Offset
}

// armedDeletion is what an armed press completes: Span taken, or for a
// join the press going on as it would have.
type armedDeletion struct {
	span     model.UTF16Range
	join     bool
	backward bool
}

// follow leaves f for the next intent. Within Do it is left once the intent
// is done, at the selection it leaves; outside Do, at the selection now.
func (s *Session) follow(f followUp) {
	if s.doing {
		s.proposed = &f
		return
	}
	s.stamp(f)
}

func (s *Session) stamp(f followUp) {
	at, err := s.VisualSelection()
	if err != nil {
		s.pending = nil
		return
	}
	f.at, f.revision = at, s.runtime.Revision()
	s.pending = &f
}

// live is the follow-up the last intent left, while the selection and the
// page stand as it left them.
func (s *Session) live() (followUp, bool) {
	f := s.pending
	if f == nil || f.revision != s.runtime.Revision() {
		return followUp{}, false
	}
	if at, err := s.VisualSelection(); err != nil || at != f.at {
		return followUp{}, false
	}
	return *f, true
}

// begin starts an intent: what the last one left becomes prior, for this
// one alone to go on with.
func (s *Session) begin() {
	s.prior, _ = s.live()
	s.pending, s.proposed, s.doing = nil, nil, true
}

// end finishes an intent, leaving what it proposed for the next.
func (s *Session) end() {
	s.doing, s.prior = false, followUp{}
	if s.proposed != nil {
		s.stamp(*s.proposed)
		s.proposed = nil
	}
}
