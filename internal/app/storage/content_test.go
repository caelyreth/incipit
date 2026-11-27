package storage

import (
	"errors"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/store"
)

func TestPageContentSavesAsRowsAndLoads(t *testing.T) {
	root := t.TempDir()
	s := openStore(t, root)
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	para := snap.Root().Slot(0).Child(0)
	tx := snap.Begin(doc.Origin{Class: doc.OriginTyping})
	if err := tx.Apply(doc.EditText{At: doc.SlotRef{Node: para.ID()}, Items: []doc.Item{doc.TextItem("findable words", nil)}}); err != nil {
		t.Fatal(err)
	}
	snap, _ = tx.Commit()
	saver := s.Autosave()
	saver.Content(1, Summary{Title: "One"}, snap)
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openStore(t, root)
	back, err := s.LoadPage(1, profile.Schema(), doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	if back == nil || back.Root().Slot(0).Child(0).Text() != "findable words" {
		t.Fatalf("loaded %v", back)
	}
}

// A page the notebook has not written yet has no content to load.
func TestUnwrittenPageLoadsEmpty(t *testing.T) {
	s := openStore(t, t.TempDir())
	defer s.Close()
	snap, err := s.LoadPage(42, profile.Schema(), doc.NewInterner())
	if err != nil || snap != nil {
		t.Fatalf("unwritten page: %v, %v", snap, err)
	}
}

// A page saved before pages kept their tags, tasks and picture has them
// read from its content once, as the notebook loads.
func TestTagsTasksAndPictureSavedBeforeAreReadOnce(t *testing.T) {
	root := t.TempDir()
	s := openStore(t, root)
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	para := snap.Root().Slot(0).Child(0)
	tx := snap.Begin(doc.Origin{Class: doc.OriginTyping})
	if err := tx.Apply(doc.EditText{At: doc.SlotRef{Node: para.ID()}, Items: []doc.Item{doc.TextItem("about #tides", nil)}}); err != nil {
		t.Fatal(err)
	}
	task := doc.NewNode(profile.Item, taskAttrs(profile.TaskOpen), doc.InlineSlot(doc.TextItem("swim", nil)), doc.BlockSlot())
	figure := profile.NewFigure(profile.Schema(), profile.Picture{Media: "cafe", Width: 4, Height: 3}, "")
	if err := tx.Apply(doc.Insert{Parent: snap.Root().ID(), Nodes: []*doc.Node{doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(task)), figure}}); err != nil {
		t.Fatal(err)
	}
	snap, _ = tx.Commit()
	saver := s.Autosave()
	saver.Content(1, Summarize(snap.Root()), snap)
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE pages SET tags = NULL, open_tasks = NULL, done_tasks = NULL, picture = NULL`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		s = openStore(t, root)
		notebook, err := s.Load()
		if err != nil {
			t.Fatal(err)
		}
		if got := notebook.Pages[0].Tags; len(got) != 1 || got[0] != (Tag{Name: "tides", Count: 1}) {
			t.Fatalf("tags %+v", got)
		}
		if got := notebook.Pages[0].Tasks; got.Open != 1 || got.Done != 0 {
			t.Fatalf("tasks %+v", got)
		}
		if got := notebook.Pages[0].Picture; got != "cafe" {
			t.Fatalf("picture %q", got)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// A page whose save would not load back as it is in memory is held, its
// disk at the last good save, while the rest is written; its next good
// change is written as ever.
func TestAPageThatWouldNotLoadBackIsHeld(t *testing.T) {
	s := openStore(t, t.TempDir())
	defer s.Close()
	page := func(text string) *doc.Snapshot {
		snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
		if err != nil {
			t.Fatal(err)
		}
		tx := snap.Begin(doc.Origin{Class: doc.OriginTyping})
		if err := tx.Apply(doc.EditText{At: doc.SlotRef{Node: snap.Root().Slot(0).Child(0).ID()}, Items: []doc.Item{doc.TextItem(text, nil)}}); err != nil {
			t.Fatal(err)
		}
		snap, _ = tx.Commit()
		return snap
	}
	text := func(key PageKey) string {
		t.Helper()
		snap, err := s.LoadPage(key, profile.Schema(), doc.NewInterner())
		if err != nil || snap == nil {
			t.Fatalf("page %d: %v, %v", key, snap, err)
		}
		return snap.Root().Slot(0).Child(0).Text()
	}
	saver := s.Autosave()
	saver.Content(1, Summary{}, page("good"))
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	saver.verify = func(prev, next *doc.Snapshot, d store.Diff) error {
		if next.Root().Slot(0).Child(0).Text() == "broken" {
			return store.ErrCorrupt
		}
		return store.Verify(prev, next, d)
	}
	saver.Content(1, Summary{}, page("broken"))
	saver.Content(2, Summary{}, page("other"))
	if err := saver.Flush(); !errors.Is(err, ErrHeld) || !errors.Is(saver.Err(), ErrHeld) {
		t.Fatalf("flush %v, err %v; want the page held", err, saver.Err())
	}
	if got := text(1); got != "good" {
		t.Fatalf("held page on disk %q, want the last good save", got)
	}
	if got := text(2); got != "other" {
		t.Fatalf("the other page %q", got)
	}
	saver.Content(1, Summary{}, page("fixed"))
	if err := saver.Flush(); err != nil || saver.Err() != nil {
		t.Fatalf("flush %v, err %v", err, saver.Err())
	}
	if got := text(1); got != "fixed" {
		t.Fatalf("page %q, want the good change written", got)
	}
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
}
