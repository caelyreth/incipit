package window

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caelyreth/neverland/internal/app/library"
	"github.com/caelyreth/neverland/internal/app/search"
	"github.com/caelyreth/neverland/internal/text/query"
)

const (
	lineBatchSize  = 8
	lineBatchDelay = 12 * time.Millisecond
)

// lineKey is a sheet at a revision of the index: its lines stand while
// it does.
type lineKey struct {
	id  library.SheetID
	rev uint64
}

// line is the line of a sheet's text a query matches, and where.
type line struct {
	text  string
	spans []query.Span
}

// lineJob is a sheet whose line is wanted, and how to read its text: off
// the UI thread, from what cannot change under it (indexEntry).
type lineJob struct {
	key  lineKey
	read func() (search.Text, error)
}

// lines reads the lines of found sheets' texts that the search matches,
// off the UI thread, for the search being shown: a new query lets the
// last one's go.
type lines struct {
	changed func()

	mu      sync.Mutex
	current string
	matcher *query.Matcher
	done    map[lineKey]line
	asked   map[lineKey]bool
	pending []lineJob

	gen  atomic.Uint64
	wake chan struct{}
	stop chan struct{}
	quit chan struct{}
}

func newLines(changed func()) *lines {
	l := &lines{
		changed: changed, done: map[lineKey]line{}, asked: map[lineKey]bool{},
		wake: make(chan struct{}, 1), stop: make(chan struct{}), quit: make(chan struct{}),
	}
	go l.run()
	return l
}

// generation counts the lines read: what was shown at one stands while it
// does.
func (l *lines) generation() uint64 { return l.gen.Load() }

// query makes text, matched by m, the search whose lines are read.
func (l *lines) query(text string, m *query.Matcher) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if text == l.current && l.matcher != nil {
		return
	}
	l.current, l.matcher = text, m
	l.done, l.asked, l.pending = map[lineKey]line{}, map[lineKey]bool{}, nil
}

// line is a sheet's line, once read.
func (l *lines) line(k lineKey) (line, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	got, ok := l.done[k]
	return got, ok
}

// want asks for the lines of jobs, for the search text.
func (l *lines) want(text string, jobs []lineJob) {
	l.mu.Lock()
	if text != l.current {
		l.mu.Unlock()
		return
	}
	asked := false
	for _, j := range jobs {
		if !l.asked[j.key] {
			l.asked[j.key] = true
			l.pending = append(l.pending, j)
			asked = true
		}
	}
	l.mu.Unlock()
	if asked {
		select {
		case l.wake <- struct{}{}:
		default:
		}
	}
}

func (l *lines) close() {
	close(l.stop)
	<-l.quit
}

func (l *lines) run() {
	defer close(l.quit)
	for {
		select {
		case <-l.stop:
			return
		case <-l.wake:
		}
		for {
			start := time.Now()
			processed := 0
			for processed < lineBatchSize {
				l.mu.Lock()
				if len(l.pending) == 0 {
					l.mu.Unlock()
					break
				}
				job, text, m := l.pending[0], l.current, l.matcher
				l.pending = l.pending[1:]
				l.mu.Unlock()

				got := line{}
				if t, err := job.read(); err != nil {
					log.Printf("search line: %v", err)
				} else if spans := m.Find(t.Body); len(spans) > 0 {
					got.text, got.spans = query.Line(t.Body, spans, lineWidth)
				}

				l.mu.Lock()
				if text == l.current {
					l.done[job.key] = got
				}
				l.mu.Unlock()
				processed++

				select {
				case <-l.stop:
					return
				default:
				}
				if time.Since(start) >= lineBatchDelay {
					break
				}
			}
			if processed == 0 {
				break
			}
			l.gen.Add(1)
			if l.changed != nil {
				l.changed()
			}
		}
	}
}
