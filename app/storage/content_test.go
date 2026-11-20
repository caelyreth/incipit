package storage

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
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

// A page saved before pages kept their tags and tasks has them read from
// its content once, as the notebook loads.
func TestTagsAndTasksSavedBeforeAreReadOnce(t *testing.T) {
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
	if err := tx.Apply(doc.Insert{Parent: snap.Root().ID(), Nodes: []*doc.Node{doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(task))}}); err != nil {
		t.Fatal(err)
	}
	snap, _ = tx.Commit()
	saver := s.Autosave()
	saver.Content(1, Summarize(snap.Root()), snap)
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE pages SET tags = NULL, open_tasks = NULL, done_tasks = NULL`); err != nil {
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
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
