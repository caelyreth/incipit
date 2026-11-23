package storage

import (
	"slices"
	"sync"
	"testing"

	"github.com/caelyreth/neverland/internal/app/library"
)

func TestLayoutEqualComparesByValue(t *testing.T) {
	layout := func() Layout {
		return Layout{Columns: []Column{{Page: 1}, {Page: 2, Scroll: 4}}, Front: 1, HasFront: true, Scroll: map[PageKey]float32{1: 12}}
	}
	changed := []func(*Layout){
		func(l *Layout) { l.Columns[0].Page = 3 },
		func(l *Layout) { l.Columns[1].View.Node = 3 },
		func(l *Layout) { l.Front = 0 },
		func(l *Layout) { l.HasFront = false },
		func(l *Layout) { l.Scroll[1] = 13 },
	}
	if !layout().Equal(layout()) {
		t.Fatal("separately allocated layouts differ")
	}
	for index, change := range changed {
		other := layout()
		change(&other)
		if layout().Equal(other) || other.Equal(layout()) {
			t.Fatalf("change %d went unnoticed", index)
		}
	}
}

func TestSaverOwnsQueuedLayoutAndLibrary(t *testing.T) {
	store := openStore(t, t.TempDir())
	saver := store.Autosave()
	saver.Page(1, Summary{Title: "one"})

	layout := Layout{
		Columns:  []Column{{Page: 1}},
		HasFront: true,
		Scroll:   map[PageKey]float32{1: 12},
	}
	records := library.Records{
		Groups: []library.GroupRecord{{
			Group:    library.Group{ID: 2, Name: "Original"},
			Position: 1,
		}},
		Sheets: []library.SheetRecord{{
			Sheet:    library.Sheet{ID: 1, Group: 2},
			Position: 2,
		}},
	}
	saver.Layout(layout)
	saver.Library(records)

	// These are the caller's values once the calls return.
	layout.Columns[0].Page = 9
	layout.Scroll[1] = 99
	records.Groups[0].Name = "Changed"
	records.Sheets[0].Group = 9

	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	stored := notebook.Layout
	if !slices.Equal(stored.Columns, []Column{{Page: 1}}) || !stored.HasFront || stored.Scroll[1] != 12 {
		t.Fatalf("layout after caller mutation = %+v", stored)
	}
	if len(notebook.Library.Groups) != 1 || notebook.Library.Groups[0].Name != "Original" {
		t.Fatalf("groups after caller mutation = %+v", notebook.Library.Groups)
	}
	if len(notebook.Library.Sheets) != 1 || notebook.Library.Sheets[0].Group != 2 {
		t.Fatalf("sheets after caller mutation = %+v", notebook.Library.Sheets)
	}
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSaverOwnsConcurrentLayoutSnapshots(t *testing.T) {
	store := openStore(t, t.TempDir())
	saver := store.Autosave()

	stopFlushing := make(chan struct{})
	flushesDone := make(chan struct{})
	var flushErr error
	go func() {
		defer close(flushesDone)
		for {
			select {
			case <-stopFlushing:
				return
			default:
			}
			if err := saver.Flush(); err != nil {
				flushErr = err
				return
			}
		}
	}()

	var enqueued sync.WaitGroup
	for index := 1; index <= 32; index++ {
		enqueued.Add(1)
		go func(key PageKey) {
			defer enqueued.Done()
			layout := Layout{
				Columns:  []Column{{Page: key, Scroll: float32(key)}},
				HasFront: true,
				Scroll:   map[PageKey]float32{key: float32(key)},
			}
			saver.Layout(layout)

			layout.Columns[0].Page = 0
			layout.Scroll[key] = 999
		}(PageKey(index))
	}
	enqueued.Wait()
	close(stopFlushing)
	<-flushesDone
	if flushErr != nil {
		t.Fatal(flushErr)
	}
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}

	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	layout := notebook.Layout
	if !layout.HasFront || len(layout.Columns) != 1 {
		t.Fatalf("incoherent saved layout = %+v", layout)
	}
	key := layout.Columns[0].Page
	if layout.Scroll[key] != float32(key) || layout.Columns[0].Scroll != float32(key) {
		t.Fatalf("saved scroll = %v for page %d", layout.Scroll[key], key)
	}
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
