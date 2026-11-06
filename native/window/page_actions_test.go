package window

import (
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/native/documentview"
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
			if _, err := session.Do(editor.Break, time.Now()); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if _, err := session.Do(editor.Insert(string(r), kernel.EventOriginTyping), time.Now()); err != nil {
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

func TestMountedPageChangeDetachesInputSynchronously(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	first := app.Session
	app.Bridge = documentview.NewBridge(first.Document(), nil, nil)
	app.Element.Input = app.Bridge
	second, err := app.Workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	if app.Session == first || app.Bridge != nil || app.Element.Input != nil {
		t.Fatal("selecting a new page retained the old page's input")
	}
	bridge := documentview.NewBridge(app.Session.Document(), nil, nil)
	app.Bridge, app.Element.Input = bridge, bridge
	if err := app.Workspace.Close(0); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	if app.Bridge != bridge || app.Element.Input != bridge {
		t.Fatal("closing an inactive page detached the current input")
	}
	if err := app.Workspace.Close(second); err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	if app.Session != nil || app.Bridge != nil || app.Element.Input != nil {
		t.Fatal("closing the last page retained a mounted input")
	}
}
