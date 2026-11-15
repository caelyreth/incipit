package window

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/native/shell"
)

func openNotebookApp(t *testing.T, root string) *AppState {
	t.Helper()
	store, err := storage.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	app, err := OpenAppState(store)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Close() })
	return app
}

// settleSearch hands the index what changed and waits until it took it, as
// frames and the index's goroutine do in time.
func settleSearch(app *AppState) {
	app.reindex()
	app.finder.indexer.Flush()
}

// activeSession is the session of the page in front.
func activeSession(t *testing.T, app *AppState) *sheetSession {
	t.Helper()
	id, ok := app.Workspace.Active()
	if !ok {
		t.Fatal("no page in front")
	}
	return app.sessions[id]
}

// nameActivePage names the page in front, as its first heading will.
func nameActivePage(t *testing.T, app *AppState, title string) {
	t.Helper()
	session := activeSession(t, app)
	session.summary.Title, session.saved = title, false
	app.refreshShell()
}

// The pages, their titles and the tabs they stand in are there again
// after a quit.
func TestTheNotebookReopensAsItWasLeft(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	nameActivePage(t, app, "Chapter one")
	app.save()
	app.Shell.NewPage()
	nameActivePage(t, app, "second")
	app.Workspace.SetActiveScroll(workspace.ScrollPosition{Y: 36})
	app.save()
	app.Shell.NewPage()
	app.save()
	app.Shell.ClosePageID("2")
	app.Shell.SelectPage("0")
	app.save()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	pages := app.Workspace.Pages()
	if len(pages) != 3 || pages[0].Title != "Chapter one" || pages[1].Title != "second" {
		t.Fatalf("pages %+v", pages)
	}
	if tabs := app.Workspace.Opened(); !slices.Equal(tabs, []workspace.PageID{0, 1}) {
		t.Fatalf("tabs %v", tabs)
	}
	if active, _ := app.Workspace.Active(); active != 0 {
		t.Fatalf("active %d", active)
	}
	if app.Shell.Open[0].Title != "Chapter one" {
		t.Fatalf("open sheet title %q", app.Shell.Open[0].Title)
	}
	app.Shell.SelectPage("1")
	if scroll, _ := app.Workspace.ActiveScroll(); scroll.Y != 36 {
		t.Fatalf("second page scroll %v", scroll)
	}
	// A new page takes a key no stored page has.
	app.Shell.NewPage()
	keys := map[storage.PageKey]bool{}
	for _, session := range app.sessions {
		if keys[session.key] {
			t.Fatalf("key %d given twice", session.key)
		}
		keys[session.key] = true
	}
}

func TestANewNotebookOpensOnOneSavedPage(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	if _, ok := app.Workspace.Active(); len(app.Workspace.Pages()) != 1 || !ok {
		t.Fatal("a new notebook has no page in front")
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	app = openNotebookApp(t, root)
	defer app.Close()
	if len(app.Workspace.Pages()) != 1 {
		t.Fatalf("%d pages after reopening an empty notebook", len(app.Workspace.Pages()))
	}
}

// Groups, the sheets filed in them and the trash come back with the
// notebook; deleting for good takes a sheet off the disk.
func TestTheLibraryIsKeptAndItsTrashEmptied(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	novel := app.NewGroup("", shell.Last)
	app.RenameGroup(novel, "Novel")
	app.NewSheet(novel, shell.Last)
	nameActivePage(t, app, "Chapter one")
	chapter := app.Shell.ActivePage
	app.DuplicateSheet(chapter)
	if sheets := app.Sheets(shell.Place{Kind: shell.PlaceGroup, Group: novel}); len(sheets) != 2 ||
		sheets[0].Title != "Chapter one" || sheets[1].Title != "Chapter one" {
		t.Fatalf("novel holds %+v", sheets)
	}
	app.TrashSheet(chapter)
	if slices.ContainsFunc(app.Shell.Open, func(page shell.Page) bool { return page.ID == chapter }) {
		t.Fatal("a trashed sheet stays open")
	}
	inbox := app.Sheets(shell.Place{Kind: shell.PlaceGroup, Group: "1"})
	app.TrashSheet(inbox[0].ID)
	app.EmptyTrash()
	if trash := app.Sheets(shell.Place{Kind: shell.PlaceTrash}); len(trash) != 0 {
		t.Fatalf("trash after emptying %+v", trash)
	}
	app.save()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	groups := app.Groups("")
	if len(groups) != 2 || groups[1].Name != "Novel" {
		t.Fatalf("groups %+v", groups)
	}
	sheets := app.Sheets(shell.Place{Kind: shell.PlaceGroup, Group: groups[1].ID})
	if len(sheets) != 1 || sheets[0].Title != "Chapter one" {
		t.Fatalf("novel after reopening %+v", sheets)
	}
	if len(app.Workspace.Pages()) != 1 {
		t.Fatalf("%d pages on disk after emptying the trash", len(app.Workspace.Pages()))
	}
}

func TestChaptersAreKeptAndFileTheSheetsBelowThem(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	novel := app.NewGroup("", shell.Last)
	app.RenameGroup(novel, "Novel")
	app.NewSheet(novel, shell.Last)
	first := app.Shell.ActivePage
	app.NewSheet(novel, shell.Last)
	second := app.Shell.ActivePage
	chapter := app.NewChapter(novel, 1)
	app.RenameChapter(chapter, "Act One")

	entries := app.Entries(novel)
	if len(entries) != 3 || entries[0].ID != first ||
		entries[1].Kind != shell.EntryChapter || entries[1].ID != chapter ||
		entries[2].ID != second {
		t.Fatalf("novel entries %+v", entries)
	}
	session, _ := app.sessionOf(second)
	if got, ok := app.library.SheetChapter(library.SheetID(session.key)); !ok ||
		chapterName(got) != chapter {
		t.Fatalf("second sheet chapter %q %v", chapterName(got), ok)
	}
	app.MoveEntry(first, novel, 2)
	session, _ = app.sessionOf(first)
	if got, ok := app.library.SheetChapter(library.SheetID(session.key)); !ok ||
		chapterName(got) != chapter {
		t.Fatalf("moved sheet chapter %q %v", chapterName(got), ok)
	}
	app.save()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	entries = app.Entries(novel)
	if len(entries) != 3 || entries[0].Title != "Act One" || entries[0].ID != chapter ||
		entries[2].ID != first {
		t.Fatalf("chapters after reopening %+v", entries)
	}
}

func TestUnchangedFramesDoNotEnqueueAnotherLayout(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	defer app.Close()
	app.Workspace.SetActiveScroll(workspace.ScrollPosition{Y: 12})
	app.saveLayout()
	saved := app.notebook.layout
	if len(saved.Scroll) != 1 {
		t.Fatalf("saved layout %+v", saved)
	}
	for range 5 {
		app.saveLayout()
	}
	if !app.notebook.layout.Equal(saved) {
		t.Fatal("an unchanged frame changed the saved layout")
	}
}
