package motion

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// SelectionFrequency is how fast a selection's rows glide: a little
// slower than the caret, so the caret leads to the range's end while the
// range settles in beneath it.
const SelectionFrequency = float32(34)

// SelectionFade is how long a selection takes to appear from none.
const SelectionFade = 160 * time.Millisecond

// Selection glides a selection's rows, each edge on its own spring, so a
// selection retargeted mid-motion carries on from where it is drawn at
// the speed it is going. Rows that keep their line glide to their new
// extent; when the rows change, each new row starts as the drawn row
// nearest it and glides to its place, so a selection spreads from what
// was selected. Only a selection appearing from none fades in.
type Selection struct {
	rows   []Rect
	drawn  []ui.Rect
	shown  time.Time
	fading bool
	alpha  float32
}

// Step glides toward the target rows and returns the rows drawn now.
// Unanimated, as while a drag follows the pointer, the rows stand at
// their targets but are kept, so motion resumes from them.
func (s *Selection) Step(target []ui.Rect, now time.Time, animate bool) []ui.Rect {
	if !animate {
		s.rows = make([]Rect, len(target))
		for i, r := range target {
			s.rows[i].Step(r, now, SelectionFrequency, false)
		}
		s.drawn = append(s.drawn[:0], target...)
		s.alpha, s.fading = 1, false
		return s.drawn
	}
	if !s.sameRows(target) {
		if len(s.drawn) == 0 && len(target) > 0 {
			s.shown, s.fading = now, true
		}
		drawn := append([]ui.Rect(nil), s.drawn...)
		s.rows = make([]Rect, len(target))
		if len(drawn) > 0 {
			for i, r := range target {
				s.rows[i].Step(nearestRow(drawn, r), now, SelectionFrequency, false)
			}
		}
	}
	s.fade(len(target) > 0, now)
	s.drawn = s.drawn[:0]
	for i, r := range target {
		s.drawn = append(s.drawn, s.rows[i].Step(r, now, SelectionFrequency, true))
	}
	return s.drawn
}

// sameRows reports whether target keeps the drawn rows' lines, so each
// row glides on rather than starting again.
func (s *Selection) sameRows(target []ui.Rect) bool {
	if len(s.rows) != len(target) {
		return false
	}
	for i := range s.rows {
		if !s.rows[i].Shown() || s.rows[i].axes[1].Target() != target[i].Y {
			return false
		}
	}
	return true
}

// fade eases a selection appearing from none in: a cubic ease-out, so the
// range is legible quickly and settles softly.
func (s *Selection) fade(visible bool, now time.Time) {
	s.alpha = 1
	if !visible {
		s.fading = false
	}
	if !s.fading {
		return
	}
	progress := float32(now.Sub(s.shown)) / float32(SelectionFade)
	if progress >= 1 {
		s.fading = false
		return
	}
	rest := 1 - max(0, progress)
	s.alpha = 1 - rest*rest*rest
}

// Opacity is the fade the last Step drew at.
func (s *Selection) Opacity() float32 { return s.alpha }

// Moving reports rows still gliding or fading in.
func (s *Selection) Moving() bool {
	if s.fading {
		return true
	}
	for i := range s.rows {
		if s.rows[i].Moving() {
			return true
		}
	}
	return false
}

// nearestRow is the row of rows whose vertical middle is nearest
// target's.
func nearestRow(rows []ui.Rect, target ui.Rect) ui.Rect {
	best, distance := rows[0], float32(-1)
	middle := target.Y + target.H/2
	for _, row := range rows {
		gap := abs(row.Y + row.H/2 - middle)
		if distance < 0 || gap < distance {
			best, distance = row, gap
		}
	}
	return best
}
