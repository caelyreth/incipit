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
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
)

func openStore(t *testing.T, root string) *Store {
	t.Helper()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func page(t *testing.T, text string) *model.Document {
	t.Helper()
	document, _, err := editor.NewBlockDocument(text)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// edit changes a document as an editor transaction would.
func edit(t *testing.T, document *model.Document, operation func(*model.DocumentDraft) error) {
	t.Helper()
	if _, err := document.Transact(operation); err != nil {
		t.Fatal(err)
	}
}

func block(document *model.Document, index int) model.NodeID {
	snapshot := document.Snapshot()
	root, _ := snapshot.Node(snapshot.Root())
	return root.Children()[index]
}

func sameDocument(t *testing.T, want, got model.DocumentSnapshot) {
	t.Helper()
	if want.Root() != got.Root() || want.NextID() != got.NextID() {
		t.Fatalf("root %v next %d, want root %v next %d", got.Root(), got.NextID(), want.Root(), want.NextID())
	}
	wantNodes, gotNodes := want.Nodes(), got.Nodes()
	if len(wantNodes) != len(gotNodes) {
		t.Fatalf("%d nodes, want %d", len(gotNodes), len(wantNodes))
	}
	for index, node := range wantNodes {
		a, b := node.Record(), gotNodes[index].Record()
		if a.ID != b.ID || a.Kind != b.Kind || a.Parent != b.Parent || a.HasParent != b.HasParent ||
			a.NextMarkID != b.NextMarkID || len(a.Children) != len(b.Children) ||
			len(a.Attributes) != len(b.Attributes) || len(a.Marks) != len(b.Marks) ||
			(a.Text == nil) != (b.Text == nil) || a.Text != nil && *a.Text != *b.Text {
			t.Fatalf("node %v: %+v, want %+v", a.ID, b, a)
		}
		for key, value := range a.Attributes {
			if b.Attributes[key] != value {
				t.Fatalf("node %v attribute %s: %v, want %v", a.ID, key, b.Attributes[key], value)
			}
		}
		for index := range a.Marks {
			if a.Marks[index] != b.Marks[index] {
				t.Fatalf("node %v mark %d: %+v, want %+v", a.ID, index, b.Marks[index], a.Marks[index])
			}
		}
		for index := range a.Children {
			if a.Children[index] != b.Children[index] {
				t.Fatalf("node %v children %v, want %v", a.ID, b.Children, a.Children)
			}
		}
	}
}

func TestANotebookReopensAsItWasLeft(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	first, second := page(t, "# Title\nhello world"), page(t, "")
	edit(t, first, func(draft *model.DocumentDraft) error {
		node := block(first, 1)
		if err := draft.SetAttribute(node, "kind", model.TextValue("quote")); err != nil {
			return err
		}
		if err := draft.SetAttribute(node, "depth", model.IntegerValue(2)); err != nil {
			return err
		}
		if err := draft.SetAttribute(node, "open", model.BoolValue(true)); err != nil {
			return err
		}
		_, err := draft.AddMark(node, model.NewUTF16Range(0, 5), "strong", 2)
		return err
	})
	saver := store.Autosave(nil)
	saver.Page(1, "Title", first.Snapshot())
	saver.Page(2, "", second.Snapshot())
	selection := Selection{Anchor: Position{Node: 3, Offset: 2, Affinity: 1}, Head: Position{Node: 3, Offset: 5}}
	saver.Layout(Layout{
		Tabs: []PageKey{2, 1}, Active: 1, HasActive: true,
		Scroll:     map[PageKey]float32{1: 120},
		Selections: map[PageKey]Selection{1: selection},
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
	sameDocument(t, first.Snapshot(), notebook.Pages[0].Document.Snapshot())
	sameDocument(t, second.Snapshot(), notebook.Pages[1].Document.Snapshot())
	layout := notebook.Layout
	if len(layout.Tabs) != 2 || layout.Tabs[0] != 2 || layout.Active != 1 || !layout.HasActive ||
		layout.Scroll[1] != 120 || layout.Selections[1] != selection {
		t.Fatalf("layout %+v", layout)
	}
}

func TestEditsAreWrittenAsTheyChangeTheTree(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	document := page(t, "one\ntwo\nthree")
	saver := store.Autosave(nil)
	saver.Page(1, "", document.Snapshot())
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	// Remove a block, change one, add one: each write is only the
	// difference from the last.
	edit(t, document, func(draft *model.DocumentDraft) error {
		if err := draft.RemoveNode(block(document, 1)); err != nil {
			return err
		}
		return draft.SetTextValue(block(document, 0), "first")
	})
	saver.Page(1, "", document.Snapshot())
	if err := saver.Flush(); err != nil {
		t.Fatal(err)
	}
	edit(t, document, func(draft *model.DocumentDraft) error {
		id, err := draft.CreateNode(draft.Root(), 0, editor.ParagraphNode)
		if err != nil {
			return err
		}
		return draft.SetTextValue(id, "zero")
	})
	saver.Page(1, "", document.Snapshot())
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
	sameDocument(t, document.Snapshot(), notebook.Pages[0].Document.Snapshot())
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
	document, _, _ := editor.NewBlockDocument("written before the crash")
	saver := store.Autosave(nil)
	saver.Page(7, "", document.Snapshot())
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
		plainText(notebook.Pages[0].Document.Snapshot()) != "written before the crash\n" {
		t.Fatalf("after the crash: %+v", notebook.Pages)
	}
}

func TestADamagedNotebookIsReplacedByItsBackup(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave(nil)
	saver.Page(1, "", page(t, "kept in the backup").Snapshot())
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
	if len(notebook.Pages) != 1 || plainText(notebook.Pages[0].Document.Snapshot()) != "kept in the backup\n" {
		t.Fatalf("restored %+v", notebook.Pages)
	}
}

func TestADamagedPageIsLeftAndReported(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave(nil)
	saver.Page(1, "", page(t, "sound").Snapshot())
	saver.Page(2, "", page(t, "broken").Snapshot())
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE nodes SET children = '[99]' WHERE page = 2 AND parent IS NULL`); err != nil {
		t.Fatal(err)
	}
	store.Close()

	store = openStore(t, root)
	defer store.Close()
	notebook, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(notebook.Pages) != 1 || notebook.Pages[0].Key != 1 ||
		len(notebook.Damaged) != 1 || notebook.Damaged[0].Key != 2 || notebook.NextKey != 3 {
		t.Fatalf("pages %+v damaged %+v next %d", notebook.Pages, notebook.Damaged, notebook.NextKey)
	}
	var rows int
	if err := store.db.QueryRow(`SELECT count(*) FROM nodes WHERE page = 2`).Scan(&rows); err != nil || rows == 0 {
		t.Fatalf("the damaged page's rows are gone: %v", err)
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
	saver := store.Autosave(nil)
	defer saver.Close()
	document := page(t, "typed")
	deadline := time.Now().Add(longestDelay + time.Second)
	// Typing on at a steady pace still writes within the longest delay.
	for time.Now().Before(deadline) {
		saver.Page(1, "", document.Snapshot())
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
	at := time.Now()
	if err := shelf.AddSheet(1, novel, at); err != nil {
		t.Fatal(err)
	}
	if err := shelf.AddSheet(2, library.Inbox, at); err != nil {
		t.Fatal(err)
	}
	if err := shelf.TrashSheet(2, at); err != nil {
		t.Fatal(err)
	}
	saver := store.Autosave(nil)
	saver.Page(1, "", page(t, "one").Snapshot())
	saver.Page(2, "", page(t, "two").Snapshot())
	saver.Page(3, "", page(t, "three").Snapshot())
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
	if sheets := restored.Sheets(novel); len(sheets) != 1 || sheets[0].ID != 1 {
		t.Fatalf("novel holds %+v", sheets)
	}
	if !restored.SheetInTrash(2) {
		t.Fatal("the trash was lost")
	}
	if group, _ := restored.Group(novel); group.Name != "Novel" {
		t.Fatalf("group %+v", group)
	}
}
