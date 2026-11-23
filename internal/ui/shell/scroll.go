package shell

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// Where the field is scrolled is one thing, read and moved in one way.
// Each place the field shows, a view with its query, keeps its own scroll:
// gone to, it stands as it was left, or at its home when new (the field's
// horizon, a list's end for filed order and time sorts, or a list's top for
// title and word-count sorts; short lists stand on the foot), as a
// browser's history does. Moved to at a change, the field goes there on
// the change's own clock, the horizon and what it drew alike, so nothing
// moves on screen for the scroll alone: the change moves only what truly
// stands elsewhere, straight from where it was seen. Within a place the
// field stays where it is as what it holds changes, kept within its reach
// and at its end when it was there; and the keys, the pill and what
// arrives bring it on a spring of its own (comeHome).

// placeScroll is where a place was scrolled to as it was left.
type placeScroll struct {
	place  string
	scroll float32
}

// placesKept is how many places the field remembers being scrolled in.
const placesKept = 32

// placeKey names a place: the view, and the query it shows.
func placeKey(listed bool, query string) string {
	if listed {
		return "list\x00" + query
	}
	return "field\x00" + query
}

// remember is places with place scrolled to scroll, the latest last, the
// oldest let go of past placesKept.
func remember(places []placeScroll, place string, scroll float32) []placeScroll {
	places = slices.DeleteFunc(places, func(p placeScroll) bool { return p.place == place })
	places = append(places, placeScroll{place, scroll})
	if len(places) > placesKept {
		places = places[len(places)-placesKept:]
	}
	return places
}

// recall is where place was scrolled to as it was left, or home when new.
func recall(places []placeScroll, place string, home float32) float32 {
	for _, p := range places {
		if p.place == place {
			return p.scroll
		}
	}
	return home
}

// placeHome is where a new place opens: the field's horizon (0), the end (hi)
// for a list in filed order or forward time sorts ("updated", "created"), and
// the top (0) for title, word-count, and reversed time sorts. scrollTo clamps
// the result to [lo, hi], so short lists always stand on the foot.
func (s *State) placeHome(listed bool) float32 {
	if !listed {
		return 0
	}
	switch s.sortValue() {
	case "", "updated", "created":
		return s.field.hi
	default:
		return 0
	}
}

// scrollTo scrolls the field to to, within its reach, for a change that
// lays it out anew: along the change's own clock, from where it stands, as
// the pill's place and the cards go (along), so the horizon, the pill and
// what was drawn move as one, each straight to where it stands; turning
// views, or laid out for the first time, at once, what goes staying where
// it was drawn.
func (s *State) scrollTo(view ui.Element, to float32) {
	to = max(s.field.lo, min(to, s.field.hi))
	if s.field.rest != nil && s.field.next.depth != stageSwap {
		s.field.home, s.field.homeTo = to != s.field.scroll, to
		return
	}
	s.field.scroll, s.field.home, s.field.homing = to, false, false
	view.Jump("scroll", to)
}

// scrollAt is where the field is scrolled to, or, coming home, where it
// goes.
func (s *State) scrollAt() float32 {
	if s.field.home {
		return s.field.homeTo
	}
	return s.field.scroll
}

// homeward brings the field back to its horizon, scrolled from it, as the
// pill pressed or Escape do. A list has no horizon: its pill never leaves
// its foot, and the list stays where it is read.
func (s *State) homeward() {
	if !s.field.listed && (s.field.scroll != 0 || s.field.over.Offset() != 0) {
		s.field.home, s.field.homeTo = true, 0
	}
}

// stageReachDown is how far rects reach down from 0, and up from it.
func stageReachDown(rects []stageRect) (top, end float32) {
	for _, r := range rects {
		top, end = min(top, r.Y), max(end, r.Y+r.H)
	}
	return top, end
}
