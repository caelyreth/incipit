package search

import (
	"sync"
	"sync/atomic"
	"time"
)

// Meta is what the notebook's library says of a page: where it is filed,
// when it was made and changed, and whether it is in the trash.
type Meta struct {
	Groups           []int64
	Created, Updated time.Time
	Trashed          bool
}

// Text is what a page's text says: its summary's title, tags and words,
// and its content (Read).
type Text struct {
	Title string
	Tags  []string
	Words int
	Content
}

// Entry is a page as the notebook holds it now: its revision, its
// library's record, and how to read its text. Text is called off the UI
// thread, only when the index does not hold the revision yet, so it reads
// what cannot change under it: an immutable snapshot, or the store.
type Entry struct {
	Key  int64
	Rev  uint64
	Meta Meta
	Text func() (Text, error)
}

// page is the entry as the index keeps it.
func (e Entry) page(t Text) Page {
	return Page{
		Key: e.Key, Rev: e.Rev, Title: t.Title, Tags: t.Tags, Words: t.Words,
		Headings: t.Headings, Body: t.Body, Code: t.Code, Quote: t.Quote, Kinds: t.Kinds, Domains: t.Domains, Tasks: t.Tasks,
		Groups: e.Meta.Groups, Created: e.Meta.Created, Updated: e.Meta.Updated, Trashed: e.Meta.Trashed,
	}
}

const (
	// settle is how long the indexer waits for changes to pause, and
	// settleAtMost the longest it waits while they go on.
	settle       = 300 * time.Millisecond
	settleAtMost = 2 * time.Second
	// batchPages is how many pages go into the index at once.
	batchPages = 64
)

// Indexer keeps an index in step with a notebook, off the UI thread. The
// notebook says what it holds (Sync, Put, Drop); the indexer reads the
// pages whose revision the index lacks, after changes pause, and lets
// the rest be.
type Indexer struct {
	x *Index
	// Changed is called, from the indexer's goroutine, once the index
	// took changes; Failed with what it could not read or keep.
	changed func()
	failed  func(error)

	mu      sync.Mutex
	pending map[int64]*Entry // nil is a page to let go
	all     map[int64]bool   // what a pending Sync holds; nil for none

	wake  chan struct{}
	flush chan chan struct{}
	stop  chan struct{}
	done  chan struct{}
	gen   atomic.Uint64
	// indexed is each page's revision the index holds; the goroutine's.
	indexed map[int64]uint64
}

// NewIndexer keeps x in step. changed and failed may be nil.
func NewIndexer(x *Index, changed func(), failed func(error)) (*Indexer, error) {
	revs, err := x.Revisions()
	if err != nil {
		return nil, err
	}
	ix := &Indexer{
		x: x, changed: changed, failed: failed, pending: map[int64]*Entry{},
		wake: make(chan struct{}, 1), flush: make(chan chan struct{}), stop: make(chan struct{}), done: make(chan struct{}),
		indexed: revs,
	}
	go ix.run()
	return ix, nil
}

// Index is the index kept in step.
func (ix *Indexer) Index() *Index { return ix.x }

// Generation counts the changes the index took: a search made at one
// generation stands while it does.
func (ix *Indexer) Generation() uint64 { return ix.gen.Load() }

// Sync says every page the notebook holds: those the index holds besides
// go from it.
func (ix *Indexer) Sync(all []Entry) {
	ix.mu.Lock()
	// The whole notebook stands in for whatever was pending before it.
	ix.all = make(map[int64]bool, len(all))
	ix.pending = make(map[int64]*Entry, len(all))
	for i := range all {
		ix.all[all[i].Key] = true
		ix.pending[all[i].Key] = &all[i]
	}
	ix.mu.Unlock()
	ix.poke()
}

// Put says pages that changed.
func (ix *Indexer) Put(entries ...Entry) {
	ix.mu.Lock()
	for i := range entries {
		ix.pending[entries[i].Key] = &entries[i]
		if ix.all != nil {
			ix.all[entries[i].Key] = true
		}
	}
	ix.mu.Unlock()
	ix.poke()
}

// Drop says pages the notebook no longer holds.
func (ix *Indexer) Drop(keys ...int64) {
	ix.mu.Lock()
	for _, k := range keys {
		ix.pending[k] = nil
		if ix.all != nil {
			delete(ix.all, k)
		}
	}
	ix.mu.Unlock()
	ix.poke()
}

// Flush takes what is pending into the index now and returns once it has.
func (ix *Indexer) Flush() {
	reply := make(chan struct{})
	select {
	case ix.flush <- reply:
		<-reply
	case <-ix.done:
	}
}

// Close stops the indexer, letting what is pending go: the next launch
// catches up from the revisions. It does not close the index.
func (ix *Indexer) Close() {
	select {
	case <-ix.stop:
	default:
		close(ix.stop)
	}
	<-ix.done
}

func (ix *Indexer) poke() {
	select {
	case ix.wake <- struct{}{}:
	default:
	}
}

func (ix *Indexer) run() {
	defer close(ix.done)
	for {
		select {
		case <-ix.stop:
			return
		case reply := <-ix.flush:
			ix.apply()
			close(reply)
			continue
		case <-ix.wake:
		}
		// Wait for changes to pause, but not past settleAtMost.
		first := time.Now()
		timer := time.NewTimer(settle)
	settling:
		for {
			select {
			case <-ix.stop:
				timer.Stop()
				return
			case reply := <-ix.flush:
				timer.Stop()
				ix.apply()
				close(reply)
				break settling
			case <-ix.wake:
				if time.Since(first) < settleAtMost {
					timer.Reset(settle)
				}
			case <-timer.C:
				ix.apply()
				break settling
			}
		}
	}
}

// apply takes what is pending into the index.
func (ix *Indexer) apply() {
	ix.mu.Lock()
	pending, all := ix.pending, ix.all
	ix.pending, ix.all = map[int64]*Entry{}, nil
	ix.mu.Unlock()
	var drop []int64
	if all != nil {
		for k := range ix.indexed {
			if !all[k] {
				drop = append(drop, k)
			}
		}
	}
	var batch []Page
	keep := func() {
		if len(batch) == 0 {
			return
		}
		if err := ix.x.Put(batch...); err != nil {
			ix.fail(err)
		} else {
			for _, p := range batch {
				ix.indexed[p.Key] = p.Rev
			}
		}
		batch = batch[:0]
	}
	took := false
	for k, e := range pending {
		if e == nil {
			if _, ok := ix.indexed[k]; ok {
				drop = append(drop, k)
			}
			continue
		}
		if rev, ok := ix.indexed[k]; ok && rev == e.Rev {
			continue
		}
		t, err := e.Text()
		if err != nil {
			ix.fail(err)
			continue
		}
		batch = append(batch, e.page(t))
		took = true
		if len(batch) == batchPages {
			keep()
			select {
			case <-ix.stop:
				return
			default:
			}
		}
	}
	keep()
	if len(drop) > 0 {
		if err := ix.x.Delete(drop...); err != nil {
			ix.fail(err)
		} else {
			for _, k := range drop {
				delete(ix.indexed, k)
			}
			took = true
		}
	}
	if took {
		ix.gen.Add(1)
		if ix.changed != nil {
			ix.changed()
		}
	}
}

func (ix *Indexer) fail(err error) {
	if ix.failed != nil {
		ix.failed(err)
	}
}
