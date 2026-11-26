package window

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/library"
	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/ui/shell"
	"github.com/caelyreth/neverland/locales"
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
	backups, err := storage.ListBackups(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 || len(reopened.Settings.Backups) != len(backups) {
		t.Fatalf("Settings.Backups = %+v, storage.ListBackups = %d", reopened.Settings.Backups, len(backups))
	}
}
