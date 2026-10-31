package window

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
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
