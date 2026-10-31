package window

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
)

// A page is named after its first heading, whatever its level, and its
// tab follows as the heading is written.
func TestPagesAreNamedAfterTheirFirstHeading(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := app.Workspace.Page(0); page.Title != workspace.DefaultTitle {
		t.Fatalf("headless page = %q", page.Title)
	}
	session := app.Session.Document().Session()
	for _, r := range "prose\n### Chapter one" {
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
	app.refreshShellReadouts()
	if page, _ := app.Workspace.Page(0); page.Title != "Chapter one" {
		t.Fatalf("headed page = %q", page.Title)
	}
	if app.Shell.Tabs[0].Title != "Chapter one" {
		t.Fatalf("tab title = %q", app.Shell.Tabs[0].Title)
	}
}

func TestPageActionsCloseTabsByID(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
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
