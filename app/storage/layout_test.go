package storage

import (
	"slices"
	"sync"
	"testing"

	"github.com/caelyreth/neverland/app/library"
)

func TestLayoutEqualComparesVisualSelectionsByValue(t *testing.T) {
	withVisual := func(anchor, head int) Layout {
		return Layout{
			Tabs:      []PageKey{1},
			Active:    1,
			HasActive: true,
			Scroll:    map[PageKey]float32{1: 12},
			Selections: map[PageKey]Selection{
				1: {
					Anchor: Position{Node: 2, Offset: 1, Affinity: 1},
					Head:   Position{Node: 2, Offset: 4, Affinity: 2},
					Visual: &ProjectedSelection{Anchor: anchor, Head: head},
				},
			},
		}
	}
	tests := []struct {
		name  string
		left  Layout
		right Layout
		equal bool
	}{
		{
			name:  "separately allocated visuals",
			left:  withVisual(3, 8),
			right: withVisual(3, 8),
			equal: true,
		},
		{
			name:  "visual anchor changed",
			left:  withVisual(3, 8),
			right: withVisual(4, 8),
			equal: false,
		},
		{
			name:  "visual head changed",
			left:  withVisual(3, 8),
			right: withVisual(3, 9),
			equal: false,
		},
		{
			name: "visual presence changed",
			left: withVisual(3, 8),
			right: func() Layout {
				layout := withVisual(3, 8)
				selection := layout.Selections[1]
				selection.Visual = nil
				layout.Selections[1] = selection
				return layout
			}(),
			equal: false,
		},
		{
			name: "semantic selection changed",
			left: withVisual(3, 8),
			right: func() Layout {
				layout := withVisual(3, 8)
				selection := layout.Selections[1]
				selection.Head.Offset = 5
				layout.Selections[1] = selection
				return layout
			}(),
			equal: false,
		},
		{
			name: "scroll changed",
			left: withVisual(3, 8),
			right: func() Layout {
				layout := withVisual(3, 8)
				layout.Scroll[1] = 13
				return layout
			}(),
			equal: false,
		},
		{
			name: "tabs changed",
			left: withVisual(3, 8),
			right: func() Layout {
				layout := withVisual(3, 8)
				layout.Tabs[0] = 2
				return layout
			}(),
			equal: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.left.Equal(test.right); got != test.equal {
				t.Fatalf("left.Equal(right) = %v, want %v", got, test.equal)
			}
			if got := test.right.Equal(test.left); got != test.equal {
				t.Fatalf("right.Equal(left) = %v, want %v", got, test.equal)
			}
		})
	}
}

func TestSaverOwnsQueuedLayoutAndLibrary(t *testing.T) {
	store := openStore(t, t.TempDir())
	saver := store.Autosave(nil)
	document := page(t, "one")
	saver.Page(1, "", document.Snapshot())

	visual := &ProjectedSelection{Anchor: 3, Head: 8}
	layout := Layout{
		Tabs:      []PageKey{1},
		Active:    1,
		HasActive: true,
		Scroll:    map[PageKey]float32{1: 12},
		Selections: map[PageKey]Selection{
			1: {
				Anchor: Position{Node: 2, Offset: 1, Affinity: 1},
				Head:   Position{Node: 2, Offset: 4, Affinity: 2},
				Visual: visual,
			},
		},
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
	visual.Anchor, visual.Head = 90, 91
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
	if got := stored.Selections[1].Visual; got == nil || *got != (ProjectedSelection{Anchor: 3, Head: 8}) {
		t.Fatalf("visual selection after caller mutation = %+v", got)
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
	saver := store.Autosave(nil)

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
			visual := &ProjectedSelection{Anchor: int(key), Head: int(key) + 1}
			layout := Layout{
				Tabs:      []PageKey{key},
				Active:    key,
				HasActive: true,
				Scroll:    map[PageKey]float32{key: float32(key)},
				Selections: map[PageKey]Selection{
					key: {Visual: visual},
				},
			}
			saver.Layout(layout)

			layout.Tabs[0] = 0
			layout.Scroll[key] = 999
			visual.Anchor, visual.Head = 0, 0
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
	visual := layout.Selections[key].Visual
	if visual == nil || visual.Anchor != int(key) || visual.Head != int(key)+1 {
		t.Fatalf("saved visual = %+v for page %d", visual, key)
	}
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
