package window

import (
	"time"

	"github.com/caelyreth/neverland/internal/app/storage"
)

// written counts the words written each day (storage.DayOf): what writing
// adds to the pages' summaries, less what it takes away. A page's first
// reading is where it starts from, so a page loaded, copied or made
// whole writes nothing. changed are the days the saver is yet to be
// handed.
type written struct {
	days    map[string]int
	changed map[string]bool
}

// add counts words more as written at.
func (w *written) add(at time.Time, words int) {
	if words == 0 {
		return
	}
	day := storage.DayOf(at)
	if w.days == nil {
		w.days = map[string]int{}
	}
	if w.changed == nil {
		w.changed = map[string]bool{}
	}
	w.days[day] += words
	w.changed[day] = true
}

// on is the words written on the day at falls on.
func (w *written) on(at time.Time) int { return w.days[storage.DayOf(at)] }

// saveWritten hands the saver the days written on since the last frame.
func (a *AppState) saveWritten() {
	for day := range a.written.changed {
		a.notebook.saver.Written(day, a.written.days[day])
	}
	a.written.changed = nil
}
