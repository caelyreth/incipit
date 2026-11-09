package storage

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/caelyreth/neverland/app/library"
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
// hand it values it owns from then on; memory remains the truth, and what is
// pending stays pending until it is on disk.
type Saver struct {
	store *Store

	mu      sync.Mutex
	pending map[PageKey]string
	deleted map[PageKey]bool
	library *library.Records
	layout  *Layout
	first   time.Time
	last    time.Time
	flushes []chan error
	err     error
	// backupErr is the last backup's failure: the notebook itself is
	// saved, only its copy is behind.
	backupErr error
	// watch is told after each attempt to write.
	watch   func()
	closing bool
	wake    chan struct{}
	done    chan struct{}

	// wrote is whether anything was written since the last backup.
	wrote      bool
	lastBackup time.Time
	now        func() time.Time
}

// Autosave starts saving the notebook.
func (s *Store) Autosave() *Saver {
	saver := &Saver{
		store:      s,
		pending:    map[PageKey]string{},
		deleted:    map[PageKey]bool{},
		wake:       make(chan struct{}, 1),
		done:       make(chan struct{}),
		lastBackup: time.Now(),
		now:        time.Now,
	}
	go saver.run()
	return saver
}

// Page asks for a page's record to be saved with title. A later call before
// the write replaces it.
func (v *Saver) Page(key PageKey, title string) {
	v.mu.Lock()
	v.pending[key] = title
	v.touch()
	v.mu.Unlock()
}

// Delete asks for a page to be removed from disk for good. A change of it
// still pending goes with it.
func (v *Saver) Delete(key PageKey) {
	v.mu.Lock()
	delete(v.pending, key)
	v.deleted[key] = true
	v.touch()
	v.mu.Unlock()
}

// Library asks for the library's filing to be saved. The saver copies the
// records before returning, so the caller may reuse them immediately.
func (v *Saver) Library(records library.Records) {
	records.Groups = slices.Clone(records.Groups)
	records.Sheets = slices.Clone(records.Sheets)
	records.Chapters = slices.Clone(records.Chapters)
	v.mu.Lock()
	v.library = &records
	v.touch()
	v.mu.Unlock()
}

// Layout asks for the notebook's layout to be saved. The saver copies it
// before returning.
func (v *Saver) Layout(layout Layout) {
	layout = layout.clone()
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

// Watch has fn called, from the saver's goroutine, after each attempt to
// write and back up, for Err and BackupErr to be read anew.
func (v *Saver) Watch(fn func()) {
	v.mu.Lock()
	v.watch = fn
	v.mu.Unlock()
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
		dirty := len(v.pending) > 0 || len(v.deleted) > 0 || v.library != nil || v.layout != nil
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
			v.mu.Lock()
			watch := v.watch
			v.mu.Unlock()
			if watch != nil && !closing {
				watch()
			}
			if closing {
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
// failure it goes back, unless something newer of the same kind arrived.
func (v *Saver) writePending() error {
	v.mu.Lock()
	pending, deleted, records, layout := v.pending, v.deleted, v.library, v.layout
	v.pending, v.deleted, v.library, v.layout = map[PageKey]string{}, map[PageKey]bool{}, nil, nil
	v.first = time.Time{}
	v.mu.Unlock()
	if len(pending) == 0 && len(deleted) == 0 && records == nil && layout == nil {
		return nil
	}
	work := change{library: records, layout: layout}
	for key, title := range pending {
		work.pages = append(work.pages, pageWrite{key: key, title: title})
	}
	for key := range deleted {
		work.deleted = append(work.deleted, key)
	}
	err := v.store.write(context.Background(), work, v.now())
	v.mu.Lock()
	defer v.mu.Unlock()
	if err != nil {
		for key, title := range pending {
			if _, newer := v.pending[key]; !newer && !v.deleted[key] {
				v.pending[key] = title
			}
		}
		for key := range deleted {
			v.deleted[key] = true
		}
		if v.library == nil {
			v.library = records
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
