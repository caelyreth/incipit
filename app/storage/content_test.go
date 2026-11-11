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
	hits, err := s.Search("findable", 10)
	if err != nil || len(hits) != 1 || hits[0] != 1 {
		t.Fatalf("search: %v %v", hits, err)
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
