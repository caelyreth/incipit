package window

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/app/library"
	"github.com/caelyreth/incipit/internal/app/storage"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/caelyreth/incipit/internal/ui/shell"
	"github.com/caelyreth/incipit/locales"
)

func TestEmptyPagePreservesMediaSheetAndDiscardsBlankParagraph(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())

	app.NewSheet(app.Shell.CurrentGroup(), shell.Last)
	blankSession := activeSession(t, app)
	if !app.emptyPage(blankSession.page) {
		t.Fatal("new untyped sheet reported non-empty")
	}
	if front, ok := app.Workspace.Front(); ok {
		app.closeColumn(columnName(front))
	}
	if _, exists := app.library.Sheet(library.SheetID(blankSession.key)); exists {
		t.Fatal("blank untyped sheet was not discarded on close")
	}

	app.NewSheet(app.Shell.CurrentGroup(), shell.Last)
	picSession := activeSession(t, app)
	el := app.pageElement()
	if el == nil {
		t.Fatal("pageElement nil")
	}
	if !el.Editor.Do(profile.PlaceFigures{
		Pictures: []profile.Picture{{Media: "media-hash-1", Width: 120, Height: 80}},
	}, time.Now()) {
		t.Fatal("failed to insert Figure block into fresh sheet")
	}
	if app.emptyPage(picSession.page) {
		t.Fatal("sheet holding only a Figure block was reported empty")
	}
	if front, ok := app.Workspace.Front(); ok {
		app.closeColumn(columnName(front))
	}
	if _, exists := app.library.Sheet(library.SheetID(picSession.key)); !exists {
		t.Fatal("figure-only sheet was unexpectedly removed on close")
	}
}

func TestCorruptSearchIndexRebuildsAndNotifies(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeIntoActivePage(t, app, "Resilient notebook notes")
	settleSearch(app)
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	indexDir := filepath.Join(root, indexFolder)
	entries, _ := os.ReadDir(indexDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			_ = os.WriteFile(filepath.Join(indexDir, entry.Name()), []byte("corrupt"), 0o644)
		}
	}
	_ = os.WriteFile(filepath.Join(indexDir, "store"), []byte("corrupt-index-bytes"), 0o644)

	reopened, lost := openChosen(root, t.TempDir())
	if lost != nil || reopened == nil {
		t.Fatalf("corrupt search index turned healthy notebook into lost=%+v", lost)
	}
	defer reopened.Close()
	if got := titles(reopened, "Resilient"); len(got) != 1 {
		t.Fatalf("after rebuild search found %v, want 1 match", got)
	}
	notices := reopened.Notices.take()
	var sawRebuilt bool
	for _, change := range notices {
		if change.notice.ID == "search.rebuilt" && change.notice.Title == locales.SearchIndexRebuilt() {
			sawRebuilt = true
		}
	}
	if !sawRebuilt {
		t.Fatalf("expected %q notice after search index rebuild, got %+v", locales.SearchIndexRebuilt(), notices)
	}
}

func TestBindNotebookSettingsPopulatesRootAndBackups(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeIntoActivePage(t, app, "Important sheet")
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	// Copy the verified closed database into Backups/ and reopen.
	dbBytes, err := os.ReadFile(filepath.Join(root, "notebook.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Backups"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Backups", "notebook-20261126-120000.db"), dbBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	reopened := openNotebookApp(t, root)
	reopened.bindNotebookSettings()
	if reopened.Settings.NotebookRoot != root {
		t.Fatalf("NotebookRoot = %q, want %q", reopened.Settings.NotebookRoot, root)
	}
	reopened.backupsWg.Wait()
	reopened.syncBackupsToSettings()
	backups, err := storage.ListBackups(root)
	if err != nil {
		t.Fatal(err)
	}
	hasSeeded := false
	for _, b := range reopened.Settings.Backups {
		if filepath.Base(b.Path) == "notebook-20261126-120000.db" {
			hasSeeded = true
			break
		}
	}
	if len(backups) == 0 || len(reopened.Settings.Backups) < 1 || !hasSeeded {
		t.Fatalf("Settings.Backups = %+v, storage.ListBackups = %d", reopened.Settings.Backups, len(backups))
	}

	// Subsequent refreshSettingsData reuses cached backups without re-scanning.
	reopened.backupsMu.Lock()
	cached := reopened.backupsCached
	backupsRoot := reopened.backupsRoot
	reopened.backupsMu.Unlock()
	reopened.refreshSettingsData()
	if len(reopened.Settings.Backups) != len(cached) || backupsRoot != root {
		t.Fatalf("cached backups lost across refreshSettingsData: %+v", reopened.Settings.Backups)
	}
}

func TestPerformBackupRestoreFlushesBeforeClosingAndAbortsOnFlushError(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeIntoActivePage(t, app, "Before backup")
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	dbBytes, err := os.ReadFile(filepath.Join(root, "notebook.db"))
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(root, "Backups", "notebook-20261126-120001.db")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, dbBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate FlushPending failure by closing underlying store early.
	reopened := openNotebookApp(t, root)
	el := reopened.pageElement()
	if el == nil || !el.Editor.Do(vocab.Type{Text: "Unsaved edit"}, time.Now()) {
		t.Fatal("failed typing unsaved edit")
	}
	_ = reopened.notebook.store.Close()
	err = reopened.performBackupRestore(root, backupPath)
	if err == nil {
		t.Fatal("expected error when FlushPending fails")
	}
	if reopened.closing.Load() || reopened.notebook.closed {
		t.Fatalf("expected notebook to remain open and uncommitted when FlushPending fails (closing=%v closed=%v)", reopened.closing.Load(), reopened.notebook.closed)
	}
	_ = reopened.Close()

	// Clean restore flushes pending edits, closes, and replaces database.
	clean := openNotebookApp(t, root)
	if err := clean.performBackupRestore(root, backupPath); err != nil {
		t.Fatalf("clean performBackupRestore failed: %v", err)
	}
	if !clean.closing.Load() || !clean.quitConfirmed || !clean.notebook.closed {
		t.Fatalf("expected notebook closed after clean restore: closing=%v quit=%v closed=%v", clean.closing.Load(), clean.quitConfirmed, clean.notebook.closed)
	}
}

func TestFlushForQuitBlocksActiveExport(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	app.exporting = &exportJob{writing: true}
	if app.flushForQuit() {
		t.Fatal("expected flushForQuit to block while export is actively writing")
	}
	app.exporting.writing = false
	if !app.flushForQuit() {
		t.Fatal("expected flushForQuit to succeed once export finishes writing")
	}
}
