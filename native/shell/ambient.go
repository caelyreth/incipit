package shell

// Ambient cards are a few quiet cards the field shows when nothing is
// asked (field-and-desk.md §4.4.1): how much was written today, a line
// for the day. They answer only an empty query, stand after everything
// else, are never more than ambientMost, and each says itself whether it
// shows, turned off or with nothing to say.

// ambient is one kind of ambient card.
type ambient interface {
	// id names the card among the field's.
	id() string
	// shows reports whether the card stands in the field now.
	shows(s *State) bool
	// card is the card, as it stands now.
	card(s *State) cardKind
}

// ambients are the ambient cards, in the order they are offered.
var ambients []ambient

// ambientMost is how many ambient cards stand at once.
const ambientMost = 2

type ambientSource struct{}

func (ambientSource) answer(s *State, text string, f *found) {
	if text != "" || s.field.words != "" {
		return
	}
	shown := 0
	for _, a := range ambients {
		if shown == ambientMost {
			return
		}
		if a.shows(s) {
			f.add("ambient:"+a.id(), tierAmbient, a.card(s))
			shown++
		}
	}
}
