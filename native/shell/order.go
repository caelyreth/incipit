package shell

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/egoist/mygo/ui"
)

// The field's order is one the eye can read (field-and-desk.md §4.7.5): a
// sorted field fills in reading order, a ranked one by nearness to the
// pill; a sort chosen stands as its ribbon, the order of itself as none;
// and what arrives, made, moved, put back or pinned, stands where the
// order puts it, the field bringing it into view and marking it a moment.

// sortedField reports whether the query states an order of its own, read
// in rows, rather than the best match first.
func (s *State) sortedField() bool {
	for _, text := range s.field.conditions {
		if c, ok := condition(text); ok && written(c).Key.Name == "sort" {
			return strings.TrimPrefix(query.Fold(c.Value), "-") != "relevance"
		}
	}
	return false
}

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
// cards at frames in a field h tall, poised poiseY, the band reaching
// over and under; and lets go of what has been marked its while, or was
// never shown in it.
func (s *State) reveal(c *ui.Context, frames []stageFrame, f found, h, poiseY, over, under float32) {
	// The row the keys go to is brought into view as well.
	if s.field.chosen != s.field.shownChosen {
		if r, ok := s.field.rowRects[s.field.chosen]; ok {
			s.inView(r, h, poiseY, over, under)
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
		at := slices.IndexFunc(f.cards, func(cd card) bool { return cd.id == id || slices.Contains(cd.kind.rows(), id) })
		if at >= 0 && at < len(frames) {
			a.seen = now
			s.inView(frames[at].Rect, h, poiseY, over, under)
		}
		s.field.arrivals[id] = a
	}
}

// inView scrolls the field, h tall and poised poiseY, the band reaching
// over and under, as far as brings r, in the field, into view.
func (s *State) inView(r stageRect, h, poiseY, over, under float32) {
	y := h/2 - poiseY - s.field.scroll + r.Y
	to := s.field.scroll
	switch {
	case y < over:
		to -= over - y
	case y+r.H > h-under:
		to += min(y+r.H-(h-under), y-over)
	}
	if to != s.field.scroll {
		s.field.home, s.field.homeTo = true, max(s.field.lo, min(to, s.field.hi))
	}
}
