package shell

import (
	"slices"
	"time"

	"github.com/egoist/mygo/ui"
)

// The field's order is one the eye can read (field-and-desk.md §4.7.5): a
// ranked field stands by nearness to the pill, and an exact order is the
// list's (list.go); a sort chosen stands as its ribbon, the order of
// itself as none; and what arrives, made, moved, put back or pinned, stands where the
// order puts it, the field bringing it into view and marking it a moment.

// arrivalFor is how long what arrives is marked.
const arrivalFor = 1200 * time.Millisecond

// arrival is when the field first looked for something arriving, and
// when it first showed it.
type arrival struct{ looked, seen time.Time }

// arrive marks ids, rows or cards, as arriving.
func (s *State) arrive(ids ...string) {
	if s.field.arrivals == nil {
		s.field.arrivals = map[string]arrival{}
	}
	for _, id := range ids {
		s.field.arrivals[id] = arrival{}
	}
}

// arrivalAt is how strongly id is marked as arriving at now: from full,
// easing away over arrivalFor from when it was first shown.
func (s *State) arrivalAt(id string, now time.Time) float32 {
	a, ok := s.field.arrivals[id]
	if !ok || a.seen.IsZero() {
		return 0
	}
	t := float32(now.Sub(a.seen)) / float32(arrivalFor)
	if t >= 1 || t < 0 {
		return 0
	}
	return (1 - t) * (1 - t)
}

// reveal brings what arrives into view as the field first shows it, f's
// cards at frames; and lets go of what has been marked its while, or was
// never shown in it.
func (s *State) reveal(c *ui.Context, frames []stageFrame, f found) {
	// The row the keys go to is brought into view as well.
	if s.field.chosen != s.field.shownChosen {
		if r, ok := s.field.rowRects[s.field.chosen]; ok {
			s.inView(r)
		}
		s.field.shownChosen = s.field.chosen
	}
	if len(s.field.arrivals) == 0 {
		return
	}
	now := c.Now()
	c.AnimationFrame()
	for id, a := range s.field.arrivals {
		if a.looked.IsZero() {
			a.looked = now
		}
		switch {
		case !a.seen.IsZero() && now.Sub(a.seen) >= arrivalFor, a.seen.IsZero() && now.Sub(a.looked) >= arrivalFor:
			delete(s.field.arrivals, id)
			continue
		case !a.seen.IsZero():
			continue
		}
		at := slices.IndexFunc(f.cards, func(cd card) bool { return cd.id == id || slices.Contains(cd.rows, id) })
		if at >= 0 && at < len(frames) {
			a.seen = now
			s.inView(frames[at].Rect)
		}
		s.field.arrivals[id] = a
	}
}

// inView scrolls the field as far as brings r, in the field, clear into
// its viewport.
func (s *State) inView(r stageRect) {
	at := s.scrollAt()
	y := s.field.base - at + r.Y
	over, under := s.field.viewTop, s.field.viewBottom
	to := at
	switch {
	case y < over:
		to -= over - y
	case y+r.H > under:
		to += min(y+r.H-under, y-over)
	}
	if to != at {
		s.field.home, s.field.homeTo = true, max(s.field.lo, min(to, s.field.hi))
	}
}
