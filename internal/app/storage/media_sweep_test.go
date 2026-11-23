package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
)

// withFigures is snap with a figure appended for each media.
func withFigures(t *testing.T, snap *doc.Snapshot, media ...string) *doc.Snapshot {
	t.Helper()
	tx := snap.Begin(doc.Origin{Class: doc.OriginCommand})
	for _, m := range media {
		figure := profile.NewFigure(profile.Schema(), profile.Picture{Media: m, Width: 2, Height: 2}, "")
		if err := tx.Apply(doc.Insert{Parent: snap.Root().ID(), Nodes: []*doc.Node{figure}}); err != nil {
			t.Fatal(err)
		}
	}
	next, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	return next
}

// withoutFigures is snap with its first n figures removed.
func withoutFigures(t *testing.T, snap *doc.Snapshot, n int) *doc.Snapshot {
	t.Helper()
	var ids []doc.ID
	for _, c := range snap.Root().Slot(0).Children() {
		if c.Kind() == profile.Figure && len(ids) < n {
			ids = append(ids, c.ID())
		}
	}
	tx := snap.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.Remove{IDs: ids}); err != nil {
		t.Fatal(err)
	}
	next, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func emptyPage(t *testing.T, key PageKey) *doc.Snapshot {
	t.Helper()
	snap, err := profile.EmptyPage(doc.ID(key), 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func saveContent(t *testing.T, saver *Saver, key PageKey, snap *doc.Snapshot) {
	t.Helper()
	saver.Content(key, Summarize(snap.Root()), snap)
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
}

// sweepLater sweeps as a launch past the grace would, with no backup
// kept: what the backups hold is tested on its own.
func sweepLater(t *testing.T, s *Store) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(s.root, backupDir)); err != nil {
		t.Fatal(err)
	}
	if err := s.sweepMedia(context.Background(), time.Now().Add(mediaGrace+time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func assertKept(t *testing.T, s *Store, m Media, kept bool) {
	t.Helper()
	_, row := s.Media(m.Hash)
	_, err := os.Stat(s.MediaFile(m))
	switch {
	case kept && (!row || err != nil):
		t.Fatalf("picture %s went (row %v, file %v)", m.Hash[:8], row, err)
	case !kept && (row || !errors.Is(err, os.ErrNotExist)):
		t.Fatalf("picture %s stayed (row %v, file %v)", m.Hash[:8], row, err)
	}
}

// One picture shown by two figures of a sheet and one of another is used
// three times, and goes only once the last of them does.
func TestASharedPictureGoesWithItsLastFigure(t *testing.T) {
	s := openStore(t, t.TempDir())
	defer s.Close()
	shared, err := s.AddMedia(picture(t, 2, 2, 1), "")
	if err != nil {
		t.Fatal(err)
	}
	saver := s.Autosave()
	defer saver.Close()
	one := withFigures(t, emptyPage(t, 1), shared.Hash, shared.Hash)
	two := withFigures(t, emptyPage(t, 2), shared.Hash)
	saveContent(t, saver, 1, one)
	saveContent(t, saver, 2, two)
	if uses, err := mediaUses(context.Background(), s.db); err != nil || uses[shared.Hash] != 3 {
		t.Fatalf("uses %v (%v), want 3", uses, err)
	}

	saveContent(t, saver, 1, withoutFigures(t, one, 1))
	sweepLater(t, s)
	assertKept(t, s, shared, true)
	saveContent(t, saver, 1, withoutFigures(t, one, 2))
	sweepLater(t, s)
	assertKept(t, s, shared, true)
	saveContent(t, saver, 2, withoutFigures(t, two, 1))
	sweepLater(t, s)
	assertKept(t, s, shared, false)
}

// A figure deleted and its deletion undone in one session finds its
// picture: nothing goes while the notebook is open, and the next launch
// counts the figure back.
func TestUndoingAFiguresDeletionKeepsItsPicture(t *testing.T) {
	root := t.TempDir()
	s := openStore(t, root)
	m, err := s.AddMedia(picture(t, 2, 2, 2), "")
	if err != nil {
		t.Fatal(err)
	}
	saver := s.Autosave()
	shown := withFigures(t, emptyPage(t, 1), m.Hash)
	saveContent(t, saver, 1, shown)
	saveContent(t, saver, 1, withoutFigures(t, shown, 1))
	assertKept(t, s, m, true)
	saveContent(t, saver, 1, shown)
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s = openStore(t, root)
	defer s.Close()
	sweepLater(t, s)
	assertKept(t, s, m, true)
	back, err := s.LoadPage(1, profile.Schema(), doc.NewInterner())
	if err != nil || Summarize(back.Root()).Picture != m.Hash {
		t.Fatalf("the page came back without its figure (%v)", err)
	}
}

// A sheet in the trash keeps its pictures; one deleted for good lets them
// go, once no backup shows them and the grace has passed.
func TestDeletingASheetLetsItsPicturesGo(t *testing.T) {
	root := t.TempDir()
	s := openStore(t, root)
	m, err := s.AddMedia(picture(t, 2, 2, 3), "")
	if err != nil {
		t.Fatal(err)
	}
	saver := s.Autosave()
	saveContent(t, saver, 1, withFigures(t, emptyPage(t, 1), m.Hash))
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	s.Close()

	// The closing backup shows the sheet with its figure; dated an hour
	// back, so the next backup, in the same second, is not written over it.
	backups, err := s.backups()
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups %v (%v)", backups, err)
	}
	earlier := filepath.Join(root, backupDir, backupPrefix+backups[0].taken.Add(-time.Hour).Format(backupLayout)+backupSuffix)
	if err := os.Rename(backups[0].path, earlier); err != nil {
		t.Fatal(err)
	}
	s = openStore(t, root)
	saver = s.Autosave()
	saver.Delete(1)
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	later := time.Now().Add(mediaGrace + time.Hour)
	if err := s.sweepMedia(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	assertKept(t, s, m, true)
	if err := s.sweepMedia(ctx, later); err != nil {
		t.Fatal(err)
	}
	assertKept(t, s, m, true)
	sweepLater(t, s)
	assertKept(t, s, m, false)
	s.Close()
}

// A page the sweep cannot read may show any picture, so none goes.
func TestAnUnreadablePageKeepsEveryPicture(t *testing.T) {
	s := openStore(t, t.TempDir())
	defer s.Close()
	m, err := s.AddMedia(picture(t, 2, 2, 4), "")
	if err != nil {
		t.Fatal(err)
	}
	saver := s.Autosave()
	saveContent(t, saver, 1, emptyPage(t, 1))
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE pages SET format = 99`); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(s.root, backupDir)); err != nil {
		t.Fatal(err)
	}
	if err := s.sweepMedia(context.Background(), time.Now().Add(mediaGrace+time.Hour)); !errors.Is(err, errUnreadable) {
		t.Fatalf("swept past an unreadable page: %v", err)
	}
	assertKept(t, s, m, true)
}
