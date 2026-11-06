package window

import (
	"testing"

	"github.com/caelyreth/neverland/engine/surface"

	"github.com/egoist/mygo/ui"
)

func TestCheckboxClickIsSavedAndReopened(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "- [ task")
	frames(tester, 3)
	if err := tester.Click("task"); err != nil {
		t.Fatal(err)
	}
	frames(tester, 3)
	if got := app.Session.Document().Text(); got != "- [x] task" {
		t.Fatalf("checked page = %q", got)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	defer app.Close()
	flow, err := app.Session.Document().Session().SurfaceFlow()
	if err != nil || len(surface.ObjectsOf[surface.Checkbox](flow)) != 1 || !surface.ObjectsOf[surface.Checkbox](flow)[0].Checked {
		t.Fatalf("reopened checkbox = %v, %v", flow, err)
	}
	if got := app.Session.Document().Text(); got != "- [x] task" {
		t.Fatalf("reopened page = %q", got)
	}
}
