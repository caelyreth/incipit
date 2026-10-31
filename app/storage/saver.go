package storage

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

const (
	// settleDelay is how long the notebook waits after the last change, so
	// one burst of typing is one write.
	settleDelay = 250 * time.Millisecond
	// longestDelay bounds how long a change waits while typing goes on:
	// the most a crash could take back.
	longestDelay = time.Second
	// retryDelay paces the attempts after a failed write; nothing pending
	// is dropped meanwhile.
	retryDelay = 2 * time.Second
)

// Saver writes the notebook as it changes, off the caller's thread. Callers
// hand it immutable snapshots, which it can read at its leisure; memory
// remains the truth, and what is pending stays pending until it is on disk.
type Saver struct {
	store *Store

	mu      sync.Mutex
	pending map[PageKey]pendingPage
	layout  *Layout
	first   time.Time
	last    time.Time
	flushes []chan error
	err     error
	// backupErr is the last backup's failure: the notebook itself is
	// saved, only its copy is behind.
	backupErr error
	closing   bool
	wake      chan struct{}
	done      chan struct{}

	// stored is what is on disk for each page, read by the loop alone.
	stored map[PageKey]model.DocumentSnapshot
	// wrote is whether anything was written since the last backup.
	wrote      bool
	lastBackup time.Time
	now        func() time.Time
}

type pendingPage struct {
	title    string
	snapshot model.DocumentSnapshot
}

// Autosave starts saving the notebook. stored holds the snapshot each page
// was loaded as, so the first write of a page is only what changed.
func (s *Store) Autosave(stored map[PageKey]model.DocumentSnapshot) *Saver {
	saver := &Saver{
		store:      s,
		pending:    map[PageKey]pendingPage{},
		wake:       make(chan struct{}, 1),
		done:       make(chan struct{}),
		stored:     maps.Clone(stored),
		lastBackup: time.Now(),
		now:        time.Now,
	}
	if saver.stored == nil {
		saver.stored = map[PageKey]model.DocumentSnapshot{}
	}
	go saver.run()
	return saver
}

// Page asks for a page to be saved as the snapshot shows it. A later call
// before the write replaces it.
func (v *Saver) Page(key PageKey, title string, snapshot model.DocumentSnapshot) {
	v.mu.Lock()
	v.pending[key] = pendingPage{title: title, snapshot: snapshot}
	v.touch()
	v.mu.Unlock()
}

// Layout asks for the notebook's layout to be saved.
func (v *Saver) Layout(layout Layout) {
	layout.Tabs = append([]PageKey(nil), layout.Tabs...)
	layout.Scroll = maps.Clone(layout.Scroll)
	layout.Selections = maps.Clone(layout.Selections)
	v.mu.Lock()
	v.layout = &layout
	v.touch()
	v.mu.Unlock()
}

// touch marks a change; the caller holds the lock.
func (v *Saver) touch() {
	now := v.now()
	if v.first.IsZero() {
		v.first = now
	}
	v.last = now
	v.signal()
}

func (v *Saver) signal() {
	select {
	case v.wake <- struct{}{}:
	default:
	}
}

// Flush writes everything pending now and reports how that went.
func (v *Saver) Flush() error {
	answer := make(chan error, 1)
	v.mu.Lock()
	if v.closing {
		v.mu.Unlock()
		return ErrClosed
	}
	v.flushes = append(v.flushes, answer)
	v.signal()
	v.mu.Unlock()
	return <-answer
}

// Err is the last write's failure, or nil once a write succeeds again.
func (v *Saver) Err() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.err
}

// BackupErr is the last backup's failure, or nil once one succeeds again.
func (v *Saver) BackupErr() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.backupErr
}

// Close writes everything pending, takes a backup when the notebook
// changed, and stops. It reports a write that could not be made.
func (v *Saver) Close() error {
	v.mu.Lock()
	if v.closing {
		v.mu.Unlock()
		<-v.done
		return v.Err()
	}
	v.closing = true
	v.signal()
	v.mu.Unlock()
	<-v.done
	return v.Err()
}

func (v *Saver) run() {
	defer close(v.done)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		v.mu.Lock()
		urgent := v.closing || len(v.flushes) > 0
		dirty := len(v.pending) > 0 || v.layout != nil
		var wait time.Duration
		if dirty && !urgent {
			now := v.now()
			wait = min(v.last.Add(settleDelay).Sub(now), v.first.Add(longestDelay).Sub(now))
			if v.err != nil {
				wait = max(wait, retryDelay-now.Sub(v.last))
			}
		}
		closing := v.closing
		v.mu.Unlock()

		switch {
		case urgent || dirty && wait <= 0:
			err := v.writePending()
			if err == nil {
				v.backup(closing)
			}
			v.answer(err)
			if closing {
				if err != nil {
					v.rescue()
				}
				return
			}
			if err != nil {
				timer.Reset(retryDelay)
				select {
				case <-timer.C:
				case <-v.wake:
					timer.Stop()
				}
			}
		case dirty:
			timer.Reset(wait)
			select {
			case <-timer.C:
			case <-v.wake:
				timer.Stop()
			}
		default:
			<-v.wake
		}
	}
}

// writePending takes what is pending and writes it in one transaction. On
// failure it goes back, unless a newer change of the same page arrived.
func (v *Saver) writePending() error {
	v.mu.Lock()
	pending, layout := v.pending, v.layout
	v.pending, v.layout = map[PageKey]pendingPage{}, nil
	v.first = time.Time{}
	v.mu.Unlock()
	if len(pending) == 0 && layout == nil {
		return nil
	}
	writes := make([]pageWrite, 0, len(pending))
	for key, page := range pending {
		stored, known := v.stored[key]
		writes = append(writes, pageWrite{
			key:      key,
			title:    page.title,
			snapshot: page.snapshot,
			stored:   stored,
			known:    known,
		})
	}
	err := v.store.write(context.Background(), writes, layout, v.now())
	v.mu.Lock()
	defer v.mu.Unlock()
	if err != nil {
		for key, page := range pending {
			if _, newer := v.pending[key]; !newer {
				v.pending[key] = page
			}
		}
		if v.layout == nil {
			v.layout = layout
		}
		if v.first.IsZero() {
			v.first = v.now()
		}
		v.last = v.now()
		v.err = err
		return err
	}
	for key, page := range pending {
		v.stored[key] = page.snapshot
	}
	v.err = nil
	v.wrote = true
	return nil
}

// backup takes the periodic backup while the notebook changes, and the
// closing one when it changed at all.
func (v *Saver) backup(closing bool) {
	now := v.now()
	if !v.wrote || !closing && now.Sub(v.lastBackup) < backupInterval {
		return
	}
	err := v.store.backup(context.Background(), now)
	v.mu.Lock()
	v.backupErr = err
	v.mu.Unlock()
	if err == nil {
		v.wrote = false
		v.lastBackup = now
	}
}

func (v *Saver) answer(err error) {
	v.mu.Lock()
	flushes := v.flushes
	v.flushes = nil
	v.mu.Unlock()
	for _, answer := range flushes {
		answer <- err
	}
}
