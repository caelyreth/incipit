package storage

import (
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/library"
)

func openStore(t *testing.T, root string) *Store {
	t.Helper()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestANotebookReopensAsItWasLeft(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave()
	saver.Page(1, "Title")
	saver.Page(2, "")
	saver.Layout(Layout{
		Tabs: []PageKey{2, 1}, Active: 1, HasActive: true,
		Scroll: map[PageKey]float32{1: 120},
	})
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store = openStore(t, root)
	defer store.Close()
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(notebook.Pages) != 2 || notebook.Pages[0].Key != 1 || notebook.Pages[1].Key != 2 || notebook.NextKey != 3 {
		t.Fatalf("pages %+v next %d", notebook.Pages, notebook.NextKey)
	}
	if notebook.Pages[0].Title != "Title" || notebook.Pages[1].Title != "" {
		t.Fatalf("titles %+v", notebook.Pages)
	}
	if notebook.Pages[0].Created.IsZero() || notebook.Pages[0].Updated.Before(notebook.Pages[0].Created) {
		t.Fatalf("times %+v", notebook.Pages[0])
	}
	layout := notebook.Layout
	if !notebook.HasLayout || len(layout.Tabs) != 2 || layout.Tabs[0] != 2 || layout.Active != 1 ||
		!layout.HasActive || layout.Scroll[1] != 120 {
		t.Fatalf("layout %+v", layout)
	}
}

// A later write of a page keeps when it was made and moves when it changed.
func TestARewrittenPageKeepsItsCreation(t *testing.T) {
	store := openStore(t, t.TempDir())
	defer store.Close()
	saver := store.Autosave()
	defer saver.Close()
	saver.Page(1, "First")
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	saver.Page(1, "Second")
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	first, second := before.Pages[0], after.Pages[0]
	if second.Title != "Second" || !second.Created.Equal(first.Created) || !second.Updated.After(first.Updated) {
		t.Fatalf("before %+v after %+v", first, second)
	}
}

func TestOneProcessHoldsTheNotebook(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	if _, err := Open(root); !errors.Is(err, ErrLocked) {
		t.Fatalf("second open: %v", err)
	}
	store.Close()
	openStore(t, root).Close()
}

// TestMain lets a test run the notebook in a process of its own, which it
// can then kill.
func TestMain(m *testing.M) {
	if root := os.Getenv("NEVERLAND_CRASH_ROOT"); root != "" {
		crashWhileWriting(root)
		return
	}
	os.Exit(m.Run())
}

// crashWhileWriting saves a page and dies without closing anything.
func crashWhileWriting(root string) {
	store, err := Open(root)
	if err != nil {
		os.Exit(2)
	}
	saver := store.Autosave()
	saver.Page(7, "written before the crash")
	if err := saver.Flush(); err != nil {
		os.Exit(3)
	}
	os.Exit(1)
}

func TestACrashKeepsWhatWasWritten(t *testing.T) {
	root := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = append(os.Environ(), "NEVERLAND_CRASH_ROOT="+root)
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("crashing process: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, databaseName+"-wal")); err != nil {
		t.Fatalf("the crash left no log to replay: %v", err)
	}
	store := openStore(t, root)
	defer store.Close()
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(notebook.Pages) != 1 || notebook.Pages[0].Key != 7 ||
		notebook.Pages[0].Title != "written before the crash" {
		t.Fatalf("after the crash: %+v", notebook.Pages)
	}
}

func TestADamagedNotebookIsReplacedByItsBackup(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave()
	saver.Page(1, "kept in the backup")
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	store.Close()
	backups, err := (&Store{root: root}).backups()
	if err != nil || len(backups) == 0 {
		t.Fatalf("no backup on close: %v", err)
	}

	garbage := make([]byte, 8192)
	for index := range garbage {
		garbage[index] = byte(index * 31)
	}
	if err := os.WriteFile(filepath.Join(root, databaseName), garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	store = openStore(t, root)
	defer store.Close()
	if !store.Recovery.Repaired() || store.Recovery.Restored == "" {
		t.Fatalf("recovery %+v", store.Recovery)
	}
	if moved, err := os.ReadFile(filepath.Join(store.Recovery.Damaged, databaseName)); err != nil || len(moved) != len(garbage) {
		t.Fatalf("the damaged notebook was not kept: %v", err)
	}
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(notebook.Pages) != 1 || notebook.Pages[0].Title != "kept in the backup" {
		t.Fatalf("restored %+v", notebook.Pages)
	}
}

func TestAnotherApplicationsDatabaseIsLeftAlone(t *testing.T) {
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE theirs (x)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(root); !errors.Is(err, ErrForeign) {
		t.Fatalf("open: %v", err)
	}
}

func TestBackupsKeepTheRecentThenOneADay(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, backupDir)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 30, 12, 0, 0, 0, time.Local)
	// Four a day for sixty days.
	for index := range 240 {
		taken := start.Add(-time.Duration(index) * 6 * time.Hour)
		name := filepath.Join(folder, backupPrefix+taken.Format(backupLayout)+backupSuffix)
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store := &Store{root: root}
	if err := store.prune(); err != nil {
		t.Fatal(err)
	}
	backups, _ := store.backups()
	if len(backups) != recentBackups+dailyBackups-4 {
		// The twelve recent ones span four days, which count among the
		// thirty.
		t.Fatalf("%d backups kept", len(backups))
	}
	if !backups[0].taken.Equal(start) {
		t.Fatalf("newest kept is %v", backups[0].taken)
	}
}

func TestChangesAreWrittenWithoutBeingAsked(t *testing.T) {
	store := openStore(t, t.TempDir())
	defer store.Close()
	saver := store.Autosave()
	defer saver.Close()
	deadline := time.Now().Add(longestDelay + time.Second)
	// Typing on at a steady pace still writes within the longest delay.
	for time.Now().Before(deadline) {
		saver.Page(1, "typed")
		var rows int
		if err := store.db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows == 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("nothing was written while changes kept arriving")
}

func TestTheNotebookFolderIsMarkedLocal(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS syncs Documents implicitly")
	}
	root := t.TempDir()
	openStore(t, root).Close()
	if _, err := os.Stat(filepath.Join(root, ".nosync")); err != nil {
		t.Fatalf("no .nosync marker: %v", err)
	}
	if output, err := exec.Command("xattr", "-p", "com.apple.fileprovider.ignore#P", root).CombinedOutput(); err != nil {
		t.Fatalf("the folder is not ignored by the File Provider: %v %s", err, output)
	}
}

func TestTheLibraryAndDeletionsAreKept(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	shelf := library.New()
	novel, _ := shelf.AddGroup(0, "Novel")
	if err := shelf.StyleGroup(novel, "feather", 3); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	if err := shelf.AddSheet(1, novel, at); err != nil {
		t.Fatal(err)
	}
	chapter, err := shelf.AddChapter(novel, "Part One")
	if err != nil {
		t.Fatal(err)
	}
	if err := shelf.AddSheet(2, library.Inbox, at); err != nil {
		t.Fatal(err)
	}
	if err := shelf.TrashSheet(2, at); err != nil {
		t.Fatal(err)
	}
	saver := store.Autosave()
	saver.Page(1, "one")
	saver.Page(2, "two")
	saver.Page(3, "three")
	saver.Library(shelf.Records())
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	saver.Delete(3)
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	store.Close()

	store = openStore(t, root)
	defer store.Close()
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(notebook.Pages) != 2 {
		t.Fatalf("%d pages after deleting one", len(notebook.Pages))
	}
	restored := library.Restore(notebook.Library)
	if group, _ := restored.Group(novel); group.Icon != "feather" || group.Color != 3 {
		t.Fatalf("novel's look %+v", group)
	}
	if sheets := restored.Sheets(novel); len(sheets) != 1 || sheets[0].ID != 1 {
		t.Fatalf("novel holds %+v", sheets)
	}
	if got, _ := restored.Chapter(chapter); got.Name != "Part One" {
		t.Fatalf("chapter %+v", got)
	}
	entries := restored.Entries(novel)
	if len(entries) != 2 || !entries[1].IsChapter() || entries[1].Chapter != chapter {
		t.Fatalf("novel entries %+v", entries)
	}
	if !restored.SheetInTrash(2) {
		t.Fatal("the trash was lost")
	}
	if group, _ := restored.Group(novel); group.Name != "Novel" {
		t.Fatalf("group %+v", group)
	}
}

// Whoever watches the saver is told after each write, to read how it went.
func TestTheWatcherIsToldAfterEachWrite(t *testing.T) {
	store := openStore(t, t.TempDir())
	defer store.Close()
	saver := store.Autosave()
	defer saver.Close()
	told := make(chan error, 4)
	saver.Watch(func() { told <- saver.Err() })
	saver.Page(1, "typed")
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-told:
		if err != nil {
			t.Fatalf("told of %v after a write that went well", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the watcher was not told of the write")
	}
}
