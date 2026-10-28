package window

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/workspace"
)

func TestPageActionsRenameAndCloseTabsByID(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.RenamePage("0")
	if app.PageTitle == nil || !app.PageTitle.Open || app.PageTitle.Value != workspace.DefaultTitle {
		t.Fatalf("page title state = %#v", app.PageTitle)
	}
	app.PageTitle.Confirm("0", "  Chapter one  ")
	page, ok := app.Workspace.Page(0)
	if !ok || page.Title != "Chapter one" {
		t.Fatalf("renamed page = %#v, %v", page, ok)
	}

	second, err := app.Workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	app.Shell.ClosePageID("0")
	if slices.Contains(app.Workspace.Tabs(), 0) {
		t.Fatalf("closed tab remains: %#v", app.Workspace.Tabs())
	}
	if app.Shell.ActivePage != "1" {
		t.Fatalf("active page after closing inactive tab = %q", app.Shell.ActivePage)
	}

	app.Shell.ClosePageID("1")
	if _, active := app.Workspace.Active(); len(app.Workspace.Tabs()) != 0 || active {
		t.Fatalf("tabs after closing final page 1/%d = %#v", second, app.Workspace.Tabs())
	}
}
