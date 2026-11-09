package window

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/app/workspace"
	"github.com/egoist/mygo/ui"
)

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

// The page shows its title over an empty page; with no page open the
// shell says how to open one.
func TestPageViewShowsTheOpenPagesTitle(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	texts := tester.Texts()
	page := slices.Index(texts, "Page")
	if page < 0 || page+1 >= len(texts) || texts[page+1] != workspace.DefaultTitle {
		t.Fatalf("no page heading in %#v", texts)
	}
	app.Shell.ClosePage()
	tester.Frame()
	if !tester.HasText("Open a sheet from the library, or press ⌘N for a new one") {
		t.Fatalf("page-less texts = %#v", tester.Texts())
	}
}
