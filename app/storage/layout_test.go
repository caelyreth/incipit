package storage

import (
	"slices"
	"sync"
	"testing"

	"github.com/caelyreth/neverland/app/library"
)

func TestLayoutEqualComparesByValue(t *testing.T) {
	layout := func() Layout {
		return Layout{Tabs: []PageKey{1, 2}, Active: 1, HasActive: true, Scroll: map[PageKey]float32{1: 12}}
	}
	changed := []func(*Layout){
		func(l *Layout) { l.Tabs[0] = 3 },
		func(l *Layout) { l.Active = 2 },
		func(l *Layout) { l.HasActive = false },
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
		Tabs:      []PageKey{1},
		Active:    1,
		HasActive: true,
		Scroll:    map[PageKey]float32{1: 12},
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
		Chapters: []library.ChapterRecord{{
			Chapter:  library.Chapter{ID: 3, Group: 2, Name: "Original chapter"},
			Position: 3,
		}},
	}
	saver.Layout(layout)
	saver.Library(records)

	// These are the caller's values once the calls return.
	layout.Tabs[0] = 9
	layout.Scroll[1] = 99
	records.Groups[0].Name = "Changed"
	records.Sheets[0].Group = 9
	records.Chapters[0].Name = "Changed chapter"

	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	stored := notebook.Layout
	if !slices.Equal(stored.Tabs, []PageKey{1}) || stored.Active != 1 || stored.Scroll[1] != 12 {
		t.Fatalf("layout after caller mutation = %+v", stored)
	}
	if len(notebook.Library.Groups) != 1 || notebook.Library.Groups[0].Name != "Original" {
		t.Fatalf("groups after caller mutation = %+v", notebook.Library.Groups)
	}
	if len(notebook.Library.Sheets) != 1 || notebook.Library.Sheets[0].Group != 2 {
		t.Fatalf("sheets after caller mutation = %+v", notebook.Library.Sheets)
	}
	if len(notebook.Library.Chapters) != 1 || notebook.Library.Chapters[0].Name != "Original chapter" {
		t.Fatalf("chapters after caller mutation = %+v", notebook.Library.Chapters)
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
				Tabs:      []PageKey{key},
				Active:    key,
				HasActive: true,
				Scroll:    map[PageKey]float32{key: float32(key)},
			}
			saver.Layout(layout)

			layout.Tabs[0] = 0
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
	if !layout.HasActive || len(layout.Tabs) != 1 || layout.Active != layout.Tabs[0] {
		t.Fatalf("incoherent saved layout = %+v", layout)
	}
	key := layout.Active
	if layout.Scroll[key] != float32(key) {
		t.Fatalf("saved scroll = %v for page %d", layout.Scroll[key], key)
	}
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
