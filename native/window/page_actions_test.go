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
	if slices.Contains(app.Workspace.Opened(), 0) {
		t.Fatalf("closed page remains: %#v", app.Workspace.Opened())
	}
	if app.Shell.ActivePage != "1" {
		t.Fatalf("active page after closing an inactive page = %q", app.Shell.ActivePage)
	}

	app.Shell.ClosePageID("1")
	if _, active := app.Workspace.Active(); len(app.Workspace.Opened()) != 0 || active {
		t.Fatalf("open pages after closing final page 1/%d = %#v", second, app.Workspace.Opened())
	}
}

// A page is titled by its first heading, in its row, as it is written;
// with no page open the shell says how to open one.
func TestAPageIsTitledByItsFirstHeading(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	if !tester.HasText(workspace.DefaultTitle) {
		t.Fatalf("no untitled sheet in %#v", tester.Texts())
	}
	for _, r := range "# Morning notes" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Frame()
	if !tester.HasText("Morning notes") {
		t.Fatalf("the sheet is not titled by the heading: %#v", tester.Texts())
	}
	app.Shell.ClosePage()
	tester.Frame()
	if !tester.HasText("Open a sheet from the library, or press ⌘N for a new one") {
		t.Fatalf("page-less texts = %#v", tester.Texts())
	}
}
