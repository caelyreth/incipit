package window

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
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
	return app
}

func typeText(t *testing.T, app *AppState, text string) {
	t.Helper()
	session := app.Session.Document().Session()
	for _, r := range text {
		if r == '\n' {
			if _, err := session.ExecuteText(editor.TextNewline); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if _, err := session.ReplaceSelection(string(r), kernel.EventOriginTyping); err != nil {
			t.Fatal(err)
		}
	}
	app.Session.Document().Refresh()
}

// What is written, the pages it is written in and the tabs they stand in
// are there again after a quit.
func TestTheNotebookReopensAsItWasLeft(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeText(t, app, "# Chapter one\nprose")
	app.save()
	app.Shell.NewPage()
	typeText(t, app, "second")
	app.save()
	app.Shell.NewPage()
	app.save()
	app.Shell.ClosePageID("2")
	app.Shell.SelectPage("0")
	app.save()
	text := app.Session.Document().Text()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	pages := app.Workspace.Pages()
	if len(pages) != 3 || pages[0].Title != "Chapter one" {
		t.Fatalf("pages %+v", pages)
	}
	if tabs := app.Workspace.Tabs(); !slices.Equal(tabs, []workspace.PageID{0, 1}) {
		t.Fatalf("tabs %v", tabs)
	}
	if active, _ := app.Workspace.Active(); active != 0 {
		t.Fatalf("active %d", active)
	}
	if got := app.Session.Document().Text(); got != text {
		t.Fatalf("page text %q, want %q", got, text)
	}
	app.Shell.SelectPage("1")
	if got := app.Session.Document().Text(); got != "second" {
		t.Fatalf("second page text %q", got)
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
	if len(app.Workspace.Pages()) != 1 || app.Session == nil {
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

// Each page's caret, or the text selected on it, is where it was left.
func TestSelectionsAreWhereTheyWereLeft(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeText(t, app, "# Title\nfirst line\nsecond line")
	if err := app.Session.Document().SelectProjected(10, 14); err != nil {
		t.Fatal(err)
	}
	first := app.Session.Document().Selection()
	app.save()
	app.Shell.NewPage()
	typeText(t, app, "other page")
	if err := app.Session.Document().SelectProjected(3, 3); err != nil {
		t.Fatal(err)
	}
	second := app.Session.Document().Selection()
	app.save()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	if got := app.Session.Document().Selection(); got != second {
		t.Fatalf("front page selection %+v, want %+v", got, second)
	}
	app.Shell.SelectPage("0")
	if got := app.Session.Document().Selection(); got != first {
		t.Fatalf("first page selection %+v, want %+v", got, first)
	}
	if err := app.Session.Document().LastError(); err != nil {
		t.Fatal(err)
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
	typeText(t, app, "# Chapter one")
	chapter := app.Shell.ActivePage
	app.DuplicateSheet(chapter)
	if sheets := app.Sheets(shell.Place{Kind: shell.PlaceGroup, Group: novel}); len(sheets) != 2 ||
		sheets[0].Title != "Chapter one" || sheets[1].Title != "Chapter one" {
		t.Fatalf("novel holds %+v", sheets)
	}
	app.TrashSheet(chapter)
	if slices.ContainsFunc(app.Shell.Tabs, func(tab shell.Page) bool { return tab.ID == chapter }) {
		t.Fatal("a trashed sheet keeps its tab")
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
